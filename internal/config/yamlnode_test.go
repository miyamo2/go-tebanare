package config

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestResolveCycle(t *testing.T) {
	// An anchor on a list that contains an alias to itself.
	n := resolve(parseYAML(t, "&a [*a]"))
	if n == nil || n.Kind != yaml.SequenceNode {
		t.Fatalf("resolve = %v", n)
	}
	if got := resolve(n.Content[0]); got != n {
		t.Errorf("resolve(alias) = %v, want the list", got)
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct{ src, want string }{
		{"~", "null"},
		{"abc", `string "abc"`},
		{"'1'", `string "1"`},
		{"1", "integer 1"},
		{"1.5", "float 1.5"},
		{"true", "boolean true"},
		{"!!int abc", `!!int "abc"`},
		{"[a]", "a list"},
		{"!!set {a}", "a mapping tagged !!set"},
		{strings.Repeat("x", 50), `string "` + strings.Repeat("x", 40) + `..."`},
	}
	for _, tt := range tests {
		if got := describe(resolve(parseYAML(t, tt.src))); got != tt.want {
			t.Errorf("describe(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
	if got := describe(nil); got != "nothing" {
		t.Errorf("describe(nil) = %q", got)
	}
}

func TestPredicates(t *testing.T) {
	root := resolve(parseYAML(t, "m: {}\nl: []\ns: x\nq: '~'\nn: ~\nt: !t [x]\na: &a [y]\nr: *a"))
	get := func(key string) *yaml.Node {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == key {
				return resolve(root.Content[i+1])
			}
		}
		t.Fatalf("no key %s", key)
		return nil
	}
	tests := []struct {
		key                string
		mapping, str, null bool
	}{
		{"m", true, false, false},
		{"l", false, false, false},
		{"s", false, true, false},
		{"q", false, true, false},
		{"n", false, false, true},
		{"t", false, false, false},
		{"r", false, false, false},
	}
	for _, tt := range tests {
		n := get(tt.key)
		if isMapping(n) != tt.mapping || isString(n) != tt.str || isNull(n) != tt.null {
			t.Errorf("%s: mapping %v, string %v, null %v", tt.key, isMapping(n), isString(n), isNull(n))
		}
	}
	if isMapping(nil) || isString(nil) || isNull(nil) {
		t.Error("a predicate accepts nil")
	}
}
