package astmatch

import (
	"go/ast"
	"strings"
	"testing"
)

func TestMatchList(t *testing.T) {
	// Elements are identifiers; "SEQ" is Seq and every other name must be
	// equal.
	list := func(s string) []ast.Expr {
		var out []ast.Expr
		for _, n := range strings.Fields(s) {
			if n == "SEQ" {
				out = append(out, Seq())
			} else {
				out = append(out, ast.NewIdent(n))
			}
		}
		return out
	}
	eq := func(p, s ast.Expr) bool { return p.(*ast.Ident).Name == s.(*ast.Ident).Name }
	tests := []struct {
		pat, src string
		want     bool
	}{
		{"", "", true},
		{"", "a", false},
		{"a", "", false},
		{"SEQ", "", true},
		{"SEQ", "a b c", true},
		{"a SEQ", "a b c", true},
		{"SEQ c", "a b c", true},
		{"SEQ c", "a b", false},
		{"a SEQ c", "a c", true},
		{"a SEQ b SEQ c", "a x b y b z c", true},
		{"a SEQ b SEQ c", "a x c y b", false},
		{"SEQ SEQ", "a", true},
		{"a b", "a b", true},
		{"a b", "b a", false},
	}
	for _, tt := range tests {
		if got := matchList(list(tt.pat), list(tt.src), &Env{}, eq); got != tt.want {
			t.Errorf("matchList(%q, %q) = %v, want %v", tt.pat, tt.src, got, tt.want)
		}
	}
}

func TestExpandFields(t *testing.T) {
	a, b := ast.NewIdent("A"), ast.NewIdent("B")
	fl := &ast.FieldList{List: []*ast.Field{
		{Names: []*ast.Ident{ast.NewIdent("x"), ast.NewIdent("y")}, Type: a},
		{Type: b},
	}}
	got := expandFields(fl)
	if len(got) != 3 || got[0] != a || got[1] != a || got[2] != b {
		t.Errorf("expandFields = %v", got)
	}
	if expandFields(nil) != nil {
		t.Error("expandFields(nil) is not empty")
	}
}
