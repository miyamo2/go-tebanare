package sigpattern

import (
	"go/ast"
	"go/token"
)

// parseInterface parses an interface type. Elements are methods, embedded
// types, and unions, separated by ';' or newlines.
func (p *parser) parseInterface() ast.Expr {
	p.next() // interface
	p.expect(token.LBRACE)
	p.bodies++
	defer func() { p.bodies-- }()
	var list []*ast.Field
	for p.err == nil && p.peek().tok != token.RBRACE {
		f := &ast.Field{}
		if t := p.peek(); t.tok == token.IDENT {
			p.next()
			if p.peekCont().tok == token.LPAREN {
				f.Names = []*ast.Ident{ast.NewIdent(t.lit)}
				f.Type = p.parseSignature()
			} else {
				f.Type = p.unionRest(p.typeName(t))
			}
		} else {
			f.Type = p.parseType(true)
		}
		list = append(list, f)
		if !p.bodySep() {
			break
		}
	}
	p.expect(token.RBRACE)
	return &ast.InterfaceType{Methods: &ast.FieldList{List: list}}
}

// parseStruct parses a struct type with named fields, embedded fields, and
// tags, separated by ';' or newlines.
func (p *parser) parseStruct() ast.Expr {
	p.next() // struct
	p.expect(token.LBRACE)
	p.bodies++
	defer func() { p.bodies-- }()
	var list []*ast.Field
	for p.err == nil && p.peek().tok != token.RBRACE {
		list = append(list, p.parseField())
		if !p.bodySep() {
			break
		}
	}
	p.expect(token.RBRACE)
	return &ast.StructType{Fields: &ast.FieldList{List: list}}
}

func (p *parser) parseField() *ast.Field {
	f := &ast.Field{}
	switch t := p.peek(); t.tok {
	case token.MUL:
		p.next()
		f.Type = &ast.StarExpr{X: p.typeName(p.expect(token.IDENT))}
	case token.IDENT:
		p.next()
		switch n := p.peekCont(); {
		case n.tok == token.LBRACK:
			name, typ := p.identBracket(t)
			if name != nil {
				f.Names = []*ast.Ident{name}
			}
			f.Type = typ
		case n.tok == token.COMMA:
			f.Names = []*ast.Ident{ast.NewIdent(t.lit)}
			for p.err == nil && p.peek().tok == token.COMMA {
				p.next()
				f.Names = append(f.Names, ast.NewIdent(p.expect(token.IDENT).lit))
			}
			f.Type = p.parseType(false)
		case isTypeStart(n.tok):
			f.Names = []*ast.Ident{ast.NewIdent(t.lit)}
			f.Type = p.parseType(false)
		default:
			f.Type = p.typeName(t)
		}
	default:
		p.errorExpected(t, "field name or embedded type")
		return f
	}
	if t := p.peekCont(); p.err == nil && t.tok == token.STRING {
		p.next()
		f.Tag = &ast.BasicLit{Kind: token.STRING, Value: t.lit}
	}
	return f
}

// bodySep consumes the separator after an interface or struct element and
// reports whether another element may follow.
func (p *parser) bodySep() bool {
	t := p.peek()
	switch {
	case p.err != nil || t.tok == token.RBRACE:
		return false
	case t.tok == token.SEMICOLON:
		p.next()
		return true
	case t.tok == token.EOF:
		return false
	case t.nl:
		return true
	}
	p.errorExpected(t, "';'")
	return false
}
