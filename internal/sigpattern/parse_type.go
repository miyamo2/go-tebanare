package sigpattern

import (
	"errors"
	"go/ast"
	goparser "go/parser"
	"go/scanner"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/astmatch"
)

// parseType parses a type. With union set, the type may be a union of
// terms and a term may be "~T", as in constraints and interface elements.
func (p *parser) parseType(union bool) ast.Expr {
	x := p.parseTerm(union)
	if union {
		x = p.unionRest(x)
	}
	return x
}

// unionRest parses the "| term" continuations of a union that starts with
// x. The union is left-associative, as go/parser builds it. Each term adds
// one level to the nesting depth, since the result nests that deeply.
func (p *parser) unionRest(x ast.Expr) ast.Expr {
	depth := p.depth
	for p.err == nil && p.peekCont().tok == token.OR {
		p.next()
		if !p.deeper() {
			break
		}
		x = &ast.BinaryExpr{X: x, Op: token.OR, Y: p.parseTerm(true)}
	}
	p.depth = depth
	return x
}

// deeper increases the nesting depth and reports an error past maxDepth.
func (p *parser) deeper() bool {
	if p.depth >= maxDepth {
		p.errorAt(p.peek().off, "pattern is nested too deeply")
		return false
	}
	p.depth++
	return true
}

// parseTerm parses a type that is not a union and tracks the nesting
// depth. The work happens in term.
func (p *parser) parseTerm(union bool) ast.Expr {
	if p.err != nil || !p.deeper() {
		return nil
	}
	x := p.term(union)
	p.depth--
	return x
}

func (p *parser) term(union bool) ast.Expr {
	switch t := p.peek(); t.tok {
	case token.IDENT:
		p.next()
		return p.typeName(t)
	case token.MUL:
		p.next()
		return &ast.StarExpr{X: p.parseTerm(false)}
	case token.LBRACK:
		return p.parseArray()
	case token.MAP:
		p.next()
		p.expect(token.LBRACK)
		key := p.parseType(false)
		p.expect(token.RBRACK)
		return &ast.MapType{Key: key, Value: p.parseTerm(false)}
	case token.CHAN:
		p.next()
		dir := ast.SEND | ast.RECV
		if p.peek().tok == token.ARROW {
			p.next()
			dir = ast.SEND
		}
		return &ast.ChanType{Dir: dir, Value: p.parseTerm(false)}
	case token.ARROW:
		p.next()
		p.expect(token.CHAN)
		return &ast.ChanType{Dir: ast.RECV, Value: p.parseTerm(false)}
	case token.FUNC:
		p.next()
		return p.parseSignature()
	case token.INTERFACE:
		return p.parseInterface()
	case token.STRUCT:
		return p.parseStruct()
	case token.LPAREN:
		p.next()
		x := p.parseType(union)
		p.expect(token.RPAREN)
		return &ast.ParenExpr{X: x}
	case token.TILDE:
		if union {
			p.next()
			return &ast.UnaryExpr{Op: token.TILDE, X: p.parseTerm(false)}
		}
	}
	p.errorExpected(p.peek(), "type")
	return nil
}

// typeName parses the rest of a type that starts with the identifier t:
// an optional qualifier and optional type arguments.
func (p *parser) typeName(t item) ast.Expr {
	var x ast.Expr
	switch {
	case p.peekCont().tok == token.PERIOD:
		p.next()
		sel := p.expect(token.IDENT)
		x = &ast.SelectorExpr{X: ast.NewIdent(t.lit), Sel: ast.NewIdent(sel.lit)}
	case t.lit == "_" || p.declared[t.lit]:
		return p.identType(t.lit) // takes no type arguments
	default:
		x = ast.NewIdent(t.lit)
	}
	if p.err == nil && p.peekCont().tok == token.LBRACK {
		x = p.typeArgs(x)
	}
	return x
}

// typeArgs parses the type arguments of x. "..." matches zero or more
// arguments.
func (p *parser) typeArgs(x ast.Expr) ast.Expr {
	p.next() // '['
	var args []ast.Expr
	for p.err == nil {
		if p.peek().tok == token.ELLIPSIS {
			p.next()
			args = append(args, astmatch.Seq())
		} else {
			args = append(args, p.parseType(false))
		}
		if !p.listSep(token.RBRACK) {
			break
		}
	}
	p.expect(token.RBRACK)
	if len(args) == 1 {
		return &ast.IndexExpr{X: x, Index: args[0]}
	}
	return &ast.IndexListExpr{X: x, Indices: args}
}

// parseArray parses "[]T" or "[N]T". "[_]T" matches an array of any length.
func (p *parser) parseArray() ast.Expr {
	p.next() // '['
	switch t := p.peek(); t.tok {
	case token.RBRACK:
		p.next()
		return &ast.ArrayType{Elt: p.parseTerm(false)}
	case token.ELLIPSIS:
		p.errorAt(t.off, "invalid use of [...] array (outside a composite literal)")
		return nil
	}
	length := p.parseLength()
	if p.err != nil {
		return nil
	}
	return &ast.ArrayType{Len: length, Elt: p.parseTerm(false)}
}

// parseLength reads the tokens of an array length up to the matching ']'
// and parses their text with go/parser. A lone "_" is astmatch.Any.
func (p *parser) parseLength() ast.Expr {
	first := p.peek()
	depth, n := 0, 0
	for p.err == nil {
		t := p.peek()
		switch t.tok {
		case token.EOF:
			p.errorExpected(t, "']'")
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACE:
			if depth == 0 {
				p.errorExpected(t, "']'")
			}
			depth--
		case token.RBRACK:
			depth--
		}
		if depth < 0 || p.err != nil {
			break
		}
		if n++; n > maxLengthTokens || depth > maxDepth {
			p.errorAt(first.off, "array length is too long")
		}
		p.next()
	}
	if p.err != nil {
		return nil
	}
	end := p.lx.prevEnd
	p.next() // ']'
	if n == 1 && first.tok == token.IDENT && first.lit == "_" {
		return astmatch.Any()
	}
	// AllErrors: in its default mode the parser stops after ten errors with
	// a panic that it recovers itself, and TinyGo's wasm targets cannot
	// recover.
	e, err := goparser.ParseExprFrom(token.NewFileSet(), "", p.src[first.off:end], goparser.AllErrors)
	if err != nil {
		var list scanner.ErrorList
		if errors.As(err, &list) && len(list) > 0 {
			p.errorAt(first.off+list[0].Pos.Offset, list[0].Msg)
		} else {
			p.errorAt(first.off, err.Error())
		}
		return nil
	}
	return e
}
