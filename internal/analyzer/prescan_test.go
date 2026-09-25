package analyzer

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// nestedParens returns a file whose brackets nest depth deep.
func nestedParens(depth int) string {
	return "package p\n\nvar x = " + strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth) + "\n"
}

// elseIfChain returns a file with one if statement followed by n else-if
// links.
func elseIfChain(n int) string {
	var b strings.Builder
	b.WriteString("package p\n\nfunc f(x int) {\n\tif x == 0 {\n\t}")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, " else if x == %d {\n\t}", i)
	}
	b.WriteString("\n}\n")
	return b.String()
}

// joinTerms returns n copies of each term, joined by " + ".
func joinTerms(n int, terms ...string) string {
	parts := make([]string, 0, n*len(terms))
	for range n {
		parts = append(parts, terms...)
	}
	return strings.Join(parts, " + ")
}

// emptyCases returns a file with a switch statement of n empty case
// clauses.
func emptyCases(n int) string {
	var b strings.Builder
	b.WriteString("package p\n\nfunc f(x int) {\n\tswitch x {\n")
	for i := range n {
		fmt.Fprintf(&b, "\tcase %d:\n", i)
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

func TestScanFileLimits(t *testing.T) {
	opt := Options{MaxBracketDepth: 5, MaxElseIfChain: 3, MaxASTDepth: 10}.withDefaults()
	const nest11 = "nest 11 levels deep, more than the limit of 10"
	body := func(stmts string) string { return "package p\n\nfunc f() {\n" + stmts + "\n}\n" }
	tests := []struct {
		name string
		src  string
		want result.SkipReason
		msg  string
		line int
	}{
		{"brackets at limit", nestedParens(5), "", "", 0},
		{"brackets past limit", nestedParens(6), result.SkipTooDeep, "brackets are nested 6 deep", 3},
		{"mixed brackets past limit", "package p\n\nvar x = []int{f([]int{g(h(i(1)))})}\n", result.SkipTooDeep, "nested 6 deep", 3},
		{"else-if at limit", elseIfChain(3), "", "", 0},
		{"else-if past limit", elseIfChain(4), result.SkipTooDeep, "else-if chains are 4 long", 8},
		{"separate else-if chains", body("\tif x == 0 {} else if x == 1 {} else if x == 2 {}\n" +
			"\tif x == 0 {} else if x == 1 {} else if x == 2 {}"), "", "", 0},
		{"else-if inside else-if", body("\tif x == 0 {} else if x == 1 {} else if x == 2 {\n" +
			"\t\tif x == 0 {} else if x == 1 {} else if x == 2 {}\n\t}"), result.SkipTooDeep, "else-if chains are 4 long", 5},
		{"else-if chain closed", body("\tif x == 0 {} else if x == 1 {} else if x == 2 {\n\t}\n" +
			"\tfor {\n\t\tif x == 0 {} else if x == 1 {}\n\t}"), "", "", 0},
		{"unary chain at limit", "package p\n\nvar x = " + strings.Repeat("!", 10) + "true\n", "", "", 0},
		{"unary chain past limit", "package p\n\nvar x = " + strings.Repeat("!", 11) + "true\n", result.SkipTooDeep, nest11, 3},
		{"receive chain past limit", "package p\n\nvar x = " + strings.Repeat("<-", 11) + "ch\n", result.SkipTooDeep, nest11, 3},
		{"slice types past limit", "package p\n\nvar x = " + strings.Repeat("[]", 11) + "int(nil)\n", result.SkipTooDeep, nest11, 3},
		// The first "[" follows the name, so it reads as an index.
		{"array types in a declaration", "package p\n\nvar x " + strings.Repeat("[1]", 12) + "int\n", result.SkipTooDeep, nest11, 3},
		{"pointer types after slices", "package p\n\ntype T = " + strings.Repeat("[]*", 6) + "int\n", result.SkipTooDeep, nest11, 3},
		{"pointer results", "package p\n\ntype T = " + strings.Repeat("func() *", 6) + "int\n", result.SkipTooDeep, nest11, 3},
		{"map types past limit", "package p\n\ntype T = " + strings.Repeat("map[int]", 11) + "int\n", result.SkipTooDeep, nest11, 3},
		{"chan types past limit", "package p\n\ntype T = " + strings.Repeat("chan ", 11) + "int\n", result.SkipTooDeep, nest11, 3},
		{"send-only chan types at limit", "package p\n\ntype T = " + strings.Repeat("chan<- ", 10) + "int\n", "", "", 0},
		// func counts one level, so ten labels exceed the limit.
		{"labels past limit", body(strings.Repeat("L: ", 10) + "return"), result.SkipTooDeep, nest11, 4},
		{"labels survive binary operators", body(strings.Repeat("L: ", 5) + "_ = a + " + strings.Repeat("!", 5) + "b"), result.SkipTooDeep, nest11, 4},
		{"flat chains", "package p\n\nvar x = " + joinTerms(4, "s.a", "f(x)", "a[i]", "-a", "*p", "<-ch", "x.(T)", "T{}", "[]int{1}[0]", "m[i](x)") + "\n", "", "", 0},
		{"plus chain inside a function", body("\t_ = 1" + strings.Repeat(" + 1", 30)), "", "", 0},
		{"empty case clauses", emptyCases(30), "", "", 0},
		{"commas restart chains", "package p\n\nvar x = []int{" + strings.Repeat("-1 + -2, ", 30) + "}\n", "", "", 0},
		{"scanner error", "package p\n\nvar x = 'ab'\n", result.SkipParseError, "rune literal", 3},
		{"unterminated raw string", "package p\n\nvar x = `ab\n", result.SkipParseError, "raw string literal not terminated", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, sk := scanFile([]byte(tt.src), opt)
			if tt.want == "" {
				if sk != nil {
					t.Fatalf("got skip %+v, want none", sk)
				}
				if info == nil {
					t.Fatal("got nil scanInfo")
				}
				return
			}
			if sk == nil {
				t.Fatalf("got no skip, want %s", tt.want)
			}
			if sk.reason != tt.want || !strings.Contains(sk.msg, tt.msg) || sk.line != tt.line {
				t.Errorf("got %s at line %d: %q, want %s at line %d containing %q", sk.reason, sk.line, sk.msg, tt.want, tt.line, tt.msg)
			}
		})
	}
}

func TestScanFileDefaults(t *testing.T) {
	opt := DefaultOptions()
	for _, tt := range []struct {
		name string
		src  string
		want result.SkipReason
	}{
		{"brackets 200", nestedParens(200), ""},
		{"brackets 201", nestedParens(201), result.SkipTooDeep},
		{"else-if 1000", elseIfChain(1000), ""},
		{"else-if 1001", elseIfChain(1001), result.SkipTooDeep},
		{"unary 1500", "package p\n\nvar x = " + strings.Repeat("^", 1500) + "1\n", ""},
		{"unary 1501", "package p\n\nvar x = " + strings.Repeat("^", 1501) + "1\n", result.SkipTooDeep},
		{"flat chain of 1600 selectors", "package p\n\nvar x = " + joinTerms(1600, "s.a") + "\n", ""},
		{"1600 empty case clauses", emptyCases(1600), ""},
	} {
		_, sk := scanFile([]byte(tt.src), opt)
		var got result.SkipReason
		if sk != nil {
			got = sk.reason
		}
		if got != tt.want {
			t.Errorf("%s: got skip %+v, want %q", tt.name, sk, tt.want)
		}
	}
}

func TestScanFileOperatorLimit(t *testing.T) {
	// The "=" counts too.
	src := func(n int) []byte { return []byte("package p\n\nvar x = 1" + strings.Repeat(" + 1", n) + "\n") }
	if _, sk := scanFile(src(maxOps-1), DefaultOptions()); sk != nil {
		t.Errorf("%d operators: got skip %+v, want none", maxOps, sk)
	}
	_, sk := scanFile(src(maxOps), DefaultOptions())
	if sk == nil || sk.reason != result.SkipTooDeep || !strings.Contains(sk.msg, "chains 50001 operators, more than the limit of 50000") {
		t.Errorf("%d operators: got skip %+v, want too-deep", maxOps+1, sk)
	}
}

// TestScanFileNestingCounter checks that the files the prescan accepts
// stay below the nesting limit of go/parser. The file nests calls as deep
// as the default bracket limit allows, puts the longest accepted operator
// chain before each call, and ends in a long unary chain.
func TestScanFileNestingCounter(t *testing.T) {
	build := func(k int) []byte {
		var b strings.Builder
		b.WriteString("package p\n\nfunc f() {\n\tx = ")
		for range DefaultMaxBracketDepth - 1 {
			b.WriteString(strings.Repeat("a + ", k) + "f(")
		}
		b.WriteString(strings.Repeat("!", 1290) + "a" + strings.Repeat(")", DefaultMaxBracketDepth-1) + "\n}\n")
		return []byte(b.String())
	}
	lo, hi := 0, maxOps/(DefaultMaxBracketDepth-1)+1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if _, sk := scanFile(build(mid), DefaultOptions()); sk == nil {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo*(DefaultMaxBracketDepth-1) < maxOps*9/10 {
		t.Fatalf("the longest accepted chain has %d operators per level", lo)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", build(lo), parser.AllErrors|parser.SkipObjectResolution); err != nil {
		t.Errorf("parse: %v", err)
	}
}
