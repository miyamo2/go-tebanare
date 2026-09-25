package sigpattern

import (
	"go/ast"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/astmatch"
)

// maxDepth limits how deeply types may nest in a pattern. It keeps the
// recursive parser and matcher within a small stack.
const maxDepth = 100

// maxLengthTokens limits the number of tokens in an array length.
const maxLengthTokens = 256

func newParser(src string) *parser {
	p := &parser{src: src, declared: map[string]bool{}}
	p.lx.init(src)
	return p
}

type parser struct {
	src      string
	lx       lexer
	err      *Error
	declared map[string]bool // type parameters declared by the pattern
	depth    int             // type nesting depth
	bodies   int             // number of enclosing interface or struct bodies
}

func (p *parser) errorAt(off int, msg string) {
	if p.err == nil {
		p.err = newError(p.src, off, msg)
	}
}

// errorExpected reports "expected <what>, found <t>" in go/parser wording,
// or the scanner error attached to t.
func (p *parser) errorExpected(t item, what string) {
	if t.errMsg != "" {
		p.errorAt(t.errOff, t.errMsg)
		return
	}
	found := "'" + t.tok.String() + "'"
	switch {
	case t.tok == token.SEMICOLON && t.lit == "\n":
		found = "newline"
	case t.tok.IsLiteral():
		found = t.lit
	}
	p.errorAt(t.off, "expected "+what+", found "+found)
}

func (p *parser) peek() item { return p.lx.peek() }

// peekCont is peek for tokens that would continue the current element. In
// an interface or struct body a newline ends the element as in Go, so the
// token after it reads as a semicolon. Everywhere else newlines are spaces.
func (p *parser) peekCont() item {
	t := p.lx.peek()
	if p.bodies > 0 && t.nl {
		t.tok, t.lit = token.SEMICOLON, "\n"
	}
	return t
}

// next consumes a token and reports its scanner error, if any.
func (p *parser) next() item {
	t := p.lx.next()
	if t.errMsg != "" {
		p.errorAt(t.errOff, t.errMsg)
	}
	return t
}

func (p *parser) expect(tok token.Token) item {
	t := p.peek()
	if t.tok != tok {
		p.errorExpected(t, "'"+tok.String()+"'")
		return t
	}
	return p.next()
}

// listSep consumes a ',' between list elements. It reports whether another
// element follows; a trailing ',' before close is allowed.
func (p *parser) listSep(close token.Token) bool {
	if p.err != nil || p.peek().tok != token.COMMA {
		return false
	}
	p.next()
	return p.peek().tok != close
}

// identType returns the type that a lone identifier denotes.
func (p *parser) identType(name string) ast.Expr {
	switch {
	case name == "_":
		return astmatch.Any()
	case p.declared[name]:
		return astmatch.TParam(name)
	}
	return ast.NewIdent(name)
}
