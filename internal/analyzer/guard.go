package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/result"
)

func checkSize(src []byte, opt Options) *skip {
	if len(src) <= opt.MaxFileSize {
		return nil
	}
	return &skip{
		reason: result.SkipTooLarge,
		msg:    fmt.Sprintf("the file has %d bytes, more than the limit of %d", len(src), opt.MaxFileSize),
	}
}

// notTarget is the skip of a path that the set's file filters exclude.
func notTarget(path string) *skip {
	return &skip{reason: result.SkipNotTarget, msg: fmt.Sprintf("%s is not selected by files.include and files.exclude", path)}
}

// deepNode returns the first node, in walk order, that lies deeper than
// limit in the syntax tree of file, where file itself has depth 1. It
// returns nil when there is none. The walk never descends below limit+1.
func deepNode(file *ast.File, limit int) ast.Node {
	var deep ast.Node
	depth := 0
	ast.Inspect(file, func(n ast.Node) bool {
		switch {
		case deep != nil:
			return false
		case n == nil:
			depth--
			return false
		}
		depth++
		if depth > limit {
			deep = n
			return false
		}
		return true
	})
	return deep
}

// startPos returns n.Pos() without recursion. The Pos methods of go/ast
// expressions such as BinaryExpr and SelectorExpr call Pos on their left
// operand, so on a left-leaning chain like "a + a + ... + a" one Pos call
// uses one stack frame per level. In the wasm engine a chain of 10,000
// terms exhausts the host's call stack.
func startPos(n ast.Node) token.Pos {
	for {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			n = x.X
		case *ast.CallExpr:
			n = x.Fun
		case *ast.SelectorExpr:
			n = x.X
		case *ast.IndexExpr:
			n = x.X
		case *ast.IndexListExpr:
			n = x.X
		case *ast.SliceExpr:
			n = x.X
		case *ast.TypeAssertExpr:
			n = x.X
		case *ast.KeyValueExpr:
			n = x.Key
		default:
			return n.Pos()
		}
	}
}
