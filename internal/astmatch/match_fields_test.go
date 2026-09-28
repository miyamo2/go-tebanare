package astmatch

import (
	"go/ast"
	"testing"
)

func TestMatchFields(t *testing.T) {
	tests := []struct {
		pat, src string // function types; their parameter lists are compared
		want     bool
	}{
		{"func(SEQ)", "func()", true},
		{"func(SEQ)", "func(a, b int, c ...string)", true},
		{"func(_)", "func(int)", true},
		{"func(_)", "func(...int)", false},
		{"func(_)", "func()", false},
		{"func(_)", "func(a, b int)", false},
		{"func(..._)", "func(args ...any)", true},
		{"func(..._)", "func(args []any)", false},
		{"func(..._)", "func(arg any)", false},
		{"func(string, _)", "func(format string, arg any)", true},
		{"func(string, _)", "func(format string, args ...any)", false},
		{"func(string, ..._)", "func(format string, args ...any)", true},
		{"func(string, SEQ)", "func(format string, args ...any)", true},
		{"func(int, int)", "func(a, b int)", true},
		{"func(SEQ, error)", "func(a int, err error)", true},
		{"func(SEQ, error)", "func(err error, a int)", false},
		{"func(SEQ, int, SEQ, string, SEQ)", "func(a, b int, s string, c bool)", true},
		{"func(SEQ, int, SEQ, string, SEQ)", "func(s string, a int)", false},
		{"func(SEQ, SEQ)", "func(a int)", true},
		{"func()", "func(a int)", false},
	}
	for _, tt := range tests {
		p := pat(t, tt.pat).(*ast.FuncType)
		s := typeExpr(t, tt.src).(*ast.FuncType)
		if got := MatchFields(p.Params, s.Params, nil); got != tt.want {
			t.Errorf("MatchFields(%s, %s) = %v, want %v", tt.pat, tt.src, got, tt.want)
		}
	}
	if !MatchFields(nil, &ast.FieldList{}, nil) || !MatchFields(&ast.FieldList{}, nil, nil) {
		t.Error("nil and empty lists differ")
	}
}

func TestMatchExprs(t *testing.T) {
	list := func(srcs ...string) []ast.Expr {
		var out []ast.Expr
		for _, s := range srcs {
			out = append(out, pat(t, s))
		}
		return out
	}
	tests := []struct {
		pat, src []ast.Expr
		want     bool
	}{
		{list("SEQ"), nil, true},
		{list("SEQ", "int"), list("string", "int"), true},
		{list("SEQ", "int"), list("int", "string"), false},
		{list("_", "_"), list("int", "string"), true},
		{list("_", "_"), list("int"), false},
		{nil, list("int"), false},
	}
	for i, tt := range tests {
		if got := MatchExprs(tt.pat, tt.src, nil); got != tt.want {
			t.Errorf("case %d: MatchExprs = %v, want %v", i, got, tt.want)
		}
	}
}
