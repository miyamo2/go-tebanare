package presets

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// parseNode parses src and returns its document node.
func parseNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("yaml %q: %v", src, err)
	}
	return &doc
}

func TestResolve(t *testing.T) {
	doc := parseNode(t, "a: &x 1\nb: *x")
	m := doc.Content[0]
	if got := resolve(doc); got != m {
		t.Errorf("resolve(document) = %v, want the mapping", got)
	}
	if got := resolve(m.Content[3]); got != m.Content[1] {
		t.Errorf("resolve(alias) = %v, want the anchored scalar", got)
	}
	for _, n := range []*yaml.Node{nil, {}, parseNode(t, "")} {
		if got := resolve(n); got != nil {
			t.Errorf("resolve(%v) = %v, want nil", n, got)
		}
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct{ src, want string }{
		{"{}", "a mapping"},
		{"!!map {}", "a mapping"},
		{"!!str {}", "a mapping tagged !!str"},
		{"[]", "a list"},
		{"! []", "a list"},
		{"!foo []", "a list tagged !foo"},
		{"~", "null"},
		{"x", `string "x"`},
		{"'1'", `string "1"`},
		{"!!str 1", `string "1"`},
		{"true", "boolean true"},
		{"!!int 0x10", "integer 0x10"},
		{"1.5", "float 1.5"},
		{"!!int abc", `!!int "abc"`},
		{"!!bool 1", `!!bool "1"`},
		{"!foo bar", `!foo "bar"`},
		{strings.Repeat("é", 41), `string "` + strings.Repeat("é", 40) + `..."`},
	}
	for _, tt := range tests {
		if got := describe(resolve(parseNode(t, tt.src))); got != tt.want {
			t.Errorf("describe(%s) = %q, want %q", tt.src, got, tt.want)
		}
	}
	if got := describe(&yaml.Node{Kind: yaml.AliasNode}); got != "an unsupported node" {
		t.Errorf("describe(alias) = %q", got)
	}
}
