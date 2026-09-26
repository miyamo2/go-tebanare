package config

import (
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

// valueOf returns the value of key m in the mapping src.
func valueOf(t *testing.T, src string) *yaml.Node {
	t.Helper()
	root := resolve(parseYAML(t, src))
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "m" {
			return root.Content[i+1]
		}
	}
	t.Fatalf("no key m in %q", src)
	return nil
}

func TestMapping(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantKeys []string
		wantErrs []string
	}{
		{"known keys", "m: {a: 1, b: x}", []string{"a", "b"}, nil},
		{"null values are unset", "m: {a: ~, b: }", nil, nil},
		{"unknown key", "m:\n  a: 1\n  c: 2", []string{"a"}, []string{
			`3:3: m.c: unknown key "c" (allowed keys: a, b)`,
		}},
		{"duplicate key", "m:\n  a: 1\n  b: 2\n  a: 3", []string{"a", "b"}, []string{
			`4:3: m.a: key "a" is already set on line 2`,
		}},
		{"duplicate null key", "m:\n  a:\n  a: 3", nil, []string{
			`3:3: m.a: key "a" is already set on line 2`,
		}},
		{"key that is not a string", "m:\n  a: 1\n  2: x\n  [b]: y", []string{"a"}, []string{
			`3:3: m: keys must be strings, found integer 2`,
			`4:3: m: keys must be strings, found a list`,
		}},
		{"merge key", "base: &b {a: 1}\nm:\n  <<: *b", nil, []string{
			`3:3: m: merge keys (<<) are not supported`,
		}},
		{"aliased key and value", "x: &k a\ny: &v 5\nm: {*k : *v}", []string{"a"}, nil},
		{"list", "m: [a, b]", nil, []string{`1:4: m: expected a mapping, found a list`}},
		{"tagged mapping", "m: !t {a: 1}", nil, []string{`1:4: m: expected a mapping, found a mapping tagged !t`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiler{}
			es, _ := c.mapping(valueOf(t, tt.src), "m", "a", "b")
			var keys []string
			for _, e := range es {
				keys = append(keys, e.name)
				if want := "m." + e.name; string(e.field) != want {
					t.Errorf("field of %s = %q, want %q", e.name, e.field, want)
				}
			}
			if !slices.Equal(keys, tt.wantKeys) {
				t.Errorf("keys = %q, want %q", keys, tt.wantKeys)
			}
			checkDiags(t, "errors", c.errs, tt.wantErrs)
		})
	}
}

func TestScalars(t *testing.T) {
	c := &compiler{}
	es, _ := c.mapping(parseYAML(t, `
s: text
q: "1"
t: True
f: false
n: 1
b: !!bool yes
l: [x]
`), "", "s", "q", "t", "f", "n", "b", "l")
	get := func(name string) entry {
		e, ok := es.get(name)
		if !ok {
			t.Fatalf("missing entry %s", name)
		}
		return e
	}
	if v, ok := c.str(get("s")); !ok || v != "text" {
		t.Errorf("str(s) = %q, %v", v, ok)
	}
	if v, ok := c.str(get("q")); !ok || v != "1" {
		t.Errorf("str(q) = %q, %v", v, ok)
	}
	if v, ok := c.boolean(get("t")); !ok || !v {
		t.Errorf("boolean(t) = %v, %v", v, ok)
	}
	if v, ok := c.boolean(get("f")); !ok || v {
		t.Errorf("boolean(f) = %v, %v", v, ok)
	}
	c.str(get("n"))
	c.boolean(get("s"))
	c.boolean(get("b"))
	c.str(get("l"))
	checkDiags(t, "errors", c.errs, []string{
		`6:4: n: expected a string, found integer 1`,
		`2:4: s: expected a boolean, found string "text"`,
		`7:4: b: expected a boolean, found !!bool "yes"`,
		`8:4: l: expected a string, found a list`,
	})
}

func TestStringList(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		scalarOK bool
		want     []string // value@field
		wantOK   bool
		wantErrs []string
	}{
		{"scalar", "v: a", true, []string{"a@v"}, true, nil},
		{"scalar not allowed", "v: a", false, nil, false, []string{
			`1:4: v: expected a list of strings, found string "a"`,
		}},
		{"list", "v: [a, 'b']", true, []string{"a@v[0]", "b@v[1]"}, true, nil},
		{"empty list", "v: []", false, nil, true, nil},
		{"bad items", "v: [a, 1, {x: y}, c]", true, []string{"a@v[0]", "c@v[3]"}, false, []string{
			`1:8: v[1]: expected a string, found integer 1`,
			`1:11: v[2]: expected a string, found a mapping`,
		}},
		{"mapping", "v: {a: b}", true, nil, false, []string{
			`1:4: v: expected a string or a list of strings, found a mapping`,
		}},
		{"aliases", "x: &x a\nv: [*x, *x]", false, []string{"a@v[0]", "a@v[1]"}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiler{}
			es, _ := c.mapping(parseYAML(t, tt.src), "", "v", "x")
			e, _ := es.get("v")
			items, ok := c.stringList(e, tt.scalarOK)
			var got []string
			for _, it := range items {
				got = append(got, it.value+"@"+string(it.field))
			}
			if !slices.Equal(got, tt.want) || ok != tt.wantOK {
				t.Errorf("stringList = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOK)
			}
			checkDiags(t, "errors", c.errs, tt.wantErrs)
		})
	}
}

func TestSequence(t *testing.T) {
	c := &compiler{}
	items, ok := c.sequence(valueOf(t, "m: [a, {b: c}]"), "m")
	if !ok || len(items) != 2 || items[1].Kind != yaml.MappingNode {
		t.Errorf("sequence = %v, %v", items, ok)
	}
	_, ok = c.sequence(valueOf(t, "m: {a: b}"), "m")
	if ok {
		t.Error("sequence accepts a mapping")
	}
	checkDiags(t, "errors", c.errs, []string{"1:4: m: expected a list, found a mapping"})
}

func TestGlobs(t *testing.T) {
	c := &compiler{}
	es, _ := c.mapping(parseYAML(t, `g: ["**/*.go", "a/[b", "{x,y}/*"]`), "files", "g")
	e, _ := es.get("g")
	got := c.globs(e)
	if want := []string{"**/*.go", "{x,y}/*"}; !slices.Equal(got, want) {
		t.Errorf("globs = %q, want %q", got, want)
	}
	checkDiags(t, "errors", c.errs, []string{`1:16: files.g[1]: invalid glob "a/[b"`})
}
