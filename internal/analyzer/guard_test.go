package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// plusChain returns a file whose syntax tree is n+4 deep: File, GenDecl,
// ValueSpec, n nested BinaryExprs, and the innermost operand.
func plusChain(n int) string {
	return "package p\n\nvar x = 1" + strings.Repeat(" + 1", n) + "\n"
}

func TestCheckSize(t *testing.T) {
	opt := Options{MaxFileSize: 10}.withDefaults()
	if sk := checkSize(make([]byte, 10), opt); sk != nil {
		t.Errorf("10 bytes: got skip %+v, want none", sk)
	}
	sk := checkSize(make([]byte, 11), opt)
	if sk == nil || sk.reason != result.SkipTooLarge {
		t.Errorf("11 bytes: got %+v, want too-large", sk)
	}
}

func TestNotTarget(t *testing.T) {
	sk := notTarget("docs/x.md")
	if sk.reason != result.SkipNotTarget || sk.msg != "docs/x.md is not selected by files.include and files.exclude" {
		t.Errorf("got %+v", sk)
	}
}

func TestDeepNode(t *testing.T) {
	const limit = 20
	for _, tt := range []struct {
		n    int
		deep bool
	}{
		{limit - 4, false},
		{limit - 3, true},
		{200, true},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "x.go", plusChain(tt.n), 0)
		if err != nil {
			t.Fatal(err)
		}
		got := deepNode(file, limit)
		if (got != nil) != tt.deep {
			t.Errorf("n=%d: got deep node %v, want deep=%v", tt.n, got, tt.deep)
		}
		if _, ok := got.(*ast.BasicLit); tt.n == limit-3 && !ok {
			t.Errorf("n=%d: got %T, want the innermost *ast.BasicLit", tt.n, got)
		}
	}
}

const occupancySrc = "package p\n" +
	"\n" +
	"func f() {\n" +
	"\ta := 1; b := 2\n" +
	"\t/* c */ g(1, 2), // comment\n" +
	"\tx := `raw\n" +
	"line` + y\n" +
	"\th() // trailing\n" +
	"}\n"

func TestOccupies(t *testing.T) {
	info, sk := scanFile([]byte(occupancySrc), DefaultOptions())
	if sk != nil {
		t.Fatal(sk)
	}
	if got := info.lines(); got != 9 {
		t.Errorf("lines() = %d, want 9", got)
	}
	tests := []struct {
		span string
		ok   bool
		line int
	}{
		{"b := 2", false, 4},
		{"a := 1", false, 4},
		{"a := 1; b := 2", true, 0},
		{"g(1, 2)", true, 0},
		{"/* c */ g(1, 2), // comment", true, 0},
		{"y", false, 7},
		{"x := `raw\nline`", false, 7},
		{"x := `raw\nline` + y", true, 0},
		{"h()", true, 0},
		{"func f() {\n\ta := 1", false, 4},
		{"func f() {\n\ta := 1; b := 2\n\t/* c */ g(1, 2), // comment\n\tx := `raw\nline` + y\n\th() // trailing\n}", true, 0},
	}
	for _, tt := range tests {
		start := strings.Index(occupancySrc, tt.span)
		if start < 0 {
			t.Fatalf("span %q not found", tt.span)
		}
		ok, line := info.occupies(start, start+len(tt.span))
		if ok != tt.ok || line != tt.line {
			t.Errorf("occupies(%q) = %v, %d, want %v, %d", tt.span, ok, line, tt.ok, tt.line)
		}
	}
}

func TestStartPos(t *testing.T) {
	for _, src := range []string{
		"a + b*c + d",
		"a.b.c(x)[1][2:3].(T).d",
		"f()()()",
		"m[K]{k: v}",
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := startPos(e), e.Pos(); got != want {
			t.Errorf("startPos(%q) = %d, want %d", src, got, want)
		}
	}
	// A chain much deeper than MaxASTDepth must not recurse.
	e, err := parser.ParseExpr(strings.Repeat("a + ", 20000) + "a")
	if err != nil {
		t.Fatal(err)
	}
	if got := startPos(e); got != 1 {
		t.Errorf("startPos(long chain) = %d, want 1", got)
	}
}
