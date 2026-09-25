package sigpattern

import (
	"go/token"
	"regexp"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/regexlimit"
)

// name is a glob or a regular expression.
type name struct {
	text   string         // the glob, or the regexp with "\/" unescaped
	regexp bool           // text is a regular expression
	re     *regexp.Regexp // nil for a glob without '*' or '?'
}

func (n name) match(s string) bool {
	if n.re != nil {
		return n.re.MatchString(s)
	}
	return s == n.text
}

// isGlobToken reports whether t can be part of a name glob.
func isGlobToken(t item) bool {
	switch t.tok {
	case token.IDENT, token.INT, token.FLOAT, token.IMAG, token.MUL:
		return true
	case token.ILLEGAL:
		return t.lit == "?"
	}
	return t.tok.IsKeyword()
}

// globRun reads adjacent glob tokens as one name glob and returns its
// text. plainIdent reports that the run is a single identifier.
func (p *parser) globRun() (text string, plainIdent bool) {
	t := p.peek()
	start, n := t.off, 0
	for isGlobToken(t) && (n == 0 || t.off == p.lx.prevEnd) {
		p.lx.next()
		// A '?' carries the scanner's "illegal character" error, which a
		// glob ignores. An error at another offset is about the character
		// after the '?', since go/scanner reads one character ahead.
		if t.errMsg != "" && (t.tok != token.ILLEGAL || t.errOff != t.off) {
			p.errorAt(t.errOff, t.errMsg)
		}
		plainIdent = n == 0 && t.tok == token.IDENT
		n++
		t = p.peek()
	}
	return p.src[start:p.lx.prevEnd], plainIdent
}

// parseName parses a name glob or a /regexp/. what names the element in
// error messages.
func (p *parser) parseName(what string) name {
	if off, slash := p.lx.slash(); slash {
		return p.parseRegexp(off)
	}
	t := p.peek()
	if !isGlobToken(t) {
		p.errorExpected(t, what)
		return name{}
	}
	text, _ := p.globRun()
	return p.globName(text)
}

func (p *parser) parseRegexp(off int) name {
	expr, ok := p.lx.regexp(off)
	if !ok {
		p.errorAt(off, "regexp not terminated")
		return name{}
	}
	if err := regexlimit.Check(expr); err != nil {
		p.errorAt(off, err.Error())
		return name{}
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		p.errorAt(off, err.Error())
		return name{}
	}
	return name{text: expr, regexp: true, re: re}
}

// globName compiles a glob to an anchored regexp: '*' matches any run of
// runes, '?' matches one rune, and every other rune matches itself.
func (p *parser) globName(glob string) name {
	if !strings.ContainsAny(glob, "*?") {
		return name{text: glob}
	}
	var b strings.Builder
	b.WriteString("^(?s:")
	for _, r := range glob {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString(")$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		p.errorAt(p.lx.prevEnd, err.Error())
		return name{}
	}
	return name{text: glob, re: re}
}
