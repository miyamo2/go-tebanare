package analyzer

import (
	"go/token"
	"testing"
)

func TestLineOf(t *testing.T) {
	for _, tt := range []struct {
		src  string
		off  int
		line int
		col  int
	}{
		{"a\nb\n", 0, 1, 1},
		{"a\nb\n", 1, 1, 2},
		{"a\nb\n", 2, 2, 1},
		{"a\nb\n", 4, 2, 3},
		{"a\nb", 3, 2, 2},
		{"", 0, 1, 1},
	} {
		info := newScanInfo([]byte(tt.src))
		line, col := info.lineCol(tt.off)
		if line != tt.line || col != tt.col {
			t.Errorf("%q offset %d: got %d:%d, want %d:%d", tt.src, tt.off, line, col, tt.line, tt.col)
		}
	}
}

func TestTokenEnd(t *testing.T) {
	src := []byte("x := `a\r\nb` + \"c\\td\" // note\r\n")
	for _, tt := range []struct {
		off  int
		tok  token.Token
		lit  string
		want int
	}{
		{0, token.IDENT, "x", 1},
		{2, token.DEFINE, "", 4},
		// go/scanner drops the \r from the literal.
		{5, token.STRING, "`a\nb`", 11},
		{12, token.ADD, "", 13},
		{14, token.STRING, `"c\td"`, 20},
	} {
		if got := tokenEnd(src, tt.off, tt.tok, tt.lit); got != tt.want {
			t.Errorf("tokenEnd(%d, %s) = %d, want %d", tt.off, tt.tok, got, tt.want)
		}
	}
	// An unterminated raw string ends at the end of the file.
	if got := tokenEnd([]byte("`abc"), 0, token.STRING, "`abc"); got != 4 {
		t.Errorf("unterminated raw string: got %d, want 4", got)
	}
}

func TestMarkOccupies(t *testing.T) {
	// Code tokens of "a; b\n  c\n": a at 0-1, b at 3-4, c at 7-8.
	info := newScanInfo([]byte("a; b\n  c\n"))
	info.mark(0, 1, 1, 1)
	info.mark(3, 4, 1, 1)
	info.mark(7, 8, 2, 2)
	if info.lines() != 2 {
		t.Errorf("lines() = %d, want 2", info.lines())
	}
	for _, tt := range []struct {
		start, end int
		ok         bool
		line       int
	}{
		{0, 1, false, 1},
		{0, 4, true, 0},
		{3, 4, false, 1},
		{0, 8, true, 0},
		{5, 8, true, 0},
	} {
		if ok, line := info.occupies(tt.start, tt.end); ok != tt.ok || line != tt.line {
			t.Errorf("occupies(%d, %d) = %v, %d, want %v, %d", tt.start, tt.end, ok, line, tt.ok, tt.line)
		}
	}
}
