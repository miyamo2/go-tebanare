package presets

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// compileYAML decodes settings from YAML ("" for defaults) and compiles p.
func compileYAML(t *testing.T, p *Preset, settings string) *rule.Rule {
	t.Helper()
	var node *yaml.Node
	if settings != "" {
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(settings), &doc); err != nil {
			t.Fatalf("settings %q: %v", settings, err)
		}
		node = &doc
	}
	s, err := Decode(p, node)
	if err != nil {
		t.Fatalf("Decode(%q): %v", settings, err)
	}
	r, err := p.Compile(s)
	if err != nil {
		t.Fatalf("Compile(%q): %v", settings, err)
	}
	return r
}

// parseSource parses src the way the analyzer does.
func parseSource(t *testing.T, name string, src []byte) *rule.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return &rule.File{Path: name, Fset: fset, AST: f, Src: src}
}

// funcResults compiles preset name with settings (YAML) and matches every
// function in src (after a package clause). The key is the function name.
func funcResults(t *testing.T, name, settings, src string) map[string]bool {
	t.Helper()
	p, ok := Lookup(name)
	if !ok {
		t.Fatalf("preset %s is not registered", name)
	}
	r := compileYAML(t, p, settings)
	f := parseSource(t, "x.go", []byte("package p\n\n"+src))
	got := map[string]bool{}
	for _, d := range f.AST.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			got[fd.Name.Name] = r.Func.MatchFunc(fd, f)
		}
	}
	return got
}

func checkResults(t *testing.T, got, want map[string]bool) {
	t.Helper()
	for name, w := range want {
		if g, ok := got[name]; !ok || g != w {
			t.Errorf("%s: match = %v, want %v", name, g, w)
		}
	}
}
