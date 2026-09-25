package sigpattern

import (
	"go/ast"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/astmatch"
)

// parseSignature parses Params [Results].
func (p *parser) parseSignature() *ast.FuncType {
	ft := &ast.FuncType{Params: p.parseParams(false)}
	if p.err == nil {
		ft.Results = p.parseResults()
	}
	return ft
}

// parseResults parses "...", a single type, or a parenthesized list. It
// returns nil when no results follow.
func (p *parser) parseResults() *ast.FieldList {
	t := p.peekCont()
	switch {
	case t.tok == token.ELLIPSIS:
		p.next()
		return &ast.FieldList{List: []*ast.Field{{Type: astmatch.Seq()}}}
	case t.tok == token.LPAREN:
		if fl := p.parseParams(true); fl != nil && len(fl.List) > 0 {
			return fl
		}
		return nil
	case isTypeStart(t.tok):
		return &ast.FieldList{List: []*ast.Field{{Type: p.parseType(false)}}}
	}
	return nil
}

// param is one element of a parameter or result list before Go's
// named/unnamed rule is applied.
type param struct {
	off  int        // offset of the element
	dots int        // offset of the "..." of a variadic element
	seq  bool       // a lone "..."
	name *ast.Ident // the name of a "name Type" element
	bare *ast.Ident // a lone identifier, which is a name or a type
	typ  ast.Expr
}

func (p *parser) parseParams(results bool) *ast.FieldList {
	p.expect(token.LPAREN)
	var params []param
	for p.err == nil && p.peek().tok != token.RPAREN {
		params = append(params, p.parseParam())
		if p.err != nil || p.peek().tok != token.COMMA {
			break
		}
		p.next()
	}
	if t := p.peek(); p.err == nil && t.tok != token.RPAREN {
		if t.errMsg != "" || t.tok == token.EOF {
			p.errorExpected(t, "')'")
		} else {
			p.errorAt(t.off, "missing ',' in parameter list")
		}
	}
	p.expect(token.RPAREN)
	if p.err != nil {
		return nil
	}
	return p.resolveParams(params, results)
}

func (p *parser) parseParam() param {
	t := p.peek()
	switch t.tok {
	case token.ELLIPSIS:
		p.next()
		if n := p.peek(); n.tok == token.COMMA || n.tok == token.RPAREN {
			return param{off: t.off, seq: true}
		}
		return param{off: t.off, dots: t.off, typ: &ast.Ellipsis{Elt: p.parseType(false)}}
	case token.IDENT:
		p.next()
		id := ast.NewIdent(t.lit)
		switch n := p.peek(); n.tok {
		case token.COMMA, token.RPAREN:
			return param{off: t.off, bare: id, typ: p.identType(t.lit)}
		case token.PERIOD:
			return param{off: t.off, typ: p.typeName(t)}
		case token.LBRACK:
			name, typ := p.identBracket(t)
			return param{off: t.off, name: name, typ: typ}
		case token.ELLIPSIS:
			p.next()
			return param{off: t.off, dots: n.off, name: id, typ: &ast.Ellipsis{Elt: p.parseType(false)}}
		}
		return param{off: t.off, name: id, typ: p.parseType(false)}
	}
	return param{off: t.off, typ: p.parseType(false)}
}

// identBracket parses an element that starts with the identifier t and
// '['. It is either a generic type such as "List[int]" or a name followed
// by an array or slice type such as "a [4]int". As in go/parser, the
// brackets hold an array length when a type follows them.
func (p *parser) identBracket(t item) (*ast.Ident, ast.Expr) {
	if t.lit == "_" || p.declared[t.lit] || p.arrayAfterIdent() {
		return ast.NewIdent(t.lit), p.parseTerm(false)
	}
	return nil, p.typeArgs(ast.NewIdent(t.lit))
}

// arrayAfterIdent reports whether the brackets that start at the next
// token hold an array length. They do when a type follows them. When the
// pattern ends inside the brackets or right after them, they do when they
// hold one element that does not start with "...", so that "x [4" reports
// the missing ']' at the end. It scans ahead and rewinds.
func (p *parser) arrayAfterIdent() bool {
	m := p.lx.mark()
	defer p.lx.rewind(m)
	p.lx.next() // '['
	single := p.lx.peek().tok != token.ELLIPSIS
	for depth := 1; depth > 0; {
		switch p.lx.next().tok {
		case token.EOF:
			return single
		case token.COMMA:
			single = single && depth > 1
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			depth--
		}
	}
	next := p.peekCont().tok
	return isTypeStart(next) || (next == token.EOF && single)
}

// resolveParams applies Go's named/unnamed rule to the elements other than
// a lone "...". When any element has a name, every element is named and a
// lone identifier is a name that takes the type of the next named element.
// Otherwise every element is a type. A variadic element must come last,
// apart from trailing "..." elements, and cannot be a result.
func (p *parser) resolveParams(params []param, results bool) *ast.FieldList {
	named := false
	for _, q := range params {
		named = named || q.name != nil
	}
	fl := &ast.FieldList{}
	dots := make([]int, len(params)) // "..." offset of each field's type
	var pending []int                // fields waiting for a type
	for i, q := range params {
		f := &ast.Field{Type: q.typ}
		dots[i] = q.dots
		switch {
		case q.seq:
			f.Type = astmatch.Seq()
		case !named:
		case q.name != nil:
			f.Names = []*ast.Ident{q.name}
			for _, j := range pending {
				fl.List[j].Type, dots[j] = q.typ, q.dots
			}
			pending = nil
		case q.bare != nil:
			f.Names = []*ast.Ident{q.bare}
			pending = append(pending, i)
		default:
			p.errorAt(q.off, "missing parameter name")
			return nil
		}
		fl.List = append(fl.List, f)
	}
	if len(pending) > 0 {
		p.errorAt(params[pending[0]].off, "missing parameter type")
		return nil
	}
	last := len(fl.List) - 1
	for last >= 0 && astmatch.IsSeq(fl.List[last].Type) {
		last--
	}
	for i, f := range fl.List {
		if _, ok := f.Type.(*ast.Ellipsis); !ok {
			continue
		}
		switch {
		case results:
			p.errorAt(dots[i], "invalid use of ...")
			return nil
		case i != last:
			p.errorAt(dots[i], "can only use ... with final parameter")
			return nil
		}
	}
	return fl
}

// isTypeStart reports whether a type can start with tok.
func isTypeStart(tok token.Token) bool {
	switch tok {
	case token.IDENT, token.MUL, token.LBRACK, token.LPAREN, token.MAP,
		token.CHAN, token.FUNC, token.INTERFACE, token.STRUCT, token.ARROW:
		return true
	}
	return false
}
