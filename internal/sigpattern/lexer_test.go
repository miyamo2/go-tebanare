package sigpattern

import (
	"go/token"
	"testing"
)

func scanAll(src string) []item {
	var l lexer
	l.init(src)
	var out []item
	for {
		it := l.next()
		out = append(out, it)
		if it.tok == token.EOF {
			return out
		}
	}
}

func TestLexerTokens(t *testing.T) {
	tests := []struct {
		src  string
		want []item // tok, lit, off, end, nl
	}{
		{"func Get?", []item{
			{tok: token.FUNC, lit: "func", off: 0, end: 4},
			{tok: token.IDENT, lit: "Get", off: 5, end: 8},
			{tok: token.ILLEGAL, lit: "?", off: 8, end: 9},
			{tok: token.EOF, off: 9, end: 9, nl: true},
		}},
		{"(**)", []item{
			{tok: token.LPAREN, off: 0, end: 1},
			{tok: token.MUL, off: 1, end: 2},
			{tok: token.MUL, off: 2, end: 3},
			{tok: token.RPAREN, off: 3, end: 4},
			{tok: token.EOF, off: 4, end: 4, nl: true},
		}},
		{"a\n...", []item{
			{tok: token.IDENT, lit: "a", off: 0, end: 1},
			{tok: token.ELLIPSIS, off: 2, end: 5, nl: true},
			{tok: token.EOF, off: 5, end: 5},
		}},
		{"`a\r\nb` x", []item{
			{tok: token.STRING, lit: "`a\nb`", off: 0, end: 6},
			{tok: token.IDENT, lit: "x", off: 7, end: 8},
			{tok: token.EOF, off: 8, end: 8, nl: true},
		}},
		{"`abc", []item{
			{tok: token.STRING, lit: "`abc", off: 0, end: 4},
			{tok: token.EOF, off: 4, end: 4, nl: true},
		}},
		{"\u00e9\xff", []item{
			{tok: token.IDENT, lit: "\u00e9", off: 0, end: 2},
			{tok: token.ILLEGAL, lit: "\uFFFD", off: 2, end: 3},
			{tok: token.EOF, off: 3, end: 3, nl: true},
		}},
	}
	for _, tt := range tests {
		got := scanAll(tt.src)
		if len(got) != len(tt.want) {
			t.Errorf("%q: got %d tokens, want %d: %+v", tt.src, len(got), len(tt.want), got)
			continue
		}
		for i, w := range tt.want {
			g := got[i]
			if g.tok != w.tok || g.lit != w.lit && w.lit != "" || g.off != w.off || g.end != w.end || g.nl != w.nl {
				t.Errorf("%q token %d: got %+v, want %+v", tt.src, i, g, w)
			}
		}
	}
}

func TestLexerErrors(t *testing.T) {
	got := scanAll("Get? /* x")
	if q := got[1]; q.errMsg != "illegal character U+003F '?'" || q.errOff != 3 {
		t.Errorf("'?': got %q at %d", q.errMsg, q.errOff)
	}
	if eof := got[len(got)-1]; eof.errMsg != "comment not terminated" || eof.errOff != 5 {
		t.Errorf("EOF: got %q at %d", eof.errMsg, eof.errOff)
	}
	// go/scanner skips a byte order mark at the start of its input, so the
	// lexer reports one where it restarts after a regexp.
	var l lexer
	l.init("/a/\xef\xbb\xbf)")
	l.regexp(0)
	if n := l.next(); n.tok != token.RPAREN || n.errMsg != "illegal byte order mark" || n.errOff != 3 {
		t.Errorf("after the regexp: got %+v", n)
	}
	if a := scanAll("\xef\xbb\xbfa")[0]; a.lit != "a" || a.errMsg != "" {
		t.Errorf("leading byte order mark: got %+v", a)
	}
}

func TestLexerRegexp(t *testing.T) {
	tests := []struct {
		src  string
		want string
		ok   bool
		next token.Token
	}{
		{`/^Get/(`, `^Get`, true, token.LPAREN},
		{`/a\/b/ x`, `a/b`, true, token.IDENT},
		{`/\d+\\/]`, `\d+\\`, true, token.RBRACK},
		{`/*x/)`, `*x`, true, token.RPAREN},
		{`// x`, ``, true, token.IDENT},
		{`/ a b /)`, ` a b `, true, token.RPAREN},
		{`/abc`, "", false, 0},
		{`/abc\/`, "", false, 0},
	}
	for _, tt := range tests {
		var l lexer
		l.init(tt.src)
		off, slash := l.slash()
		if !slash || off != 0 {
			t.Fatalf("%q: slash() = %d, %v", tt.src, off, slash)
		}
		got, ok := l.regexp(off)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%q: regexp() = %q, %v; want %q, %v", tt.src, got, ok, tt.want, tt.ok)
		}
		if ok {
			if n := l.next(); n.tok != tt.next {
				t.Errorf("%q: next token %v, want %v", tt.src, n.tok, tt.next)
			}
		}
	}
}

func TestLexerSlashSkipsBlanks(t *testing.T) {
	var l lexer
	l.init("( \n\t/x/")
	l.next()
	if off, ok := l.slash(); !ok || off != 4 {
		t.Errorf("slash() = %d, %v; want 4, true", off, ok)
	}
	l.init("( x")
	l.next()
	if _, ok := l.slash(); ok {
		t.Error("slash() found '/' before x")
	}
}

func TestLexerRewind(t *testing.T) {
	var l lexer
	l.init("a\nb c")
	l.next()
	m := l.mark()
	if b := l.next(); b.lit != "b" || !b.nl {
		t.Fatalf("got %+v", b)
	}
	l.next()
	l.rewind(m)
	if b := l.next(); b.lit != "b" || !b.nl || b.off != 2 || l.prevEnd != 3 {
		t.Errorf("after rewind got %+v, prevEnd %d", b, l.prevEnd)
	}
}
