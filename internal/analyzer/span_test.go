package analyzer

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestIsDirective(t *testing.T) {
	for _, tt := range []struct {
		text string
		want bool
	}{
		{"go:generate stringer -type=Kind", true},
		{"go:linkname x y", true},
		{"nolint:errcheck", true},
		{"lint:ignore U1000 reason", true},
		{"a:b", true},
		{"line gen.y:10", true},
		{"export F", true},
		{"extern f", true},
		{"line", false},
		{"Deprecated: use G.", false},
		{"TODO: fix", false},
		{"http://example.com", false},
		{" go:generate x", false},
		{"go:", false},
		{":x", false},
		{"F returns the answer.", false},
	} {
		if got := isDirective(tt.text); got != tt.want {
			t.Errorf("isDirective(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestFuncRange(t *testing.T) {
	for _, tt := range []struct {
		name       string
		doc        string
		includeDoc bool
		wantLine   int
	}{
		{"doc", "// F does it.\n// More.\n", true, 3},
		{"no doc", "", true, 3},
		{"include_doc false", "// F does it.\n", false, 4},
		{"go:generate", "// F does it.\n//go:generate stringer\n", true, 5},
		{"nolint", "//nolint:errcheck\n", true, 4},
		{"line directive", "//line gen.y:100\n", true, 4},
		{"export", "// F is exported to C.\n//export F\n", true, 5},
		{"block comment is not a directive", "/*go:generate x*/\n", true, 3},
	} {
		src := "package p\n\n" + tt.doc + "func F() {}\n"
		fset, file := parseTest(t, src)
		fd := file.Decls[0].(*ast.FuncDecl)
		from, to := funcRange(fd, tt.includeDoc)
		if got := fset.PositionFor(from, false).Line; got != tt.wantLine {
			t.Errorf("%s: starts on line %d, want %d", tt.name, got, tt.wantLine)
		}
		if to != fd.End() {
			t.Errorf("%s: ends at %v, want fd.End()", tt.name, to)
		}
	}
}

func TestOffsets(t *testing.T) {
	fset, file := parseTest(t, "package p\n\nvar x = 1\n")
	vs := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
	base := token.Pos(fset.File(file.Pos()).Base())
	for _, tt := range []struct {
		name       string
		from, to   token.Pos
		start, end int
		ok         bool
	}{
		{"node", vs.Pos(), vs.End(), 15, 20, true},
		{"whole file", base, base + 21, 0, 21, true},
		{"empty", vs.Pos(), vs.Pos(), 0, 0, false},
		{"reversed", vs.End(), vs.Pos(), 0, 0, false},
		{"no position", token.NoPos, vs.End(), 0, 0, false},
		{"past the end", vs.Pos(), base + 22, 0, 0, false},
	} {
		start, end, ok := offsets(fset, tt.from, tt.to)
		if start != tt.start || end != tt.end || ok != tt.ok {
			t.Errorf("%s: got %d, %d, %v, want %d, %d, %v", tt.name, start, end, ok, tt.start, tt.end, tt.ok)
		}
	}
}
