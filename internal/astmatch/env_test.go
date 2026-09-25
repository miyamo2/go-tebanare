package astmatch

import (
	"go/ast"
	"testing"
)

func idents(names ...string) []*ast.Ident {
	var out []*ast.Ident
	for _, n := range names {
		out = append(out, ast.NewIdent(n))
	}
	return out
}

func TestEnvBind(t *testing.T) {
	e := NewEnv(idents("A", "B", "_"))
	if !e.IsTypeParam("A") || e.IsTypeParam("_") || e.IsTypeParam("C") {
		t.Fatal("IsTypeParam")
	}
	steps := []struct {
		pat, src string
		want     bool
	}{
		{"T", "C", false}, // not a source type parameter
		{"T", "A", true},
		{"T", "A", true},  // same binding again
		{"T", "B", false}, // T is bound to A
		{"U", "A", false}, // A is bound to T
		{"U", "B", true},
		{"V", "_", true},
		{"W", "_", true}, // "_" never conflicts
	}
	for _, s := range steps {
		if got := e.Bind(s.pat, s.src); got != s.want {
			t.Errorf("Bind(%s, %s) = %v, want %v", s.pat, s.src, got, s.want)
		}
	}
	if src, ok := e.Lookup("U"); !ok || src != "B" {
		t.Errorf("Lookup(U) = %q, %v", src, ok)
	}
	if _, ok := e.Lookup("X"); ok {
		t.Error("Lookup(X) found a binding")
	}
}

func TestEnvSnapshot(t *testing.T) {
	e := NewEnv(idents("A", "B"))
	e.Bind("T", "A")
	snap := e.Snapshot()
	e.Bind("U", "B")
	e.Restore(snap)
	if _, ok := e.Lookup("U"); ok {
		t.Error("Restore kept U")
	}
	if _, ok := e.Lookup("T"); !ok {
		t.Error("Restore dropped T")
	}
	e.Restore(-1)
	e.Restore(100)
	if e.Snapshot() != snap {
		t.Error("out-of-range Restore changed the bindings")
	}
}

func TestEnvZeroValue(t *testing.T) {
	var e Env
	if e.Bind("T", "A") {
		t.Error("zero Env bound a name")
	}
	if e.IsTypeParam("A") {
		t.Error("zero Env has a type parameter")
	}
}
