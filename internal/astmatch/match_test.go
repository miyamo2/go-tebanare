package astmatch

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestMatchType(t *testing.T) {
	tests := []struct {
		pat, src string
		tparams  []string          // source type parameters
		binds    map[string]string // pattern name to source name
		quals    map[string]string
		want     bool
	}{
		// Identifiers and type parameters.
		{pat: "int", src: "int", want: true},
		{pat: "int", src: "string"},
		{pat: "T", src: "T", want: true},
		{pat: "T", src: "T", tparams: []string{"T"}},
		{pat: "TP_T", src: "E", tparams: []string{"E"}, binds: map[string]string{"T": "E"}, want: true},
		{pat: "TP_T", src: "E", tparams: []string{"E"}},
		{pat: "TP_T", src: "F", tparams: []string{"E", "F"}, binds: map[string]string{"T": "E"}},
		{pat: "TP_T", src: "*E", tparams: []string{"E"}, binds: map[string]string{"T": "E"}},

		// any and interface{}.
		{pat: "any", src: "interface{}", want: true},
		{pat: "interface{}", src: "any", want: true},
		{pat: "interface{}", src: "interface{}", want: true},
		{pat: "any", src: "any", tparams: []string{"any"}},
		{pat: "interface{}", src: "any", tparams: []string{"any"}},
		{pat: "any", src: "interface{ M() }"},

		// Any.
		{pat: "_", src: "map[string][]int", want: true},
		{pat: "_", src: "...int"},
		{pat: "[]_", src: "[]*T", want: true},

		// Parentheses.
		{pat: "(int)", src: "int", want: true},
		{pat: "int", src: "((int))", want: true},
		{pat: "*(T)", src: "(*T)", want: true},

		// Qualified identifiers.
		{pat: "context.Context", src: "context.Context", want: true},
		{pat: "context.Context", src: "ctx2.Context"},
		{pat: "context.Context", src: "Context"},
		{pat: "context.Context", src: "ctx2.Context", quals: map[string]string{"ctx2": "context"}, want: true},
		{pat: "ctx2.Context", src: "ctx2.Context", quals: map[string]string{"ctx2": "context"}, want: true},
		{pat: "context.Context", src: "context.Other"},

		// Pointers, slices, arrays, maps, channels.
		{pat: "*T", src: "*T", want: true},
		{pat: "*T", src: "T"},
		{pat: "[]int", src: "[]int", want: true},
		{pat: "[]int", src: "[4]int"},
		{pat: "[4]int", src: "[4]int", want: true},
		{pat: "[4]int", src: "[5]int"},
		{pat: "[N/2]int", src: "[N / 2]int", want: true},
		{pat: "[N/2]int", src: "[(N/2)]int"},
		{pat: "[_]int", src: "[8]int", want: true},
		{pat: "[_]int", src: "[]int"},
		{pat: "map[string]int", src: "map[string]int", want: true},
		{pat: "map[string]int", src: "map[string]bool"},
		{pat: "chan int", src: "chan int", want: true},
		{pat: "chan int", src: "<-chan int"},
		{pat: "chan<- int", src: "chan<- int", want: true},
		{pat: "<-chan int", src: "chan<- int"},

		// Function types.
		{pat: "func(int) error", src: "func(x int) error", want: true},
		{pat: "func(int) error", src: "func(int)"},
		{pat: "func(SEQ) SEQ", src: "func(a, b int) (int, error)", want: true},
		{pat: "func(..._)", src: "func(...int)", want: true},

		// Interfaces.
		{pat: "interface{ M() int }", src: "interface{ M() int }", want: true},
		{pat: "interface{ M() int }", src: "interface{ N() int }"},
		{pat: "interface{ M(); N() }", src: "interface{ N(); M() }"},
		{pat: "interface{ io.Reader; M() }", src: "interface{ io.Reader; M() }", want: true},
		{pat: "interface{ ~int | ~string }", src: "interface{ ~int | ~string }", want: true},
		{pat: "interface{ ~int | ~string }", src: "interface{ ~string | ~int }"},
		{pat: "interface{ ~int }", src: "interface{ int }"},
		{pat: "interface{ M() }", src: "interface{ M(); N() }"},
		{pat: "interface{ M() }", src: "interface{ io.Reader }"},

		// Structs.
		{pat: "struct{ a, b int }", src: "struct{ a int; b int }", want: true},
		{pat: "struct{ a, b int }", src: "struct{ a, c int }"},
		{pat: "struct{ a int \"x\" }", src: "struct{ a int \"x\" }", want: true},
		{pat: "struct{ a int \"x\" }", src: "struct{ a int `x` }"},
		{pat: "struct{ a int }", src: "struct{ a int \"x\" }"},
		{pat: "struct{ a int }", src: "struct{ a, b int }"},
		{pat: "struct{ io.Reader }", src: "struct{ io.Reader }", want: true},
		{pat: "struct{ _ }", src: "struct{ io.Reader }", want: true},

		// Instances.
		{pat: "List[int]", src: "List[int]", want: true},
		{pat: "List[int]", src: "List"},
		{pat: "List", src: "List[int]"},
		{pat: "List[SEQ]", src: "List", want: true},
		{pat: "List[SEQ]", src: "List[int, string]", want: true},
		{pat: "Map[_, int]", src: "Map[string, int]", want: true},
		{pat: "Map[_, int]", src: "Map[string, bool]"},
		{pat: "pkg.List[int]", src: "pkg.List[int]", want: true},
		{pat: "pkg.List[SEQ]", src: "pkg.List", want: true},
		{pat: "_[SEQ]", src: "List", want: true},
		{pat: "_[SEQ]", src: "List[int]", want: true},
		{pat: "_[SEQ]", src: "[]int"},
		{pat: "_[SEQ]", src: "*List"},
		{pat: "_[SEQ]", src: "map[string]int"},
		{pat: "_[SEQ]", src: "(List)", want: true},
		{pat: "_[SEQ]", src: "T", tparams: []string{"T"}},

		// Variadic elements.
		{pat: "...int", src: "...int", want: true},
		{pat: "..._", src: "...string", want: true},
		{pat: "int", src: "...int"},

		// Expressions that are not types.
		{pat: "f()", src: "f()"},
		{pat: "a + b", src: "a + b"},
	}
	for _, tt := range tests {
		env := NewEnv(idents(tt.tparams...))
		for p, s := range tt.binds {
			if !env.Bind(p, s) {
				t.Fatalf("%s: Bind(%s, %s) failed", tt.pat, p, s)
			}
		}
		env.Qualifiers(tt.quals)
		if got := MatchType(pat(t, tt.pat), typeExpr(t, tt.src), env); got != tt.want {
			t.Errorf("MatchType(%s, %s) = %v, want %v", tt.pat, tt.src, got, tt.want)
		}
	}
}

func TestMatchTypeNil(t *testing.T) {
	if !MatchType(ast.NewIdent("int"), ast.NewIdent("int"), nil) {
		t.Error("nil Env does not match int")
	}
	if !MatchType(nil, nil, nil) {
		t.Error("nil does not match nil")
	}
	if MatchType(nil, ast.NewIdent("int"), nil) || MatchType(ast.NewIdent("int"), nil, nil) {
		t.Error("nil matches int")
	}
	if MatchType(Seq(), ast.NewIdent("int"), nil) {
		t.Error("Seq matches a single type")
	}
}

// TestMatchTypeTypedNil checks that nil nodes hidden in interfaces or
// malformed trees report no match instead of panicking. engine.wasm cannot
// recover from a panic.
func TestMatchTypeTypedNil(t *testing.T) {
	intT := func() ast.Expr { return ast.NewIdent("int") }
	nilFieldList := func() *ast.FieldList { return &ast.FieldList{List: []*ast.Field{nil}} }
	tests := []struct {
		name     string
		pat, src ast.Expr
	}{
		{"nil ident source", intT(), (*ast.Ident)(nil)},
		{"nil ident pattern", (*ast.Ident)(nil), intT()},
		{"nil star source", &ast.StarExpr{X: intT()}, (*ast.StarExpr)(nil)},
		{"nil star pattern", (*ast.StarExpr)(nil), &ast.StarExpr{X: intT()}},
		{"nil paren", intT(), (*ast.ParenExpr)(nil)},
		{"nil unary pattern", (*ast.UnaryExpr)(nil), &ast.UnaryExpr{Op: token.TILDE, X: intT()}},
		{"nil selector name source", &ast.SelectorExpr{X: ast.NewIdent("p"), Sel: ast.NewIdent("T")}, &ast.SelectorExpr{X: ast.NewIdent("p")}},
		{"nil selector name pattern", &ast.SelectorExpr{X: ast.NewIdent("p")}, &ast.SelectorExpr{X: ast.NewIdent("p"), Sel: ast.NewIdent("T")}},
		{"nil selector qualifier", &ast.SelectorExpr{X: ast.NewIdent("p"), Sel: ast.NewIdent("T")}, &ast.SelectorExpr{X: (*ast.Ident)(nil), Sel: ast.NewIdent("T")}},
		{"nil array length", &ast.ArrayType{Len: ast.NewIdent("N"), Elt: intT()}, &ast.ArrayType{Len: (*ast.BasicLit)(nil), Elt: intT()}},
		{"nil struct field", &ast.StructType{Fields: &ast.FieldList{}}, &ast.StructType{Fields: nilFieldList()}},
		{"nil struct field pattern", &ast.StructType{Fields: nilFieldList()}, &ast.StructType{Fields: nilFieldList()}},
		{"nil struct field name", &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{nil}, Type: intT()}}}}, &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{nil}, Type: intT()}}}}},
		{"nil interface method", &ast.InterfaceType{Methods: nilFieldList()}, &ast.InterfaceType{Methods: nilFieldList()}},
		{"nil interface method name", &ast.InterfaceType{Methods: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{nil}, Type: &ast.FuncType{}}}}}, &ast.InterfaceType{Methods: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{nil}, Type: &ast.FuncType{}}}}}},
		{"nil param", &ast.FuncType{Params: &ast.FieldList{List: []*ast.Field{{Type: Seq()}}}}, &ast.FuncType{Params: nilFieldList()}},
		{"nil param pattern", &ast.FuncType{Params: nilFieldList()}, &ast.FuncType{Params: &ast.FieldList{}}},
		{"nil param type", &ast.FuncType{Params: &ast.FieldList{List: []*ast.Field{{}}}}, &ast.FuncType{Params: &ast.FieldList{List: []*ast.Field{{}}}}},
		{"nil index", &ast.IndexExpr{X: ast.NewIdent("List"), Index: Seq()}, (*ast.IndexExpr)(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if MatchType(tt.pat, tt.src, nil) {
				t.Error("matched")
			}
		})
	}
}
