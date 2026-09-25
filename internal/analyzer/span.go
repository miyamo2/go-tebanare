package analyzer

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// isDirective reports whether c, the text of a // comment without the
// slashes, is a directive by go/ast's rule: "line ", "extern ", and
// "export " prefixes, and the "[a-z0-9]+:[a-z0-9]" form such as
// "go:generate" or "nolint:errcheck".
func isDirective(c string) bool {
	if strings.HasPrefix(c, "line ") || strings.HasPrefix(c, "extern ") || strings.HasPrefix(c, "export ") {
		return true
	}
	colon := strings.IndexByte(c, ':')
	if colon <= 0 || colon+1 >= len(c) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		if b := c[i]; (b < 'a' || b > 'z') && (b < '0' || b > '9') {
			return false
		}
	}
	return true
}

// hasDirective reports whether a // comment of g is a directive.
func hasDirective(g *ast.CommentGroup) bool {
	for _, c := range g.List {
		if strings.HasPrefix(c.Text, "//") && isDirective(c.Text[2:]) {
			return true
		}
	}
	return false
}

// funcRange returns the source range a func rule hides for fd: the doc
// comment and the declaration when includeDoc is set and the doc comment
// holds no directive, the declaration alone otherwise.
func funcRange(fd *ast.FuncDecl, includeDoc bool) (from, to token.Pos) {
	from = fd.Pos()
	if includeDoc && fd.Doc != nil && !hasDirective(fd.Doc) {
		from = fd.Doc.Pos()
	}
	return from, fd.End()
}

// statementFor returns the statement that an expr rule with hide:
// statement hides for e (plan 4.7), or nil when there is none. The
// statement is the innermost enclosing statement, and it must be a simple
// statement (ExprStmt, AssignStmt, DeclStmt, IncDecStmt, SendStmt, GoStmt,
// DeferStmt, or ReturnStmt) that has e as a direct component, ignoring
// parentheses, and only identifiers and basic literals as its other
// components.
func statementFor(e ast.Expr, ancestors []ast.Node) ast.Stmt {
	s := innermostStmt(ancestors)
	if s == nil {
		return nil
	}
	parts, ok := components(s)
	if !ok {
		return nil
	}
	target := rule.StripParens(e)
	found := false
	for _, p := range parts {
		p = rule.StripParens(p)
		if p == target && !found {
			found = true
			continue
		}
		switch p.(type) {
		case *ast.Ident, *ast.BasicLit:
		default:
			return nil
		}
	}
	if !found {
		return nil
	}
	return s
}

// components returns the expressions of a simple statement, or false for
// other statements. A DeclStmt is simple when it declares variables or
// constants whose types are omitted or plain identifiers; its components
// are the values.
func components(s ast.Stmt) ([]ast.Expr, bool) {
	switch s := s.(type) {
	case *ast.ExprStmt:
		return []ast.Expr{s.X}, true
	case *ast.AssignStmt:
		parts := make([]ast.Expr, 0, len(s.Lhs)+len(s.Rhs))
		return append(append(parts, s.Lhs...), s.Rhs...), true
	case *ast.IncDecStmt:
		return []ast.Expr{s.X}, true
	case *ast.SendStmt:
		return []ast.Expr{s.Chan, s.Value}, true
	case *ast.GoStmt:
		return []ast.Expr{s.Call}, true
	case *ast.DeferStmt:
		return []ast.Expr{s.Call}, true
	case *ast.ReturnStmt:
		return s.Results, true
	case *ast.DeclStmt:
		gd, ok := s.Decl.(*ast.GenDecl)
		if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
			return nil, false
		}
		var parts []ast.Expr
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				return nil, false
			}
			if _, ident := vs.Type.(*ast.Ident); vs.Type != nil && !ident {
				return nil, false
			}
			parts = append(parts, vs.Values...)
		}
		return parts, true
	}
	return nil, false
}

// leadingComment returns the comment group that ends on the line above
// line, provided the group has its lines to itself: no code before it on
// its first line and none after it on its last line. It returns nil when
// there is no such group.
func leadingComment(f *rule.File, info *scanInfo, line int) *ast.CommentGroup {
	groups := f.AST.Comments
	i := sort.Search(len(groups), func(i int) bool { return f.Line(groups[i].End()) >= line })
	if i == 0 {
		return nil
	}
	g := groups[i-1]
	if f.Line(g.End()) != line-1 {
		return nil
	}
	if c := info.firstCode[f.Line(g.Pos())-1]; c >= 0 && c < f.Offset(g.Pos()) {
		return nil
	}
	if c := info.lastCode[line-2]; c > f.Offset(g.End()) {
		return nil
	}
	return g
}

// offsets converts the source range [from, to) to byte offsets. It
// reports false when the range is empty, reversed, or not inside the file,
// which can only come from a matcher's explicit span.
func offsets(fset *token.FileSet, from, to token.Pos) (start, end int, ok bool) {
	tf := fset.File(from)
	if tf == nil || !to.IsValid() || to <= from || int(to) > tf.Base()+tf.Size() {
		return 0, 0, false
	}
	return tf.Offset(from), tf.Offset(to), true
}
