package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
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
