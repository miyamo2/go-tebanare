package astmatch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// funcDecl parses a source function declaration.
func funcDecl(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package p\n"+src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return f.Decls[0].(*ast.FuncDecl)
}

// typeParams returns the type parameter list of a source function and its
// names, as in "func f[K comparable, V any]()".
func typeParams(t *testing.T, src string) (*ast.FieldList, []*ast.Ident) {
	t.Helper()
	fd := funcDecl(t, src)
	var names []*ast.Ident
	for _, f := range fieldList(fd.Type.TypeParams) {
		names = append(names, f.Names...)
	}
	return fd.Type.TypeParams, names
}

// patParams builds a pattern type parameter list. Each spec is "SEQ", "_",
// a bare name, or a name followed by a constraint in pattern syntax.
func patParams(t *testing.T, specs ...string) *ast.FieldList {
	t.Helper()
	fl := &ast.FieldList{}
	for _, s := range specs {
		name, constraint, _ := strings.Cut(s, " ")
		f := &ast.Field{Names: []*ast.Ident{ast.NewIdent(name)}}
		switch name {
		case "SEQ":
			f.Names[0] = Seq()
		case "_":
			f.Names[0] = Any()
		}
		if constraint != "" {
			f.Type = pat(t, constraint)
		}
		fl.List = append(fl.List, f)
	}
	return fl
}

func TestMatchTypeParams(t *testing.T) {
	tests := []struct {
		name  string
		pat   []string
		src   string
		want  bool
		binds map[string]string
	}{
		{"alpha", []string{"T any"}, "func f[E any]()", true, map[string]string{"T": "E"}},
		{"shared constraint", []string{"T any", "U any"}, "func f[A any, B comparable]()", false, nil},
		{"any constraint", []string{"T", "U"}, "func f[A any, B comparable]()", true, map[string]string{"T": "A", "U": "B"}},
		{"seq empty", []string{"SEQ"}, "func f()", true, nil},
		{"seq", []string{"SEQ"}, "func f[K comparable, V any]()", true, nil},
		{"blank", []string{"_", "_"}, "func f[K comparable, V any]()", true, nil},
		{"blank count", []string{"_", "_"}, "func f[T any]()", false, nil},
		{"blank constraint", []string{"_ comparable"}, "func f[K any]()", false, nil},
		{"forward reference", []string{"S ~[]TP_E", "E any"}, "func f[X ~[]Y, Y any]()", true, map[string]string{"S": "X", "E": "Y"}},
		{"forward reference to a type", []string{"S ~[]TP_E", "E any"}, "func f[X ~[]W, Y any]()", false, nil},
		{"same name twice", []string{"T", "T"}, "func f[A, B any]()", false, nil},
		{"last", []string{"SEQ", "T comparable"}, "func f[A any, B comparable]()", true, map[string]string{"T": "B"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, names := typeParams(t, tt.src)
			env := NewEnv(names)
			if got := MatchTypeParams(patParams(t, tt.pat...), src, env); got != tt.want {
				t.Fatalf("MatchTypeParams = %v, want %v", got, tt.want)
			}
			if !tt.want && env.Snapshot() != 0 {
				t.Error("failed match left bindings")
			}
			for p, s := range tt.binds {
				if got, _ := env.Lookup(p); got != s {
					t.Errorf("Lookup(%s) = %q, want %q", p, got, s)
				}
			}
		})
	}
}

func TestMatchTypeParamsThen(t *testing.T) {
	src, names := typeParams(t, "func f[A, B, C any]()")
	p := patParams(t, "SEQ", "T", "SEQ")

	env := NewEnv(names)
	var tried []string
	ok := MatchTypeParamsThen(p, src, env, func() bool {
		b, _ := env.Lookup("T")
		tried = append(tried, b)
		return b == "C"
	})
	if !ok || len(tried) != 3 {
		t.Fatalf("got %v after trying %v", ok, tried)
	}
	if b, _ := env.Lookup("T"); b != "C" {
		t.Errorf("T is bound to %q, want C", b)
	}

	env = NewEnv(names)
	if MatchTypeParamsThen(p, src, env, func() bool { return false }) {
		t.Error("matched although the continuation always fails")
	}
	if env.Snapshot() != 0 {
		t.Error("failed match left bindings")
	}
}

func TestMatchTypeParamsBudget(t *testing.T) {
	src, names := typeParams(t, "func f[A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T any]()")
	p := patParams(t, "SEQ", "V1", "SEQ", "V2", "SEQ", "V3", "SEQ", "V4", "SEQ", "V5", "SEQ")
	calls := 0
	if MatchTypeParamsThen(p, src, NewEnv(names), func() bool { calls++; return false }) {
		t.Fatal("matched although the continuation always fails")
	}
	if calls != maxAlignments {
		t.Errorf("continuation ran %d times, want %d", calls, maxAlignments)
	}
}
