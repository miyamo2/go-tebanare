package config

import (
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// value converts the YAML tree under n into the JSON value that the
// schema validates: map[string]any, []any, string, bool, int64,
// json.Number (an integer outside the int64 range), float64, or nil. It
// follows aliases; checkAliases has bounded them and ruled out cycles.
//
// It reports what JSON cannot hold or what the YAML spells in a way the
// configuration does not accept: a key set twice, a merge key, a key
// that is not a string, and a value with a tag other than the standard
// ones, or with a tag that does not fit it. It returns false after any
// such error.
func (c *compiler) value(n *yaml.Node, path []string) (any, bool) {
	r := resolve(n)
	switch {
	case r == nil || isNull(r):
		return nil, true
	case r.Kind == yaml.MappingNode && r.ShortTag() == "!!map":
		return c.mappingValue(r, path)
	case r.Kind == yaml.SequenceNode && r.ShortTag() == "!!seq":
		ok := true
		out := make([]any, 0, len(r.Content))
		for i, item := range r.Content {
			v, itemOK := c.value(item, append(path, strconv.Itoa(i)))
			ok = ok && itemOK
			out = append(out, v)
		}
		return out, ok
	case r.Kind == yaml.ScalarNode:
		if v, ok := scalarValue(r); ok {
			return v, true
		}
	}
	c.errorf(n, c.fieldOf(path), "unsupported value: %s", describe(r))
	return nil, false
}

func (c *compiler) mappingValue(m *yaml.Node, path []string) (any, bool) {
	ok := true
	out := map[string]any{}
	lines := map[string]int{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		rk := resolve(k)
		switch {
		case rk.Kind == yaml.ScalarNode && rk.ShortTag() == "!!merge":
			c.errorf(k, c.fieldOf(path), "merge keys (<<) are not supported")
			ok = false
			continue
		case !isString(rk):
			c.errorf(k, c.fieldOf(path), "keys must be strings, found %s", describe(rk))
			ok = false
			continue
		}
		name := rk.Value
		if line, dup := lines[name]; dup {
			c.errorf(k, c.fieldOf(append(path, name)), "key %q is already set on line %d", name, line)
			ok = false
			continue
		}
		lines[name] = k.Line
		val, valOK := c.value(v, append(path, name))
		ok = ok && valOK
		out[name] = val
	}
	return out, ok
}

// scalarValue returns the JSON value of the scalar n, or false when its
// tag is not one of the standard ones or does not fit its value.
func scalarValue(n *yaml.Node) (any, bool) {
	switch effectiveTag(n) {
	case "!!str":
		return n.Value, true
	case "!!bool":
		// The !!bool scalars are true, True, TRUE, and the same for false.
		return strings.EqualFold(n.Value, "true"), true
	case "!!int":
		// yaml.v3 reads integers the same way: underscores removed, then
		// strconv with base prefixes.
		text := strings.ReplaceAll(n.Value, "_", "")
		if i, err := strconv.ParseInt(text, 0, 64); err == nil {
			return i, true
		}
		if b, ok := new(big.Int).SetString(text, 0); ok {
			return json.Number(b.String()), true
		}
	case "!!float":
		// .inf, .nan, and floats out of range are not JSON numbers.
		f, err := strconv.ParseFloat(strings.ReplaceAll(n.Value, "_", ""), 64)
		if err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return f, true
		}
	}
	return nil, false
}
