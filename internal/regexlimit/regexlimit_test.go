package regexlimit

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"
	"unicode"
)

// panicCodes are the errors regexp/syntax reports by panicking and
// recovering, which the TinyGo wasm build cannot do.
var panicCodes = []syntax.ErrorCode{syntax.ErrNestingDepth, syntax.ErrLarge}

// checkInvariant fails t when Check accepts expr but parsing expr reaches
// one of the limits regexp/syntax enforces with a panic.
func checkInvariant(t *testing.T, expr string) {
	t.Helper()
	if Check(expr) != nil {
		return
	}
	_, err := syntax.Parse(expr, syntax.Perl)
	var serr *syntax.Error
	if errors.As(err, &serr) {
		for _, code := range panicCodes {
			if serr.Code == code {
				t.Errorf("Check accepts %.40q... (%d bytes), but parsing it fails: %v", expr, len(expr), code)
			}
		}
	}
}

func TestCheck(t *testing.T) {
	for _, tt := range []struct {
		expr string
		err  string
	}{
		{`^Get[A-Z]\w*$`, ""},
		{`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`, ""},
		{`(?i)\pL{0,1000}`, ""},
		{strings.Repeat("a", MaxLength), ""},
		{strings.Repeat("a", MaxLength+1), `the expression is 4097 bytes long, more than the limit of 4096`},
		{strings.Repeat("(", 99) + strings.Repeat(")", 99), ""},
		{strings.Repeat("(", 100) + strings.Repeat(")", 100), `the expression nests too deeply (estimated depth 604, more than the limit of 600)`},
		{`a{1000}{400}`, `the expression repeats too much (its length times its repetition counts is more than the limit of 400000)`},
		// Parentheses in escapes, \Q...\E, and classes are not groups.
		{strings.Repeat(`\(`, 150) + strings.Repeat(`[(]`, 150) + `\Q` + strings.Repeat("(", 150) + `\E`, ""},
		// "{n}" counts wherever it appears.
		{`[{1000}]{400}`, `the expression repeats too much (its length times its repetition counts is more than the limit of 400000)`},
	} {
		got := ""
		if err := Check(tt.expr); err != nil {
			got = err.Error()
		}
		if got != tt.err {
			t.Errorf("Check(%.40q) = %q, want %q", tt.expr, got, tt.err)
		}
	}
}

// TestDepthBound checks that the deepest shapes Check accepts stay below
// the nesting limit of regexp/syntax, which it enforces with a panic.
func TestDepthBound(t *testing.T) {
	for _, group := range []struct{ open, close string }{
		{"(", ")"},
		{"(", ")*"},
		{"(x|", ")+"},
		{"(x|y", "z)*"},
		{"(?:x|", ")*"},
		// Factoring the alternatives adds two levels: six per group.
		{"(x|y", "x|y)*"},
		{"(x|y", "x|y)x"},
		{"(x|y", "x|y)$"},
		{"(x|y", "x|y)+?"},
		{"(xy|z", "x|y)*"},
		{"(abcd|abc|ab|a", "|)*"},
	} {
		n := 0
		for Check(nest(group.open, group.close, n+1)) == nil {
			n++
		}
		if n == 0 {
			t.Errorf("%s...%s: Check rejects a single group", group.open, group.close)
			continue
		}
		if _, err := regexp.Compile(nest(group.open, group.close, n)); err != nil {
			t.Errorf("%s...%s (%d groups): %v", group.open, group.close, n, err)
		}
	}
}

func nest(open, close string, n int) string {
	return strings.Repeat(open, n) + "a" + strings.Repeat(close, n)
}

// TestBypasses holds expressions that make regexp/syntax panic; Check
// must reject every one of them.
func TestBypasses(t *testing.T) {
	for _, expr := range []string{
		nest("(x|y", "x|y)*", 167),
		nest("(x|y", "x|y)x", 200),
		nest("(x|y", "x|y)$", 200),
		nest("(x|y", "x|y)+?", 200),
		nest("(xy|z", "x|y)*", 251),
		"(?:" + strings.Repeat("a", 3400) + "){1000}",
		strings.Repeat("a{1000}", 3400),
		strings.Repeat(`[\pL\pN]`, 27000),
		"(?:(?:" + strings.Repeat("a", 4) + "){1000}){1000}",
	} {
		if Check(expr) == nil {
			t.Errorf("Check accepts %.40q... (%d bytes)", expr, len(expr))
		}
		checkInvariant(t, expr)
	}
}

// TestRuneBound checks the figures the package documentation uses to
// bound the runes in character classes.
func TestRuneBound(t *testing.T) {
	const maxRunes = 128 << 20 / 4
	folds := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) != r {
			folds++
		}
	}
	if folds > 2878 {
		t.Errorf("%d runes have a case fold, more than the documented 2878", folds)
	}

	var classes []string
	for name := range unicode.Categories {
		classes = append(classes, `\p{`+name+`}`, `\P{`+name+`}`, `\p{^`+name+`}`)
		if len(name) == 1 {
			classes = append(classes, `\p`+name, `\P`+name)
		}
	}
	for name := range unicode.Scripts {
		classes = append(classes, `\p{`+name+`}`, `\P{`+name+`}`, `\p{^`+name+`}`)
	}
	classes = append(classes, `\p{Any}`, `\P{Any}`, `\d`, `\D`, `\s`, `\S`, `\w`, `\W`)
	for _, name := range []string{"alnum", "alpha", "ascii", "blank", "cntrl", "digit", "graph", "lower", "print", "punct", "space", "upper", "word", "xdigit"} {
		classes = append(classes, `[:`+name+`:]`, `[:^`+name+`:]`)
	}
	worst := 0.0
	for _, class := range classes {
		for _, flags := range []string{"", "(?i)"} {
			expr := flags + class
			if strings.HasPrefix(class, "[:") {
				expr = flags + "[" + class + "]"
			}
			re, err := syntax.Parse(expr, syntax.Perl)
			if err != nil {
				continue // a script regexp/syntax does not support
			}
			worst = max(worst, float64(runes(re))/float64(len(class)))
		}
	}
	if worst > 500 {
		t.Errorf("a class escape holds %.1f runes per byte, more than the documented 500", worst)
	}
	if perByte := 500 + (2*2878+2)/2; MaxLength*perByte > maxRunes/2 {
		t.Errorf("MaxLength*%d = %d runes, more than half of maxRunes", perByte, MaxLength*perByte)
	}
}

func runes(re *syntax.Regexp) int {
	n := len(re.Rune)
	for _, sub := range re.Sub {
		n += runes(sub)
	}
	return n
}

// FuzzCheck checks that regexp/syntax never reaches a limit it enforces
// with a panic for an expression that Check accepts.
func FuzzCheck(f *testing.F) {
	for _, seed := range []string{
		`^Get[A-Z]\w*$`,
		`(?i)\PL{0,1000}`,
		`(x|y(x|y(x|ya)*x|y)*x|y)*`,
		`(abcd|abc|ab|a(abcd|abc|ab|a)|)*`,
		`(?:(?:a{1000}){1000}){1000}`,
		`[\pL\pN]{1000}`,
		`\Q(((\E[(]\(`,
		`(?P<n>a)|(?:b{2,}|c{3,5})+?`,
		fmt.Sprintf("(?:%s){30}", strings.Repeat("a", 100)),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		checkInvariant(t, expr)
		// Repeating the input probes nesting and size far beyond what
		// the fuzzer reaches by mutation alone.
		for _, n := range []int{8, 64, 512} {
			if len(expr)*n <= 2*MaxLength {
				checkInvariant(t, strings.Repeat(expr, n))
			}
		}
		// Nesting the parts before and after the first "a" probes the
		// height per group.
		if open, close, ok := strings.Cut(expr, "a"); ok {
			for _, n := range []int{50, 100, 200} {
				if len(expr)*n <= 2*MaxLength {
					checkInvariant(t, nest(open, close, n))
				}
			}
		}
	})
}
