package rule

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestKindOf(t *testing.T) {
	src := `package p

var v = []int{1}

func f() {
	if true {
		x := g(1) + 2
		_ = x
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if n != nil {
			k := KindOf(n)
			if k == "" {
				t.Errorf("KindOf(%T) = \"\"", n)
			}
			seen[k] = true
		}
		return true
	})
	for _, k := range []string{"File", "GenDecl", "ValueSpec", "CompositeLit", "ArrayType", "FuncDecl", "IfStmt", "AssignStmt", "BinaryExpr", "CallExpr", "BasicLit", "Ident"} {
		if !seen[k] {
			t.Errorf("kind %s not seen", k)
		}
	}
	if KindOf(nil) != "" {
		t.Error("KindOf(nil) != \"\"")
	}
}
