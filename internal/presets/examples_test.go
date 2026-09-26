package presets

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// stmtKinds maps each stmt preset to the node kind its test data holds.
var stmtKinds = map[string]string{"iferr": "IfStmt"}

// candidates returns the nodes the preset is tried on: every top-level
// function for func presets, and every statement of the preset's kind for
// stmt presets.
func candidates(t *testing.T, p *Preset, f *rule.File) []ast.Node {
	t.Helper()
	var out []ast.Node
	if p.Kind == FuncKind {
		for _, d := range f.AST.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				out = append(out, fd)
			}
		}
		return out
	}
	kind := stmtKinds[p.Name]
	if kind == "" {
		t.Fatalf("stmtKinds has no entry for preset %s", p.Name)
	}
	ast.Inspect(f.AST, func(n ast.Node) bool {
		if n != nil && rule.KindOf(n) == kind {
			out = append(out, n)
		}
		return true
	})
	return out
}

func matches(r *rule.Rule, n ast.Node, f *rule.File) bool {
	if fd, ok := n.(*ast.FuncDecl); ok && r.Func != nil {
		return r.Func.MatchFunc(fd, f)
	}
	if r.Node == nil || !r.Node.Accepts(n) {
		return false
	}
	_, ok := r.Node.MatchNode(n, f)
	return ok
}

func TestExamples(t *testing.T) {
	for _, p := range All() {
		if len(p.Examples) == 0 {
			t.Errorf("%s: no examples", p.Name)
		}
		for _, ex := range p.Examples {
			r := compileYAML(t, p, ex.Settings)
			src := "package p\n\n" + ex.Code + "\n"
			if p.Kind == StmtKind {
				src = "package p\n\nfunc _() {\n" + ex.Code + "\n}\n"
			}
			f := parseSource(t, "example.go", []byte(src))
			nodes := candidates(t, p, f)
			if len(nodes) != 1 {
				t.Errorf("%s: example %q has %d candidates, want 1", p.Name, ex.Code, len(nodes))
				continue
			}
			if got := matches(r, nodes[0], f); got != ex.Match {
				t.Errorf("%s (settings %q): match(%q) = %v, want %v", p.Name, ex.Settings, ex.Code, got, ex.Match)
			}
		}
	}
}

// TestPresets checks every registered preset: its descriptive fields and
// the rule compiled from the default settings.
func TestPresets(t *testing.T) {
	for _, p := range All() {
		if p.Name == "" || p.Summary == "" || len(p.Criteria) == 0 || p.NewSettings == nil || p.Compile == nil {
			t.Errorf("%s: incomplete preset %+v", p.Name, p)
			continue
		}
		texts := append([]string{p.Summary}, p.Criteria...)
		for _, s := range Settings(p) {
			texts = append(texts, s.Description)
		}
		for _, ex := range p.Examples {
			if ex.Note != "" {
				texts = append(texts, ex.Note)
			}
		}
		for _, s := range texts {
			if !strings.HasSuffix(s, ".") || strings.ContainsAny(s, "\u2013\u2014") {
				t.Errorf("%s: %q must end with a period and hold no en or em dash", p.Name, s)
			}
		}

		r, err := p.Compile(p.NewSettings())
		if err != nil {
			t.Errorf("%s: Compile(defaults): %v", p.Name, err)
			continue
		}
		if r.ID != p.Name || r.Preset != p.Name || r.Description != p.Summary || r.Target != p.Kind.Target() {
			t.Errorf("%s: rule %+v", p.Name, r)
		}
		if (r.Func != nil) != (p.Kind == FuncKind) || (r.Node != nil) != (p.Kind == StmtKind) {
			t.Errorf("%s: rule has Func %v and Node %v", p.Name, r.Func, r.Node)
		}
		if _, err := p.Compile(struct{}{}); err == nil {
			t.Errorf("%s: Compile(struct{}{}) succeeded", p.Name)
		}
		if _, err := p.Compile(nil); err == nil {
			t.Errorf("%s: Compile(nil) succeeded", p.Name)
		}
	}
}
