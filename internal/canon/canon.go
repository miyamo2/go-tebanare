// Package canon builds the normalized text of Go syntax nodes. The
// analyzer uses it in the keys that pair declarations and in the labels of
// matches. Stmt and expr rules run their regular expressions on this text.
//
// The normalized text of a node is its go/printer output without position
// information and without comments, with every run of ASCII whitespace
// (space, \t, \n, \r, \f, \v) collapsed into one space and the ends
// trimmed. Whitespace inside string literals is collapsed too, so a raw
// string that spans lines becomes one line. The original line breaks do
// not affect the result:
//
//	log.Printf(
//		"find %s",
//		id,
//	)
//
// becomes
//
//	log.Printf("find %s", id)
//
// Statements in a block are separated by one space, without ";".
package canon

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// DefaultMaxNodeSize is the default limit, in source bytes, on the size of
// a node that Cache.Text normalizes.
const DefaultMaxNodeSize = 8 << 10

// Cache builds and memoizes the normalized text of the nodes of one parsed
// file. Create it with NewCache.
type Cache struct {
	fset *token.FileSet
	max  int
	memo map[ast.Node]string
}

// NewCache returns a Cache for nodes whose positions belong to fset.
// A maxNodeSize of zero or less means DefaultMaxNodeSize. With a nil fset,
// Text measures a node by the difference of its end and start positions.
func NewCache(fset *token.FileSet, maxNodeSize int) *Cache {
	if maxNodeSize <= 0 {
		maxNodeSize = DefaultMaxNodeSize
	}
	return &Cache{fset: fset, max: maxNodeSize, memo: map[ast.Node]string{}}
}

// Text returns the normalized text of n. It reports false when n spans
// more than the size limit in the source, when the position of n is not in
// the Cache's FileSet, and when go/printer cannot print n (for example an
// *ast.FieldList).
//
// The limit keeps nested statements cheap: each ancestor of a statement
// prints the statement again, so deep nesting would take quadratic time.
//
// Like Normalize, Text may change the comment fields under n while it
// runs, so no other code may read the syntax tree at the same time.
func (c *Cache) Text(n ast.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	if s, ok := c.memo[n]; ok {
		return s, true
	}
	size, ok := c.size(n)
	if !ok || size > c.max {
		return "", false
	}
	s, ok := format(n)
	if !ok {
		return "", false
	}
	c.memo[n] = s
	return s, true
}

// size returns the number of source bytes from the start to the end of n.
func (c *Cache) size(n ast.Node) (int, bool) {
	if c.fset == nil {
		return int(n.End() - n.Pos()), n.Pos().IsValid()
	}
	f := c.fset.File(n.Pos())
	if f == nil {
		return 0, false
	}
	return f.Offset(n.End()) - f.Offset(n.Pos()), true
}

// Normalize returns the normalized text of n without a size limit and
// without memoization. It returns "" for nil and for nodes that go/printer
// cannot print.
//
// When the nodes under n hold comments in their Doc and Comment fields (or
// n is an *ast.File with Comments), Normalize sets these fields to nil
// while it prints and restores them before it returns, so no other code
// may read the syntax tree at the same time.
func Normalize(n ast.Node) string {
	if n == nil {
		return ""
	}
	s, _ := format(n)
	return s
}

// emptyFset has no files, so go/printer finds no line information for any
// position and lays the node out as if it had no line breaks.
var emptyFset = token.NewFileSet()

func format(n ast.Node) (string, bool) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, emptyFset, n); err != nil {
		return "", false
	}
	// go/printer prints every comment group that n holds, so output
	// without "//" and "/*" has no comments to drop.
	if b := buf.Bytes(); bytes.Contains(b, []byte("//")) || bytes.Contains(b, []byte("/*")) {
		restore := dropComments(n)
		defer restore()
		buf.Reset()
		if err := printer.Fprint(&buf, emptyFset, n); err != nil {
			return "", false
		}
	}
	return collapse(buf.Bytes()), true
}

// dropComments sets the comment fields of n and the nodes under it to nil
// and returns a function that restores them. go/printer prints the comment
// groups in the Doc and Comment fields of declarations, specs, and fields
// wherever its layout flushes them, even inside an expression, and a
// pending comment changes the layout too: an empty function body becomes
// "{\n}".
func dropComments(n ast.Node) (restore func()) {
	var groups []**ast.CommentGroup
	add := func(fields ...**ast.CommentGroup) {
		for _, f := range fields {
			if *f != nil {
				groups = append(groups, f)
			}
		}
	}
	var file *ast.File
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.File:
			file = n
			add(&n.Doc)
		case *ast.FuncDecl:
			add(&n.Doc)
		case *ast.GenDecl:
			add(&n.Doc)
		case *ast.Field:
			add(&n.Doc, &n.Comment)
		case *ast.ImportSpec:
			add(&n.Doc, &n.Comment)
		case *ast.ValueSpec:
			add(&n.Doc, &n.Comment)
		case *ast.TypeSpec:
			add(&n.Doc, &n.Comment)
		}
		return true
	})
	saved := make([]*ast.CommentGroup, len(groups))
	for i, g := range groups {
		saved[i], *g = *g, nil
	}
	var comments []*ast.CommentGroup
	if file != nil {
		comments, file.Comments = file.Comments, nil
	}
	return func() {
		for i, g := range groups {
			*g = saved[i]
		}
		if file != nil {
			file.Comments = comments
		}
	}
}

// collapse replaces every run of ASCII whitespace in b with one space and
// trims both ends. It reuses b as the output buffer.
func collapse(b []byte) string {
	out := b[:0]
	pending := false
	for _, c := range b {
		if isSpace(c) {
			pending = len(out) > 0
			continue
		}
		if pending {
			out = append(out, ' ')
			pending = false
		}
		out = append(out, c)
	}
	return string(out)
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}
