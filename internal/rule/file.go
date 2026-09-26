package rule

import (
	"go/ast"
	"go/token"
	"sort"
)

// File is the per-file context shared by all matchers during one analysis.
type File struct {
	// Path is the slash-separated path relative to the repository root.
	Path string
	Fset *token.FileSet
	AST  *ast.File
	Src  []byte

	methods map[string]map[string]bool
}

// Line returns the 1-based line of pos in the file itself, ignoring //line
// directives.
func (f *File) Line(pos token.Pos) int {
	return f.Fset.PositionFor(pos, false).Line
}

// Offset returns the byte offset of pos in Src.
func (f *File) Offset(pos token.Pos) int {
	return f.Fset.PositionFor(pos, false).Offset
}

// MethodsOf returns the names of the methods declared in this file whose
// receiver base type is named recv. The result must not be modified.
func (f *File) MethodsOf(recv string) map[string]bool {
	if f.methods == nil {
		f.methods = map[string]map[string]bool{}
		for _, d := range f.AST.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil {
				continue
			}
			base, _, ok := RecvBase(fd)
			if !ok {
				continue
			}
			if f.methods[base] == nil {
				f.methods[base] = map[string]bool{}
			}
			f.methods[base][fd.Name.Name] = true
		}
	}
	return f.methods[recv]
}

// HasCommentInLines reports whether a comment touches any line in
// [from, to]. Comments in the group skip are ignored; pass a function's Doc
// to exclude its doc comment.
func (f *File) HasCommentInLines(from, to int, skip *ast.CommentGroup) bool {
	groups := f.AST.Comments
	// Groups are sorted by position; find the first one that ends at or
	// after line from.
	i := sort.Search(len(groups), func(i int) bool {
		return f.Line(groups[i].End()) >= from
	})
	for ; i < len(groups); i++ {
		g := groups[i]
		if f.Line(g.Pos()) > to {
			break
		}
		if g == skip {
			continue
		}
		for _, c := range g.List {
			if f.Line(c.Pos()) <= to && f.Line(c.End()) >= from {
				return true
			}
		}
	}
	return false
}

// RecvBase returns the base type name of fd's receiver and whether the
// receiver is a pointer. Parentheses and type arguments are removed, so
// "*Cache[K, V]" yields ("Cache", true). ok is false for functions and for
// receivers that are not a (pointer to a) named type.
func RecvBase(fd *ast.FuncDecl) (name string, pointer bool, ok bool) {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return "", false, false
	}
	t := StripParens(fd.Recv.List[0].Type)
	if st, isStar := t.(*ast.StarExpr); isStar {
		pointer = true
		t = StripParens(st.X)
	}
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	id, isIdent := StripParens(t).(*ast.Ident)
	if !isIdent {
		return "", false, false
	}
	return id.Name, pointer, true
}

// StripParens removes any number of enclosing parentheses.
func StripParens(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}
