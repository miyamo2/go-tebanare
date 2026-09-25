package analyzer

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/canon"
	"github.com/miyamo2/go-tebanare/internal/rule"
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

func TestStatementFor(t *testing.T) {
	tests := []struct {
		code   string
		target string
		want   string // statement kind, "" for none
	}{
		// Simple statements whose other parts are identifiers or literals.
		{`log.Debug("x")`, `log.Debug("x")`, "ExprStmt"},
		{`_ = log.Debug("x")`, `log.Debug("x")`, "AssignStmt"},
		{`x, _ := log.Debug("x")`, `log.Debug("x")`, "AssignStmt"},
		{`x = (log.Debug("x"))`, `log.Debug("x")`, "AssignStmt"},
		{`var v = log.Debug("x")`, `log.Debug("x")`, "DeclStmt"},
		{`var v int = log.Debug("x")`, `log.Debug("x")`, "DeclStmt"},
		{`var a, b = 1, log.Debug("x")`, `log.Debug("x")`, "DeclStmt"},
		{`return nil, 1, "a", log.Debug("x")`, `log.Debug("x")`, "ReturnStmt"},
		{`defer log.Debug("x")`, `log.Debug("x")`, "DeferStmt"},
		{`go log.Debug("x")`, `log.Debug("x")`, "GoStmt"},
		{`ch <- log.Debug("x")`, `log.Debug("x")`, "SendStmt"},
		{`p.n++`, `p.n`, "IncDecStmt"},
		{`f := func() { log.Debug("x") }`, `log.Debug("x")`, "ExprStmt"},

		// (1) compound statements are never hidden.
		{`if logger.DebugEnabled() { log.Debug("x") }`, `logger.DebugEnabled()`, ""},
		{`for _, x := range log.Debug("x") { _ = x }`, `log.Debug("x")`, ""},
		{`switch log.Debug("x") { }`, `log.Debug("x")`, ""},
		// (2) the expression must be a direct component.
		{`x := compute(log.Debug("x"))`, `log.Debug("x")`, ""},
		{`x := -log.Level()`, `log.Level()`, ""},
		{`m[log.Key()]++`, `log.Key()`, ""},
		// (3) the other components must be identifiers or basic literals.
		{`x.y = log.Debug("x")`, `log.Debug("x")`, ""},
		{`return f(), log.Debug("x")`, `log.Debug("x")`, ""},
		{`a, b := log.Debug("x"), g()`, `log.Debug("x")`, ""},
		{`var v []int = log.Debug("x")`, `log.Debug("x")`, ""},
		{`var v, w = log.Debug("x"), g()`, `log.Debug("x")`, ""},
		{`s.ch <- log.Debug("x")`, `log.Debug("x")`, ""},
	}
	for _, tt := range tests {
		src := "package p\n\nfunc f() {\n\t" + tt.code + "\n}\n"
		_, file := parseTest(t, src)
		found := false
		walk(file, func(n ast.Node, _ scope, _ bool, ancestors []ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok || found || canon.Normalize(e) != tt.target {
				return true
			}
			found = true
			got := ""
			if s := statementFor(e, ancestors); s != nil {
				got = rule.KindOf(s)
			}
			if got != tt.want {
				t.Errorf("%s: statementFor(%s) = %q, want %q", tt.code, tt.target, got, tt.want)
			}
			return true
		})
		if !found {
			t.Errorf("%s: %s not found", tt.code, tt.target)
		}
	}
}

func TestStatementForPackageLevel(t *testing.T) {
	_, file := parseTest(t, "package p\n\nvar v = log.Debug(\"x\")\n")
	walk(file, func(n ast.Node, _ scope, _ bool, ancestors []ast.Node) bool {
		if e, ok := n.(*ast.CallExpr); ok {
			if s := statementFor(e, ancestors); s != nil {
				t.Errorf("got %T, want nil outside functions", s)
			}
		}
		return true
	})
}

func TestLeadingComment(t *testing.T) {
	src := "package p\n" + // 1
		"\n" + // 2
		"func f() {\n" + // 3
		"\t// About a.\n" + // 4
		"\t// More.\n" + // 5
		"\ta()\n" + // 6
		"\tx := 1 // about x\n" + // 7
		"\tb()\n" + // 8
		"\t/* one */ /* two */\n" + // 9
		"\tc()\n" + // 10
		"\t/* c */ y := 2\n" + // 11
		"\td()\n" + // 12
		"\t// Gap.\n" + // 13
		"\n" + // 14
		"\te()\n" + // 15
		"\t/* multi\n" + // 16
		"\tline */ z := 3\n" + // 17
		"\tg()\n" + // 18
		"}\n"
	info, sk := scanFile([]byte(src), DefaultOptions())
	if sk != nil {
		t.Fatal(sk)
	}
	fset, file := parseTest(t, src)
	f := &rule.File{Path: "x.go", Fset: fset, AST: file, Src: []byte(src)}
	for _, tt := range []struct {
		line int
		want int // first line of the group, 0 for none
	}{
		{6, 4},
		{8, 0},  // line comment after code
		{10, 9}, // two comments form one group
		{12, 0}, // code after the comment
		{15, 0}, // blank line in between
		{18, 0}, // code after a multi-line comment
		{4, 0},  // nothing above
		{3, 0},
	} {
		got := 0
		if g := leadingComment(f, info, tt.line); g != nil {
			got = f.Line(g.Pos())
		}
		if got != tt.want {
			t.Errorf("line %d: got group at line %d, want %d", tt.line, got, tt.want)
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
