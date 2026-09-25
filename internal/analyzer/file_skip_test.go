package analyzer

import (
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

func TestAnalyzeFileSkips(t *testing.T) {
	small := Options{MaxFileSize: 30, MaxBracketDepth: 4, MaxElseIfChain: 2, MaxASTDepth: 20}
	for _, tt := range []struct {
		name string
		path string
		src  string
		opt  Options
		want result.SkipReason
	}{
		{"fits", "x.go", "package p\n", small, ""},
		{"too large", "x.go", "package p\n\nvar x = 1 // a long comment\n", small, result.SkipTooLarge},
		{"brackets", "x.go", nestedParens(5), small, result.SkipTooDeep},
		{"brackets at limit", "x.go", nestedParens(4), small, ""},
		{"else-if chain", "x.go", elseIfChain(3), small, result.SkipTooDeep},
		{"else-if chain at limit", "x.go", elseIfChain(2), small, ""},
		{"AST depth", "x.go", plusChain(17), small, result.SkipTooDeep},
		{"AST depth at limit", "x.go", plusChain(16), small, ""},
		{"parse error", "x.go", "package p\n\nfunc {\n", small, result.SkipParseError},
		{"not a Go file", "x.txt", "package p\n", small, result.SkipNotTarget},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opt := tt.opt
			if tt.want != result.SkipTooLarge {
				opt.MaxFileSize = 0 // default
			}
			res := AnalyzeFile(nil, tt.path, []byte(tt.src), opt)
			if res.Skipped != tt.want {
				t.Fatalf("Skipped = %q, want %q (%v)", res.Skipped, tt.want, res.Diagnostics)
			}
			if tt.want == "" {
				return
			}
			if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != result.CodeSkipped ||
				!strings.Contains(res.Diagnostics[0].Message, string(tt.want)) {
				t.Errorf("diagnostics = %v, want one skipped diagnostic", res.Diagnostics)
			}
			if res.Ranges == nil || res.Funcs == nil || res.Decls == nil || len(res.Ranges)+len(res.Funcs)+len(res.Decls) != 0 {
				t.Errorf("got %+v, want empty non-nil fields", res)
			}
		})
	}
}

func TestAnalyzeFileSkipLine(t *testing.T) {
	res := AnalyzeFile(nil, "x.go", []byte(plusChain(40)), Options{MaxASTDepth: 20})
	if d := res.Diagnostics; len(d) != 1 || d[0].Line != 3 {
		t.Errorf("diagnostics = %v, want one on line 3", d)
	}
}

// TestAnalyzeFileFlatChains checks that long chains of binary operators
// are limited by the depth of the syntax tree, which grows by one level
// per operator.
func TestAnalyzeFileFlatChains(t *testing.T) {
	for _, tt := range []struct {
		term string
		n    int
		want string
	}{
		{"s.a", 800, ""},
		{"s.a", 1400, ""},
		{"f(x)", 1400, ""},
		{"a[i]", 1400, ""},
		{"-a", 1400, ""},
		{"*p", 1400, ""},
		{"s.a", 1600, "syntax tree is more than 1500 levels deep"},
	} {
		src := "package p\n\nvar x = " + joinTerms(tt.n, tt.term) + "\n"
		res := AnalyzeFile(nil, "x.go", []byte(src), DefaultOptions())
		switch {
		case tt.want == "" && res.Skipped != "":
			t.Errorf("%d terms %s: skipped: %v", tt.n, tt.term, res.Diagnostics)
		case tt.want != "" && (res.Skipped != result.SkipTooDeep || !strings.Contains(res.Diagnostics[0].Message, tt.want)):
			t.Errorf("%d terms %s: got %q %v, want too-deep: %s", tt.n, tt.term, res.Skipped, res.Diagnostics, tt.want)
		}
	}
	if res := AnalyzeFile(nil, "x.go", []byte(emptyCases(1600)), DefaultOptions()); res.Skipped != "" {
		t.Errorf("1600 empty case clauses: skipped: %v", res.Diagnostics)
	}
}
