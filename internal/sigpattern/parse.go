package sigpattern

import (
	"go/ast"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/astmatch"
)

// Parse parses a signature pattern. The returned error is an *Error.
func Parse(src string) (*Pattern, error) {
	p := newParser(src)
	pat := p.parsePattern()
	if p.err != nil {
		return nil, p.err
	}
	return pat, nil
}

func (p *parser) parsePattern() *Pattern {
	pat := &Pattern{}
	p.expect(token.FUNC)
	if _, slash := p.lx.slash(); !slash && p.err == nil && p.peek().tok == token.LPAREN {
		pat.recv = p.parseReceiver()
	}
	if p.err == nil {
		pat.name = p.parseName("name")
	}
	if t := p.peek(); p.err == nil && t.tok == token.LBRACK {
		if pat.recv != nil {
			p.errorAt(t.off, "methods cannot have type parameters")
			return nil
		}
		pat.tparams = p.parseTypeParams()
	}
	if t := p.peek(); p.err == nil && t.tok != token.EOF {
		if t.tok != token.LPAREN {
			p.errorExpected(t, "'('")
			return nil
		}
		pat.sig = p.parseSignature()
	}
	p.expect(token.EOF)
	return pat
}

// parseReceiver parses "(" [identifier] RecvType ")".
func (p *parser) parseReceiver() *receiver {
	p.next() // '('
	if _, slash := p.lx.slash(); !slash && p.peek().tok == token.IDENT {
		p.receiverName()
	}
	r := p.parseRecvType()
	p.expect(token.RPAREN)
	return r
}

// receiverName consumes the identifier at the next token when it is the
// receiver name, which is when the receiver type follows it. Otherwise the
// identifier starts the type and nothing is consumed.
//
// For a '*' right after the identifier: "h*" followed by ')' or '[' is a
// value receiver glob such as "Mock*", and "h*" followed by a separate
// name is the name h and a pointer, as in Go. "h*Handler" is an error: Go
// reads it as the name h and the type *Handler, and a glob reads it as one
// type name.
func (p *parser) receiverName() {
	m := p.lx.mark()
	id := p.next()
	if _, slash := p.lx.slash(); slash {
		return // "h /re/"
	}
	t := p.peek()
	if t.off != id.end || !isGlobToken(t) {
		if t.tok == token.RPAREN || t.tok == token.LBRACK {
			p.lx.rewind(m) // "(T)" or "(T[K])"
		}
		return
	}
	if t.tok != token.MUL {
		p.lx.rewind(m) // a glob such as "Get?"
		return
	}
	star := p.lx.mark()
	p.next()
	_, slash := p.lx.slash()
	switch u := p.peek(); {
	case !slash && u.off == t.end && isGlobToken(u):
		p.lx.rewind(m)
		glob, _ := p.globRun()
		p.errorAt(t.off, `ambiguous receiver: write "`+id.lit+" "+glob[len(id.lit):]+
			`" for a pointer receiver or "_ `+glob+`" for a value receiver glob`)
	case !slash && (u.tok == token.RPAREN || u.tok == token.LBRACK):
		p.lx.rewind(m) // "(Mock*)"
	default:
		p.lx.rewind(star) // "(h* Handler)"
	}
}

func (p *parser) parseRecvType() *receiver {
	r := &receiver{}
	if _, slash := p.lx.slash(); !slash && p.peek().tok == token.MUL {
		p.next()
		r.pointer = true
	}
	if off, slash := p.lx.slash(); slash {
		r.name = p.parseRegexp(off)
	} else {
		t := p.peek()
		if !isGlobToken(t) {
			p.errorExpected(t, "receiver type")
			return r
		}
		text, plain := p.globRun()
		if plain && text == "_" {
			r.anyType = true
			return r
		}
		r.name = p.globName(text)
	}
	if p.err == nil && p.peek().tok == token.LBRACK {
		r.args = p.parseRecvArgs()
	}
	return r
}

// parseRecvArgs parses receiver type arguments. Each one is "...", "_",
// or a name that declares a type parameter.
func (p *parser) parseRecvArgs() *ast.FieldList {
	p.next() // '['
	fl := &ast.FieldList{}
	for p.err == nil {
		t := p.peek()
		var id *ast.Ident
		switch {
		case t.tok == token.ELLIPSIS:
			id = astmatch.Seq()
		case t.tok == token.IDENT && t.lit == "_":
			id = astmatch.Any()
		case t.tok == token.IDENT:
			p.declare(t)
			id = ast.NewIdent(t.lit)
		default:
			p.errorExpected(t, "type parameter name")
			return nil
		}
		p.next()
		fl.List = append(fl.List, &ast.Field{Names: []*ast.Ident{id}})
		if !p.listSep(token.RBRACK) {
			break
		}
	}
	p.expect(token.RBRACK)
	return fl
}

// parseTypeParams parses a function type parameter list. A run of bare
// names followed by a name with a constraint shares that constraint, as in
// Go. "_" and "..." end a run, and a run that ends without a constraint
// accepts any constraint (a nil Field.Type).
func (p *parser) parseTypeParams() *ast.FieldList {
	p.prescan()
	p.next() // '['
	if t := p.peek(); p.err == nil && t.tok == token.RBRACK {
		p.errorAt(t.off, "empty type parameter list")
	}
	fl := &ast.FieldList{}
	var run []*ast.Field
	for p.err == nil {
		t := p.peek()
		f := &ast.Field{}
		switch {
		case t.tok == token.ELLIPSIS:
			f.Names = []*ast.Ident{astmatch.Seq()}
			run = nil
		case t.tok == token.IDENT && t.lit == "_":
			f.Names = []*ast.Ident{astmatch.Any()}
			run = nil
		case t.tok == token.IDENT:
			f.Names = []*ast.Ident{ast.NewIdent(t.lit)}
			run = append(run, f)
		default:
			p.errorExpected(t, "type parameter")
			return nil
		}
		p.next()
		fl.List = append(fl.List, f)
		if n := p.peek(); len(run) > 0 && n.tok != token.COMMA && n.tok != token.RBRACK {
			constraint := p.parseType(true)
			for _, g := range run {
				g.Type = constraint
			}
			run = nil
		}
		if !p.listSep(token.RBRACK) {
			break
		}
	}
	p.expect(token.RBRACK)
	return fl
}

// prescan declares the names of the type parameter list that starts at
// the next token, so a constraint can refer to a parameter declared after
// it. The lexer is rewound afterwards.
func (p *parser) prescan() {
	m := p.lx.mark()
	p.lx.next() // '['
	depth, first := 0, true
loop:
	for p.err == nil {
		t := p.lx.next()
		switch t.tok {
		case token.EOF:
			break loop
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if depth == 0 {
				break loop
			}
			depth--
		case token.COMMA:
			if depth == 0 {
				first = true
				continue
			}
		case token.IDENT:
			if first && depth == 0 && t.lit != "_" {
				p.declare(t)
			}
		}
		first = false
	}
	p.lx.rewind(m)
}

func (p *parser) declare(t item) {
	if p.declared[t.lit] {
		p.errorAt(t.off, "type parameter "+t.lit+" redeclared")
		return
	}
	p.declared[t.lit] = true
}
