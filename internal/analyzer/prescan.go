package analyzer

import (
	"fmt"
	"go/scanner"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// skip says why a file was not analyzed.
type skip struct {
	reason result.SkipReason
	// line and column locate the problem. They are 0 when unknown.
	line, column int
	msg          string
}

// maxOps limits the operator chains of the prescan (see scanFile). The
// parser panics when its nesting counter passes 100000, and a chain adds
// one to the counter per operator. The rest of the counter comes from the
// bracket, else-if, and recursion limits, which the default options keep
// below 10000.
const maxOps = 50000

// level is the prescan state of one bracket nesting level.
type level struct {
	// ops counts the operators, the opening brackets, and the keywords
	// func and chan since the last comma, semicolon, else, case, or
	// default on this level.
	ops int
	// nest counts the tokens among them that the parser handles by
	// recursion: prefix operators, type constructors, and colons.
	nest int
	// labels is the part of nest that a binary operator keeps: the count
	// up to the last colon, which may end a label.
	labels int
	// elseIf counts the else-if links of the current if statement on this
	// level.
	elseIf int
	// typ is set when the bracket that opened this level is part of a
	// type: a "[" that nest counts, or the "(" of a parameter list right
	// after func. A "*" or "<-" after the closing bracket then starts a
	// pointer or channel type.
	typ bool
}

// scanFile tokenizes src once. It fills a scanInfo for the occupancy check
// and applies the limits that protect the parser from deep recursion and
// from the panic of its nesting counter, since TinyGo's wasm targets cannot
// recover from a panic:
//
//   - The nesting of (, [, and { counted together must not exceed
//     MaxBracketDepth.
//   - The else-if links, summed over the levels of the current nesting
//     path, must not exceed MaxElseIfChain.
//   - The tokens that the parser handles by recursion, summed the same
//     way, must not exceed MaxASTDepth: prefix operators such as !x, *p,
//     and <-ch, the type constructors [], *, map, chan, and func, and
//     labels. Each of them adds a level to the syntax tree, so the depth
//     check after parsing would reject the same files. The exceptions need
//     hundreds of repeats of unusual code, such as a product
//     m[i][j] * m[i][j] * ... whose * reads as a pointer type.
//   - All operators, opening brackets, and the keywords func and chan,
//     summed the same way, must not exceed maxOps. The parser handles
//     chains of binary operators, selectors, calls, and index expressions
//     with loops, so only its nesting counter grows; the depth check after
//     parsing applies MaxASTDepth to them.
//
// Both counts restart at a comma, semicolon, else, case, or default. A
// binary operator also restarts the recursion count of its level, keeping
// the labels, because the parser has returned from the left operand.
//
// The counts follow the parser only for code without syntax errors. The
// parser recovers from an error by taking any token in place of an
// expected bracket or by skipping tokens, and then it can nest deeper than
// the tokens show.
//
// A scanner error ends the scan with a parse-error skip, since the parser
// would report the same error.
func scanFile(src []byte, opt Options) (*scanInfo, *skip) {
	info := newScanInfo(src)
	fset := token.NewFileSet()
	tf := fset.AddFile("", -1, len(src))
	var bad *skip
	var s scanner.Scanner
	s.Init(tf, src, func(pos token.Position, msg string) {
		if bad == nil {
			line, col := info.lineCol(pos.Offset)
			bad = &skip{reason: result.SkipParseError, line: line, column: col, msg: msg}
		}
	}, scanner.ScanComments)

	tooDeep := func(off int, format string, args ...any) *skip {
		line, col := info.lineCol(off)
		return &skip{reason: result.SkipTooDeep, line: line, column: col, msg: fmt.Sprintf(format, args...)}
	}

	levels := []level{{}}
	ops, nest, elseIfs := 0, 0, 0
	restart := func(l *level) {
		ops -= l.ops
		nest -= l.nest
		l.ops, l.nest, l.labels = 0, 0, 0
	}
	count := func(l *level, recursive bool) {
		l.ops++
		ops++
		if recursive {
			l.nest++
			nest++
		}
	}
	// prev is the previous token other than a comment; prevType is set
	// when prev closes a level whose typ is set.
	prev, prevType := token.ILLEGAL, false
	line := 1
	for {
		pos, tok, lit := s.Scan()
		if bad != nil {
			return nil, bad
		}
		if tok == token.EOF {
			break
		}
		top := &levels[len(levels)-1]
		switch tok {
		case token.COMMENT:
			continue
		case token.COMMA, token.SEMICOLON:
			restart(top)
			prev, prevType = tok, false
			continue
		}

		off := int(pos) - tf.Base()
		end := tokenEnd(src, off, tok, lit)
		for line < info.lines() && info.lineStart[line] <= off {
			line++
		}
		last := line
		if tok == token.STRING && lit[0] == '`' {
			last = info.lineOf(end - 1)
		}
		info.mark(off, end, line, last)

		closedType := false
		switch tok {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			typ := false
			switch tok {
			case token.LPAREN:
				typ = prev == token.FUNC
			case token.LBRACK:
				// An index expression and an array type look alike after
				// "]", so a "[" there counts as a type.
				typ = prev == token.RBRACK || !endsOperand(prev, prevType, tok)
			}
			count(top, typ && tok == token.LBRACK)
			levels = append(levels, level{typ: typ})
			if depth := len(levels) - 1; depth > opt.MaxBracketDepth {
				return nil, tooDeep(off, "brackets are nested %d deep, more than the limit of %d", depth, opt.MaxBracketDepth)
			}
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if len(levels) > 1 {
				ops -= top.ops
				nest -= top.nest
				elseIfs -= top.elseIf
				closedType = top.typ
				levels = levels[:len(levels)-1]
			}
		case token.IF:
			if prev != token.ELSE {
				elseIfs -= top.elseIf
				top.elseIf = 0
				break
			}
			top.elseIf++
			elseIfs++
			if elseIfs > opt.MaxElseIfChain {
				return nil, tooDeep(off, "else-if chains are %d long, more than the limit of %d", elseIfs, opt.MaxElseIfChain)
			}
		case token.ELSE, token.CASE, token.DEFAULT:
			// The else-if limit covers the links of an if statement, and a
			// case clause starts a new statement list.
			restart(top)
		case token.FUNC, token.CHAN:
			count(top, true)
		case token.COLON:
			count(top, true)
			top.labels = top.nest
		default:
			if !tok.IsOperator() {
				break
			}
			switch {
			case tok == token.PERIOD || tok == token.ELLIPSIS:
				count(top, false)
			case endsOperand(prev, prevType, tok):
				count(top, false)
				nest -= top.nest - top.labels
				top.nest = top.labels
			default:
				// A prefix operator, except the arrow of "chan<-", which
				// belongs to the channel type that chan counts.
				count(top, tok != token.ARROW || prev != token.CHAN)
			}
		}
		if nest > opt.MaxASTDepth {
			return nil, tooDeep(off, "operators, types, and labels nest %d levels deep, more than the limit of %d", nest, opt.MaxASTDepth)
		}
		if ops > maxOps {
			return nil, tooDeep(off, "an expression chains %d operators, more than the limit of %d", ops, maxOps)
		}
		prev, prevType = tok, closedType
	}
	return info, nil
}

// endsOperand reports whether prev, the token before the operator or "["
// tok, ends an operand, which makes tok a binary operator or an index.
// prevType is set when prev closes a type part, such as the "]" of []T or
// the ")" of func(); only "*", "<-", and "[" continue a type after it.
func endsOperand(prev token.Token, prevType bool, tok token.Token) bool {
	switch prev {
	case token.IDENT, token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING, token.RBRACE:
		return true
	case token.RPAREN, token.RBRACK:
		return !prevType || (tok != token.MUL && tok != token.ARROW && tok != token.LBRACK)
	}
	return false
}
