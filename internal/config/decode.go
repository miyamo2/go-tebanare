package config

import (
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"
)

// entry is one key of a mapping and its value.
type entry struct {
	name string
	key  *yaml.Node
	// value is the node as written, which may be an alias. Errors are
	// reported at its position; resolve it to read the value.
	value *yaml.Node
	field field
}

// entries holds the keys of a mapping that have a value.
type entries []entry

// get returns the entry for key name.
func (es entries) get(name string) (entry, bool) {
	for _, e := range es {
		if e.name == name {
			return e, true
		}
	}
	return entry{}, false
}

// mapping checks that n is a mapping whose keys are strings from allowed,
// each written once, and returns its entries in source order. A null value
// counts as unset, so its entry is left out, as are entries with a bad
// key. ok is false when n is not a mapping.
func (c *compiler) mapping(n *yaml.Node, f field, allowed ...string) (es entries, ok bool) {
	m := resolve(n)
	if !isMapping(m) {
		c.errorf(n, f, "expected a mapping, found %s", describe(m))
		return nil, false
	}
	lines := map[string]int{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		rk := resolve(k)
		switch {
		case rk.Kind == yaml.ScalarNode && rk.ShortTag() == "!!merge":
			c.errorf(k, f, "merge keys (<<) are not supported")
			continue
		case !isString(rk):
			c.errorf(k, f, "keys must be strings, found %s", describe(rk))
			continue
		}
		name := rk.Value
		kf := f.key(name)
		if !slices.Contains(allowed, name) {
			c.errorf(k, kf, "unknown key %q (allowed keys: %s)", name, strings.Join(allowed, ", "))
			continue
		}
		if line, dup := lines[name]; dup {
			c.errorf(k, kf, "key %q is already set on line %d", name, line)
			continue
		}
		lines[name] = k.Line
		if isNull(resolve(v)) {
			continue
		}
		es = append(es, entry{name: name, key: k, value: v, field: kf})
	}
	return es, true
}

// sequence checks that n is a list and returns its items as written.
func (c *compiler) sequence(n *yaml.Node, f field) ([]*yaml.Node, bool) {
	s := resolve(n)
	if !isSequence(s) {
		c.errorf(n, f, "expected a list, found %s", describe(s))
		return nil, false
	}
	return s.Content, true
}

// str returns the value of e, which must be a string.
func (c *compiler) str(e entry) (string, bool) {
	n := resolve(e.value)
	if !isString(n) {
		c.errorf(e.value, e.field, "expected a string, found %s", describe(n))
		return "", false
	}
	return n.Value, true
}

// boolean returns the value of e, which must be true or false.
func (c *compiler) boolean(e entry) (value, ok bool) {
	n := resolve(e.value)
	if n == nil || n.Kind != yaml.ScalarNode || effectiveTag(n) != "!!bool" {
		c.errorf(e.value, e.field, "expected a boolean, found %s", describe(n))
		return false, false
	}
	// The !!bool scalars are true, True, TRUE, and the same for false.
	return strings.EqualFold(n.Value, "true"), true
}

// item is one string of a string list, with its node and path.
type item struct {
	value string
	node  *yaml.Node
	field field
}

// stringList reads a list of strings. With scalarOK, a single string is
// accepted as a list of one item whose path has no index. Items that are
// not strings are reported and left out; ok is false after any error.
func (c *compiler) stringList(e entry, scalarOK bool) (items []item, ok bool) {
	n := resolve(e.value)
	if scalarOK && isString(n) {
		return []item{{value: n.Value, node: e.value, field: e.field}}, true
	}
	if !isSequence(n) {
		want := "a list of strings"
		if scalarOK {
			want = "a string or a list of strings"
		}
		c.errorf(e.value, e.field, "expected %s, found %s", want, describe(n))
		return nil, false
	}
	ok = true
	for i, raw := range n.Content {
		it := resolve(raw)
		if !isString(it) {
			c.errorf(raw, e.field.index(i), "expected a string, found %s", describe(it))
			ok = false
			continue
		}
		items = append(items, item{value: it.Value, node: raw, field: e.field.index(i)})
	}
	return items, ok
}

// globs reads a list of doublestar patterns.
func (c *compiler) globs(e entry) []string {
	items, _ := c.stringList(e, false)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if !doublestar.ValidatePattern(it.value) {
			c.errorf(it.node, it.field, "invalid glob %q", it.value)
			continue
		}
		out = append(out, it.value)
	}
	return out
}
