package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestPairingKeys(t *testing.T) {
	src := `package p

func init() {}
func (T) init() {}
func init() {}
func _() {}
func (*T) _() {}
func (T[K]) _() {}
func F() {}
func (t *T[K, V]) M() {}
func (t ([]int)) Odd() {}
func () NoRecv() {}
var init = 1
`
	file, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	keys := pairingKeys(file)
	var got []string
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			got = append(got, keys[fd])
		}
	}
	check(t, "keys", got, []string{"init#0", "T.init", "init#1", "_#0", "T._#0", "T._#1", "F", "T.M", "([]int).Odd", "().NoRecv"})
}
