package gogen

import (
	"flag"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the generated Go files")

// repoRoot is the repository root, relative to this package.
var repoRoot = filepath.Join("..", "..")

// TestGeneratedUpToDate checks that the generated Go files are the output
// of Generate. With -update it rewrites them.
func TestGeneratedUpToDate(t *testing.T) {
	files, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		path := filepath.Join(repoRoot, name)
		if *update {
			if err := os.WriteFile(path, []byte(files[name]), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run make generate to create it)", err)
		}
		if string(got) != files[name] {
			t.Errorf("%s is out of date (run make generate to regenerate it)", name)
		}
	}
}

func TestGenerateErrors(t *testing.T) {
	base := `{"properties": {"version": {"type": "integer"}, "presets": {"type": "array", "items": {"anyOf": [{"type": "string", "enum": ["p"]}]}}},
		"definitions": {"pSettings": {"type": "object", "properties": {"x": %s}}}}`
	tests := []struct {
		name, prop, want string
	}{
		{"no default sentence", `{"type": "boolean", "description": "X.", "markdownDescription": "X."}`, `does not end with "Default: <text>."`},
		{"description differs", `{"type": "boolean", "description": "X. Default: false.", "markdownDescription": "Y"}`, "markdownDescription differ"},
		{"unsupported type", `{"type": "number", "description": "X. Default: 1.", "markdownDescription": "X. Default: 1."}`, "unsupported type"},
		{"default of the wrong type", `{"type": "boolean", "default": 1, "description": "X. Default: 1.", "markdownDescription": "X. Default: 1."}`, "is not a boolean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := strings.Replace(base, "%s", tt.prop, 1)
			if _, err := generate([]byte(src)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("generate = %v, want an error containing %q", err, tt.want)
			}
		})
	}
	noQ := strings.Replace(strings.Replace(base, "%s", `{"type": "boolean", "description": "X. Default: false.", "markdownDescription": "X. Default: false."}`, 1),
		`"enum": ["p"]`, `"enum": ["p", "q"]`, 1)
	if _, err := generate([]byte(noQ)); err == nil ||
		!strings.Contains(err.Error(), "no definitions/qSettings") {
		t.Errorf("a preset without settings: %v", err)
	}
}
