package presets

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Decode reads the settings of p from node, the value that follows the
// preset name in the config. A nil node or a YAML null gives the defaults.
// A setting whose value is null keeps its default.
//
// Decode is strict: node must be a mapping, every key must name a setting
// of p, and every value must have the setting's type. The error wraps one
// *DecodeError per problem (see errors.Join), with the YAML position of the
// key or value at fault, in source order. An unknown key's error lists the
// available settings.
func Decode(p *Preset, node *yaml.Node) (any, error) {
	settings := p.NewSettings()
	n := resolve(node)
	if n == nil || isNull(n) {
		return settings, nil
	}
	if n.Kind != yaml.MappingNode || n.ShortTag() != "!!map" {
		at := node
		if at.Kind == yaml.DocumentNode {
			at = n
		}
		return nil, errors.Join(errorAt(at, "", "settings must be a mapping, found "+describe(n)))
	}

	infos := Settings(p)
	var errs []*DecodeError
	values := map[string]*yaml.Node{}
	keyLines := map[string]int{}
	// partial maps a list setting with items of the wrong type to the
	// indexes in the YAML list of the items that were decoded.
	partial := map[string][]int{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		rk := resolve(k)
		if rk.Kind != yaml.ScalarNode || effectiveTag(rk) != "!!str" {
			errs = append(errs, errorAt(k, "", "setting names must be strings, found "+describe(rk)))
			continue
		}
		name := rk.Value
		idx := slices.IndexFunc(infos, func(s SettingInfo) bool { return s.Name == name })
		if idx < 0 {
			errs = append(errs, errorAt(k, name, fmt.Sprintf("unknown setting %q (available settings: %s)",
				name, strings.Join(settingNames(infos), ", "))))
			continue
		}
		if line, dup := keyLines[name]; dup {
			errs = append(errs, errorAt(k, name, fmt.Sprintf("setting %q is already set on line %d", name, line)))
			continue
		}
		keyLines[name] = k.Line
		values[name] = v
		clean, kept, typeErrs := cleanValue(infos[idx], v)
		errs = append(errs, typeErrs...)
		if clean == nil {
			continue // the setting keeps its default
		}
		if len(typeErrs) > 0 {
			// Some items of the list are not strings. Decode the others,
			// so the checks below report their problems in the same pass.
			partial[name] = kept
		}
		one := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, clean,
		}}
		// cleanValue checked the type and range, so this decode does not
		// fail for the supported setting types.
		if err := one.Decode(settings); err != nil {
			errs = append(errs, errorAt(v, name, err.Error()))
		}
	}

	// A setting with a value of the wrong type keeps its valid default,
	// and a list keeps its string items, so the checks below report only
	// problems with the decoded values.
	if val, ok := settings.(validator); ok {
		for _, pr := range val.validate() {
			if kept, ok := partial[pr.key]; ok {
				// The list holds only its string items, so checks of the
				// list as a whole do not apply.
				if pr.index < 0 || pr.index >= len(kept) {
					continue
				}
				pr.index = kept[pr.index]
			}
			at := n
			if v := values[pr.key]; v != nil {
				at = v
				if rv := resolve(v); pr.index >= 0 && rv.Kind == yaml.SequenceNode && pr.index < len(rv.Content) {
					at = rv.Content[pr.index]
				}
			}
			errs = append(errs, errorAt(at, pr.field(), pr.msg))
		}
	}
	if len(errs) > 0 {
		slices.SortStableFunc(errs, func(a, b *DecodeError) int {
			return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
		})
		joined := make([]error, len(errs))
		for i, e := range errs {
			joined[i] = e
		}
		return nil, errors.Join(joined...)
	}
	return settings, nil
}

func settingNames(infos []SettingInfo) []string {
	names := make([]string, len(infos))
	for i, s := range infos {
		names[i] = s.Name
	}
	return names
}

func errorAt(n *yaml.Node, field, msg string) *DecodeError {
	e := &DecodeError{Field: field, Msg: msg}
	if n != nil {
		e.Line, e.Column = n.Line, n.Column
	}
	return e
}

// cleanValue checks that v has the type of the setting and returns a copy
// without aliases or custom tags. yaml.v3 decodes such a copy without
// reaching its error paths that panic, which TinyGo cannot recover from.
// It returns nil for null and for a value of the wrong type.
//
// A list with items that are not strings still gives a copy of its string
// items, with one error for each other item. kept holds the index in v of
// each item of the copy.
func cleanValue(info SettingInfo, v *yaml.Node) (clean *yaml.Node, kept []int, errs []*DecodeError) {
	rv := resolve(v)
	if isNull(rv) {
		return nil, nil, nil
	}
	wrongType := func() []*DecodeError {
		return []*DecodeError{errorAt(v, info.Name, "expected "+expected(info.Type)+", found "+describe(rv))}
	}
	scalar := func(n *yaml.Node, tag string) *yaml.Node {
		if n.Kind != yaml.ScalarNode || effectiveTag(n) != tag {
			return nil
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: n.Value}
	}
	switch info.Type {
	case "bool", "int", "string":
		tag := map[string]string{"bool": "!!bool", "int": "!!int", "string": "!!str"}[info.Type]
		c := scalar(rv, tag)
		if c == nil {
			return nil, nil, wrongType()
		}
		if info.Type == "int" && !inInt32(c) {
			return nil, nil, []*DecodeError{errorAt(v, info.Name, fmt.Sprintf(
				"expected an integer from %d to %d, found %s", math.MinInt32, math.MaxInt32, describe(rv)))}
		}
		if len(info.Enum) > 0 && !slices.Contains(info.Enum, c.Value) {
			return nil, nil, []*DecodeError{errorAt(v, info.Name, fmt.Sprintf("unknown value %q (valid values: %s)",
				c.Value, strings.Join(info.Enum, ", ")))}
		}
		return c, nil, nil
	case "[]string":
		if rv.Kind != yaml.SequenceNode || rv.ShortTag() != "!!seq" {
			return nil, nil, wrongType()
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for i, item := range rv.Content {
			c := scalar(resolve(item), "!!str")
			if c == nil {
				errs = append(errs, errorAt(item, fmt.Sprintf("%s[%d]", info.Name, i),
					"expected a string, found "+describe(resolve(item))))
				continue
			}
			seq.Content = append(seq.Content, c)
			kept = append(kept, i)
		}
		return seq, kept, errs
	}
	return nil, nil, []*DecodeError{errorAt(v, info.Name, "unsupported setting type "+info.Type)}
}

// inInt32 reports whether the integer scalar n fits in an int32. Integer
// settings stop at that range so that builds where int has 32 bits (TinyGo
// on wasm) and 64 bits accept the same values.
func inInt32(n *yaml.Node) bool {
	var i int64
	return n.Decode(&i) == nil && i >= math.MinInt32 && i <= math.MaxInt32
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && effectiveTag(n) == "!!null"
}

func expected(typ string) string {
	switch typ {
	case "bool":
		return "a boolean"
	case "int":
		return "an integer"
	case "string":
		return "a string"
	case "[]string":
		return "a list of strings"
	}
	return typ
}
