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
	// errMsg is the scanner error reported while scanning the token, at
	// byte offset errOff.
	errMsg string
	errOff int
}

// lexer tokenizes a pattern with go/scanner. It scans lazily, one token of
// lookahead, and can restart at any byte offset. Restarting lets the parser
// read a /regexp/ name as raw text and rewind after a trial parse.
type lexer struct {
	src     string
	sc      scanner.Scanner
	file    *token.File
	base    int  // offset of the scanner's input in src
	cur     item // lookahead, valid when has is set
	has     bool
	prevEnd int  // end offset of the last consumed token
	nlNext  bool // nl value for the next scanned token after a rewind
	errMsg  string
	errOff  int
}

func (l *lexer) init(src string) {
	l.src = src
	l.reset(0)
}

// reset restarts scanning at byte offset off.
func (l *lexer) reset(off int) {
	fset := token.NewFileSet()
	l.file = fset.AddFile("", -1, len(l.src)-off)
	l.base = off
	l.has = false
	l.nlNext = false
	l.errMsg = ""
	l.sc.Init(l.file, []byte(l.src[off:]), l.onError, 0)
	if off > 0 && strings.HasPrefix(l.src[off:], "\uFEFF") {
		// go/scanner skips a byte order mark at the start of its input.
		// Only the pattern itself may start with one.
		l.errMsg, l.errOff = "illegal byte order mark", off
	}
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
	nl := l.nlNext
	l.nlNext = false
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
// comment.
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
// restarts after the closing '/'.
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
			l.reset(i + 1)
			l.prevEnd = i + 1
			return b.String(), true
		default:
			b.WriteByte(c)
		}
	}
	return "", false
}

// mark is a saved lexer position for rewind.
type mark struct {
	off, prevEnd int
	nl           bool
}

func (l *lexer) mark() mark {
	it := l.peek()
	return mark{off: it.off, prevEnd: l.prevEnd, nl: it.nl}
}

func (l *lexer) rewind(m mark) {
	l.reset(m.off)
	l.prevEnd = m.prevEnd
	l.nlNext = m.nl
}
