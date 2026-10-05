package config

import (
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/miyamo2/go-tebanare/internal/configschema"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

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
	c.root = root
	v, ok := c.value(root, nil)
	if !ok || !c.validate(v) {
		return nil
	}
	// The value passed the schema, so it has the shape of the generated
	// types.
	cfg, err := configschema.DecodeConfig(v)
	if err != nil {
		c.errorf(c.root, "", "%v", err)
		return nil
	}
	set := &rule.Set{}
	if cfg.Files != nil {
		set.Include = c.globs(cfg.Files.Include, "files", "include")
		set.Exclude = c.globs(cfg.Files.Exclude, "files", "exclude")
	}
	// The items of `presets` are read from v: a generated PresetElement
	// does not tell which preset a mapping with null settings names.
	items, _ := v.(map[string]any)["presets"].([]any)
	set.Rules = c.presets(items)
	return set
}

// globs checks a list of doublestar patterns, the value at path, and
// returns the valid ones. The schema cannot check the pattern syntax.
func (c *compiler) globs(globs []string, path ...string) []string {
	if globs == nil {
		return nil
	}
	out := make([]string, 0, len(globs))
	for i, g := range globs {
		if !doublestar.ValidatePattern(g) {
			at := append(slices.Clone(path), strconv.Itoa(i))
			c.errorf(c.nodeAt(at), c.fieldOf(at), "invalid glob %q", g)
			continue
		}
		out = append(out, g)
	}
	return out
}
