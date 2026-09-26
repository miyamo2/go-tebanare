package config

import (
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// supportedVersion is the only valid value of `version`.
const supportedVersion = 1

// topKeys lists the top-level keys of a config.
var topKeys = []string{"version", "files", "presets"}

// Error is an invalid configuration.
type Error struct {
	// Diagnostics holds every error found, sorted by position. Each has
	// severity error and code config-invalid or config-syntax.
	Diagnostics []result.Diagnostic
}

// Error returns the diagnostics formatted with Diagnostic.Format(""), one
// per line.
func (e *Error) Error() string {
	if len(e.Diagnostics) == 0 {
		return "invalid configuration"
	}
	lines := make([]string, len(e.Diagnostics))
	for i, d := range e.Diagnostics {
		lines[i] = d.Format("")
	}
	return strings.Join(lines, "\n")
}

// Compile validates the configuration in src and compiles it into a
// rule.Set. The set holds the rules of the presets in `presets` order.
//
// When src is invalid, Compile returns a nil set and an *Error. The
// returned diagnostics are warnings; Compile returns them whether src is
// valid or not.
//
// yaml.v3 reports YAML syntax errors by panicking and recovering inside
// its parser. In a build where recover does not work, such as TinyGo for
// wasm-unknown, a syntax error stops the program with a trap before
// Compile can return the config-syntax error.
func Compile(src []byte) (*rule.Set, []result.Diagnostic, error) {
	c := &compiler{}
	set := c.compile(src)
	sortByPosition(c.errs)
	sortByPosition(c.warns)
	if len(c.errs) > 0 {
		return nil, c.warns, &Error{Diagnostics: c.errs}
	}
	return set, c.warns, nil
}

func (c *compiler) compile(src []byte) *rule.Set {
	root, ok := c.parse(src)
	if !ok || !c.checkAliases(root) {
		return nil
	}
	es, ok := c.mapping(root, "", topKeys...)
	if !ok {
		return nil
	}
	c.version(es, root)
	set := &rule.Set{}
	if fe, ok := es.get("files"); ok {
		set.Include, set.Exclude = c.files(fe)
	}
	if pe, ok := es.get("presets"); ok {
		set.Rules = c.presets(pe)
	}
	return set
}

// version checks that `version` is set to the supported version.
func (c *compiler) version(es entries, root *yaml.Node) {
	e, ok := es.get("version")
	if !ok {
		c.errorf(root, "", "missing required key %q", "version")
		return
	}
	n := resolve(e.value)
	if n.Kind != yaml.ScalarNode || effectiveTag(n) != "!!int" {
		c.errorf(e.value, e.field, "expected an integer, found %s", describe(n))
		return
	}
	// yaml.v3 reads integers the same way: underscores removed, then
	// strconv with base prefixes.
	v, err := strconv.ParseInt(strings.ReplaceAll(n.Value, "_", ""), 0, 64)
	if err != nil || v != supportedVersion {
		c.errorf(e.value, e.field, "unsupported version %s (supported versions: %d)", n.Value, supportedVersion)
	}
}

// files decodes `files`. An empty or missing include list means
// rule.DefaultInclude.
func (c *compiler) files(e entry) (include, exclude []string) {
	es, ok := c.mapping(e.value, e.field, "include", "exclude")
	if !ok {
		return nil, nil
	}
	if ie, ok := es.get("include"); ok {
		include = c.globs(ie)
	}
	if xe, ok := es.get("exclude"); ok {
		exclude = c.globs(xe)
	}
	return include, exclude
}
