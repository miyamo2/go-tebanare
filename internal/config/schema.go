package config

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/miyamo2/go-tebanare/internal/configschema"
	"github.com/miyamo2/go-tebanare/internal/presets"
	"github.com/miyamo2/go-tebanare/schema"
)

var (
	compileSchemaOnce sync.Once
	compiledSchema    *jsonschema.Schema
	compileSchemaErr  error
)

// loadSchema compiles the embedded schema once.
func loadSchema() (*jsonschema.Schema, error) {
	compileSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.JSON))
		if err != nil {
			compileSchemaErr = err
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(schema.ID, doc); err != nil {
			compileSchemaErr = err
			return
		}
		compiledSchema, compileSchemaErr = c.Compile(schema.ID)
	})
	return compiledSchema, compileSchemaErr
}

// validate checks v, the value of the config, against the schema and
// reports each error at the YAML node of the value at fault. It returns
// false after any error.
func (c *compiler) validate(v any) bool {
	sch, err := loadSchema()
	if err != nil {
		c.errorf(nil, "", "the schema does not compile: %v", err)
		return false
	}
	err = sch.Validate(v)
	if err == nil {
		return true
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		c.errorf(nil, "", "%v", err)
		return false
	}
	c.schemaError(ve)
	return false
}

// schemaError reports the leaves of the error tree e. Under anyOf, it
// leaves out the alternatives that fail only because the value has
// another type, such as the preset name alternative for a mapping.
func (c *compiler) schemaError(e *jsonschema.ValidationError) {
	if _, ok := e.ErrorKind.(*kind.AnyOf); ok {
		var keep []*jsonschema.ValidationError
		var want []string
		for _, alt := range e.Causes {
			if t, ok := typeMismatch(alt, e.InstanceLocation); ok {
				want = append(want, t.Want...)
				continue
			}
			keep = append(keep, alt)
		}
		if len(keep) == 0 {
			c.report(e.InstanceLocation, &kind.Type{Want: want})
			return
		}
		for _, alt := range keep {
			c.schemaError(alt)
		}
		return
	}
	if len(e.Causes) > 0 {
		for _, cause := range e.Causes {
			c.schemaError(cause)
		}
		return
	}
	c.report(e.InstanceLocation, e.ErrorKind)
}

// typeMismatch reports whether alt, an alternative of anyOf at loc, fails
// only with a type error at loc.
func typeMismatch(alt *jsonschema.ValidationError, loc []string) (*kind.Type, bool) {
	for len(alt.Causes) == 1 {
		alt = alt.Causes[0]
	}
	t, ok := alt.ErrorKind.(*kind.Type)
	return t, ok && len(alt.Causes) == 0 && slices.Equal(alt.InstanceLocation, loc)
}

var printer = message.NewPrinter(language.English)

// report reports one schema error at the value at path.
func (c *compiler) report(path []string, k jsonschema.ErrorKind) {
	n := c.nodeAt(path)
	f := c.fieldOf(path)
	presetItem := len(path) == 2 && path[0] == "presets"
	inSettings := len(path) >= 3 && path[0] == "presets"
	if _, ok := presets.Lookup(presetName(c.nodeAt(path[:min(len(path), 2)]))); ok && inSettings {
		c.ruleID = presetName(c.nodeAt(path[:2]))
		defer func() { c.ruleID = "" }()
	}
	switch k := k.(type) {
	case *kind.Type:
		if len(path) == 3 && inSettings {
			c.errorf(n, f, "settings must be a mapping, found %s", describe(resolve(n)))
			return
		}
		if presetItem {
			c.errorf(n, f, "expected a preset name or a mapping from a preset name to its settings, found %s", describe(resolve(n)))
			return
		}
		want := typeWords(k.Want)
		if want == "a list" && !slices.Equal(path, []string{"presets"}) {
			// Every list in the config but `presets` is a list of strings.
			want = "a list of strings"
		}
		c.errorf(n, f, "expected %s, found %s", want, describe(resolve(n)))
	case *kind.Required:
		for _, name := range k.Missing {
			c.errorf(n, f, "missing required key %q", name)
		}
	case *kind.AdditionalProperties:
		for _, name := range k.Properties {
			key := keyNode(n, name)
			switch {
			case presetItem:
				// f already names the preset: the item has this one key.
				c.errorf(key, f, "unknown preset %q (available presets: %s)", name, strings.Join(presets.Names(), ", "))
			case len(path) == 3 && path[0] == "presets":
				var names []string
				if p, ok := presets.Lookup(path[2]); ok {
					for _, s := range presets.Settings(p) {
						names = append(names, s.Name)
					}
				}
				c.errorf(key, f.key(name), "unknown setting %q (available settings: %s)", name, strings.Join(names, ", "))
			default:
				c.errorf(key, f.key(name), "unknown key %q (allowed keys: %s)", name, strings.Join(allowedKeys(path), ", "))
			}
		}
	case *kind.MinProperties, *kind.MaxProperties:
		if presetItem {
			c.errorf(n, f, "expected a mapping with one key, the preset name, found %d keys", len(resolve(n).Content)/2)
			return
		}
		c.errorf(n, f, "%s", k.LocalizedString(printer))
	case *kind.Enum:
		var want []string
		for _, w := range k.Want {
			if w != nil {
				want = append(want, fmt.Sprint(w))
			}
		}
		if presetItem {
			c.errorf(n, f, "unknown preset %q (available presets: %s)", fmt.Sprint(k.Got), strings.Join(want, ", "))
			return
		}
		c.errorf(n, f, "unknown value %q (valid values: %s)", fmt.Sprint(k.Got), strings.Join(want, ", "))
	case *kind.Const:
		if slices.Equal(path, []string{"version"}) {
			c.errorf(n, f, "unsupported version %s (supported versions: %v)", resolve(n).Value, k.Want)
			return
		}
		c.errorf(n, f, "must be %v, found %s", k.Want, resolve(n).Value)
	case *kind.Minimum:
		c.errorf(n, f, "must be at least %s, found %s", k.Want.RatString(), k.Got.RatString())
	case *kind.Maximum:
		c.errorf(n, f, "must be at most %s, found %s", k.Want.RatString(), k.Got.RatString())
	case *kind.MinItems:
		c.errorf(n, f, "must have at least %d %s, found %d", k.Want, plural(k.Want, "item"), k.Got)
	case *kind.Pattern:
		c.errorf(n, f, "invalid value %q: it must match the pattern %s", k.Got, k.Want)
	default:
		c.errorf(n, f, "%s", k.LocalizedString(printer))
	}
}

// typeWords names JSON Schema types for messages, leaving out null, which
// every nullable value also accepts.
func typeWords(types []string) string {
	words := map[string]string{
		"object": "a mapping", "array": "a list", "string": "a string", "integer": "an integer",
		"number": "a number", "boolean": "a boolean",
	}
	var out []string
	for _, t := range types {
		if w, ok := words[t]; ok && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return "null"
	}
	return strings.Join(out, " or ")
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// allowedKeys returns the keys of the mapping at path, in schema order.
func allowedKeys(path []string) []string {
	keys, _ := configschema.Keys(path...)
	return keys
}

// nodeAt returns the node of the value at path, or the deepest node on the
// way.
func (c *compiler) nodeAt(path []string) *yaml.Node {
	n := c.root
	for _, seg := range path {
		next := child(n, seg)
		if next == nil {
			return n
		}
		n = next
	}
	return n
}

// child returns the value under key or index seg of the node n.
func child(n *yaml.Node, seg string) *yaml.Node {
	r := resolve(n)
	switch {
	case r == nil:
		return nil
	case r.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(r.Content); i += 2 {
			if resolve(r.Content[i]).Value == seg {
				return r.Content[i+1]
			}
		}
	case r.Kind == yaml.SequenceNode:
		if i, err := strconv.Atoi(seg); err == nil && i >= 0 && i < len(r.Content) {
			return r.Content[i]
		}
	}
	return nil
}

// keyNode returns the key node of name in the mapping n, or n.
func keyNode(n *yaml.Node, name string) *yaml.Node {
	if r := resolve(n); r != nil && r.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(r.Content); i += 2 {
			if resolve(r.Content[i]).Value == name {
				return r.Content[i]
			}
		}
	}
	return n
}

// fieldOf returns the field path of the value at path, such as
// "presets[0](getter).max_depth": an item of `presets` carries the preset
// name, and the key that names the preset is not repeated.
func (c *compiler) fieldOf(path []string) field {
	var f field
	n := c.root
	for i := 0; i < len(path); i++ {
		seg := path[i]
		r := resolve(n)
		if r != nil && r.Kind == yaml.SequenceNode {
			idx, _ := strconv.Atoi(seg)
			f = f.index(idx)
			n = child(n, seg)
			if i == 1 && path[0] == "presets" {
				name := presetName(n)
				f = f.named(name)
				if name != "" && i+1 < len(path) && path[i+1] == name {
					n = child(n, name)
					i++
				}
			}
			continue
		}
		f = f.key(seg)
		n = child(n, seg)
	}
	return f
}

// presetName returns the name of the preset in the item n of `presets`,
// or "".
func presetName(n *yaml.Node) string {
	r := resolve(n)
	switch {
	case isString(r):
		return r.Value
	case isMapping(r) && len(r.Content) == 2 && isString(resolve(r.Content[0])):
		return resolve(r.Content[0]).Value
	}
	return ""
}
