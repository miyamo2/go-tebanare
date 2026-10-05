package config

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// supportedVersion is the only valid value of `version`.
const supportedVersion = 1

// File is the content of a configuration file. Compile decodes it with
// yaml.v3, which rejects unknown keys. A missing key or a null value
// leaves the field nil.
type File struct {
	// Version is the version of the configuration format. It is required.
	Version *int64 `yaml:"version"`
	// Files selects the files to analyze.
	Files *Files `yaml:"files"`
	// Presets lists the presets to enable. Each item is a preset name, or
	// a mapping from the name to its settings; the items stay YAML nodes
	// so that the errors in them carry their positions.
	Presets []yaml.Node `yaml:"presets"`
}

// Files is the `files` section of a configuration.
type Files struct {
	// Include lists the files to analyze, as doublestar globs. An empty
	// list means rule.DefaultInclude.
	Include []string `yaml:"include"`
	// Exclude lists the files to skip even when Include matches them.
	Exclude []string `yaml:"exclude"`
}

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
// yaml.v3 reports YAML syntax errors, and some values it cannot decode,
// such as a scalar whose explicit tag does not fit its value, by
// panicking and recovering. In a build where recover does not work, such
// as TinyGo for wasm-unknown, such an error stops the program with a trap
// before Compile can return the config-syntax error.
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
	root, skip, ok := c.parse(src)
	if !ok || !c.checkAliases(root) {
		return nil
	}
	var f File
	if !c.decode(src, skip, &f) {
		return nil
	}
	c.version(f.Version, root)
	set := &rule.Set{}
	if f.Files != nil {
		set.Include = c.globs(f.Files.Include, "files.include")
		set.Exclude = c.globs(f.Files.Exclude, "files.exclude")
	}
	if f.Presets != nil {
		set.Rules = c.presets(f.Presets)
	}
	return set
}

// decode decodes the document of src that parse found, after skip empty
// documents, into f. It reports the errors of yaml.v3 and returns false
// after any.
func (c *compiler) decode(src []byte, skip int, f *File) bool {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	for range skip {
		var empty yaml.Node
		if err := dec.Decode(&empty); err != nil {
			// parse read these documents without an error.
			c.syntaxError(err)
			return false
		}
	}
	err := dec.Decode(f)
	if err == nil {
		return true
	}
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		// yaml.v3 panicked and recovered, as it does for a YAML error.
		c.syntaxError(err)
		return false
	}
	for _, msg := range te.Errors {
		c.typeError(msg)
	}
	return false
}

// typeError reports an error of the yaml.v3 decoder, such as "line 3:
// cannot unmarshal !!str `x` into int64".
func (c *compiler) typeError(msg string) {
	line := 0
	if rest, ok := strings.CutPrefix(msg, "line "); ok {
		if num, text, ok := strings.Cut(rest, ": "); ok {
			if n, err := strconv.Atoi(num); err == nil {
				line, msg = n, text
			}
		}
	}
	c.errorf(&yaml.Node{Line: line}, "", "%s", msg)
}

// version checks that `version` is set to the supported version.
func (c *compiler) version(v *int64, root *yaml.Node) {
	if v == nil {
		c.errorf(root, "", "missing required key %q", "version")
		return
	}
	if *v != supportedVersion {
		c.errorf(nil, "version", "unsupported version %d (supported versions: %d)", *v, supportedVersion)
	}
}

// globs checks a list of doublestar patterns and returns the valid ones.
func (c *compiler) globs(globs []string, f field) []string {
	if globs == nil {
		return nil
	}
	out := make([]string, 0, len(globs))
	for i, g := range globs {
		if !doublestar.ValidatePattern(g) {
			c.errorf(nil, f.index(i), "invalid glob %q", g)
			continue
		}
		out = append(out, g)
	}
	return out
}
