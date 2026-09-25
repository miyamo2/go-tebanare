package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// parseTest parses src with comments.
func parseTest(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	return fset, file
}

func TestFuncLabel(t *testing.T) {
	src := `package p

func (r *Repository[T]) FindByID() {}
func (u User) Name() {}
func (c *Cache[K, V]) Get() {}
func Plain() {}
func Map[T, U any]() {}
func Pair[K comparable, V any]() {}
func Must[T interface{ ~int | ~string }]() {}
func (_ ( *T )) Paren() {}
`
	want := []string{
		"func (*Repository[T]) FindByID",
		"func (User) Name",
		"func (*Cache[K, V]) Get",
		"func Plain",
		"func Map[T, U any]",
		"func Pair[K comparable, V any]",
		"func Must[T interface{ ~int | ~string }]",
		"func ((*T)) Paren",
	}
	_, file := parseTest(t, src)
	for i, d := range file.Decls {
		if got := funcLabel(d.(*ast.FuncDecl)); got != want[i] {
			t.Errorf("decl %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestNodeLabel(t *testing.T) {
	long := strings.Repeat("a", 61)
	for _, tt := range []struct {
		in, want string
	}{
		{`log.Debug("x")`, `log.Debug("x")`},
		{strings.Repeat("a", 60), strings.Repeat("a", 60)},
		{long, strings.Repeat("a", 60) + "..."},
		{strings.Repeat("\u00e9", 61), strings.Repeat("\u00e9", 60) + "..."},
		{"", ""},
	} {
		if got := nodeLabel(tt.in); got != tt.want {
			t.Errorf("nodeLabel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
