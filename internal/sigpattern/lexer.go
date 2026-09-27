package sigpattern

import (
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"
)

// item is one token of a pattern.
type item struct {
	tok token.Token
	lit string
	off int // byte offset of the first byte
	end int // byte offset after the last byte
	// nl reports that Go would insert a semicolon before the token,
	// because a newline follows the previous token.
	nl bool
	// errMsg is the first scanner error reported while scanning the
	// token, at byte offset errOff. go/scanner reads one rune ahead, so an
	// error in the rune right after a token (an invalid UTF-8 byte or a
	// byte order mark, as in "Get\xff" or "a\uFEFFb") is carried by that
	// token, not by the next one. Callers must judge an error by errOff,
	// not by which token carries it.
	errMsg string
	errOff int
}

// lexer tokenizes a pattern with go/scanner. It scans lazily, one token of
// lookahead. The parser can read a /regexp/ name as raw text, after which
// scanning restarts, and can mark a position and rewind to it after a
// trial parse. A rewind replays exactly the tokens, including errors and
// nl flags, that were scanned after the mark.
type lexer struct {
	src     string
	sc      scanner.Scanner
	file    *token.File
	base    int  // offset of the scanner's input in src
	cur     item // lookahead, valid when has is set
	has     bool
	prevEnd int // end offset of the last consumed token
	errMsg  string
	errOff  int
}

func (l *lexer) init(src string) {
	*l = lexer{src: src}
	l.start(0, src)
}

// start makes the scanner read input, which holds the source from byte
// offset base on.
func (l *lexer) start(base int, input string) {
	fset := token.NewFileSet()
	l.file = fset.AddFile("", -1, len(input))
	l.base = base
	l.has = false
	l.errMsg = ""
	l.sc.Init(l.file, []byte(input), l.onError, 0)
}

func (l *lexer) onError(pos token.Position, msg string) {
	if l.errMsg == "" {
		l.errMsg, l.errOff = msg, l.base+pos.Offset
	}
}

// peek returns the next token without consuming it. Semicolons that
// go/scanner inserts at newlines are dropped and recorded in item.nl.
func (l *lexer) peek() item {
	if l.has {
		return l.cur
	}
	nl := false
	for {
		pos, tok, lit := l.sc.Scan()
		if tok == token.SEMICOLON && lit == "\n" {
			nl = true
			continue
		}
		it := item{tok: tok, lit: lit, off: l.base + l.file.Offset(pos), nl: nl}
		it.end = l.tokenEnd(it)
		if l.errMsg != "" {
			it.errMsg, it.errOff = l.errMsg, l.errOff
			l.errMsg = ""
		}
		l.cur, l.has = it, true
		return it
	}
}

func (l *lexer) tokenEnd(it item) int {
	switch {
	case it.tok == token.EOF:
		return it.off
	case it.tok == token.ILLEGAL:
		_, size := utf8.DecodeRuneInString(l.src[it.off:])
		return it.off + size
	case it.tok == token.STRING && strings.HasPrefix(it.lit, "`"):
		// go/scanner drops carriage returns from raw strings, so the
		// literal can be shorter than its source text.
		if i := strings.IndexByte(l.src[it.off+1:], '`'); i >= 0 {
			return it.off + i + 2
		}
		return len(l.src)
	case it.tok.IsLiteral():
		return it.off + len(it.lit)
	}
	return it.off + len(it.tok.String())
}

// next consumes and returns the next token.
func (l *lexer) next() item {
	it := l.peek()
	l.has = false
	l.prevEnd = it.end
	return it
}

// slash reports whether the next non-blank byte after the last consumed
// token is '/', and returns its offset. The parser calls it where a name
// is expected, before the scanner can read the '/' as an operator or a
// comment. Only blanks are skipped, not comments: in "func /* x */ F()"
// the name is the regexp "* x *".
func (l *lexer) slash() (int, bool) {
	o := l.prevEnd
	for o < len(l.src) && isSpace(l.src[o]) {
		o++
	}
	return o, o < len(l.src) && l.src[o] == '/'
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// regexp reads the raw regular expression that starts with the '/' at
// offset off and ends at the next '/' that is not escaped. A backslash
// escapes the byte after it: "\/" becomes "/" and any other pair is kept
// as written. ok is false when the closing '/' is missing. Scanning
// restarts after the closing '/', in the state that follows an
// identifier, so a newline after the regexp sets nl on the next token.
func (l *lexer) regexp(off int) (expr string, ok bool) {
	var b strings.Builder
	for i := off + 1; i < len(l.src); i++ {
		c := l.src[i]
		switch {
		case c == '\\' && i+1 < len(l.src):
			if l.src[i+1] != '/' {
				b.WriteByte(c)
			}
			b.WriteByte(l.src[i+1])
			i++
		case c == '/':
			// Scan ")" in place of the closing '/' and drop it. The
			// scanner then inserts a semicolon at a following newline as
			// after an identifier, and reports a byte order mark after
			// the regexp as it would anywhere else. ")" cannot join
			// with the byte after it into a longer token.
			l.start(i, ")"+l.src[i+1:])
			l.sc.Scan()
			l.prevEnd = i + 1
			return b.String(), true
		default:
			b.WriteByte(c)
		}
	}
	return "", false
}

// mark is a saved lexer state for rewind. It copies the whole lexer,
// scanner included, so that a rewind does not rescan anything: rescanning
// from a token's offset would miss errors in comments before it and the
// newline state left by the token before it.
type mark struct{ l lexer }

func (l *lexer) mark() mark {
	return mark{*l}
}

// rewind restores the state saved by mark. The token.File is shared with
// the saved scanner; rescanning adds the same line offsets again, which
// the file ignores.
func (l *lexer) rewind(m mark) {
	*l = m.l
}
