package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/canon"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

const walkSrc = `package p

import "fmt"

type T struct{ F map[string]int }

var table = []T{{F: nil}}

var fn = func(x []int) int { return len(x) }

var typed map[string]int = nil

const c = 1 << 2

func (r *T) M(a []int) (b int) {
	var m map[string]int
	type local struct{ G []byte }
	f := func(y []int) {}
	return len(m)
}
`

// candidates returns "stmt|expr Kind: text" for every node that stmt or
// expr rules consider, leaving out identifiers.
func candidates(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	walk(file, func(n ast.Node, sc scope, bodyBlock bool) bool {
		if _, ok := n.(*ast.Ident); ok {
			return true
		}
		switch {
		case isStmtCandidate(n, sc, bodyBlock):
			got = append(got, fmt.Sprintf("stmt %s: %s", rule.KindOf(n), canon.Normalize(n)))
		case isExprCandidate(n, sc):
			got = append(got, fmt.Sprintf("expr %s: %s", rule.KindOf(n), canon.Normalize(n)))
		}
		return true
	})
	return got
}

func TestWalkScope(t *testing.T) {
	got := candidates(t, walkSrc)
	want := []string{
		"expr CompositeLit: []T{{F: nil}}",
		"expr ArrayType: []T",
		"expr CompositeLit: {F: nil}",
		"expr KeyValueExpr: F: nil",
		"expr FuncLit: func(x []int) int { return len(x) }",
		"stmt ReturnStmt: return len(x)",
		"expr CallExpr: len(x)",
		"expr BinaryExpr: 1 << 2",
		"expr BasicLit: 1",
		"expr BasicLit: 2",
		"stmt DeclStmt: var m map[string]int",
		"expr MapType: map[string]int",
		"stmt DeclStmt: type local struct{ G []byte }",
		"stmt AssignStmt: f := func(y []int) { }",
		"expr FuncLit: func(y []int) { }",
		"stmt ReturnStmt: return len(m)",
		"expr CallExpr: len(m)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("candidates:\n%s\nwant:\n%s", join(got), join(want))
	}
}

func join(lines []string) string {
	s := ""
	for _, l := range lines {
		s += "\t" + l + "\n"
	}
	return s
}
