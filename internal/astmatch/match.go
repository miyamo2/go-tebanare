package astmatch

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// MatchType reports whether the source type expression src matches the
// pattern pat. A nil env behaves like an empty Env.
//
// Parentheses are ignored on both sides, and the identifier any matches an
// empty interface{} in either direction. Apart from that, both sides must
// have the same shape:
//
//   - Any matches every expression except a variadic "...T".
//   - A TParam reference matches only the source type parameter bound to it.
//   - A plain identifier matches the same identifier, unless the source
//     identifier is a type parameter.
//   - A qualified identifier p.N matches the same spelling. Env.Qualifiers
//     can add an alternative qualifier.
//   - An array length is compared as text printed by go/printer. Any as
//     the length matches every array length, but not a slice.
//   - Channel directions must be equal.
//   - Interface elements and struct fields are compared in order. Struct
//     field names and tags must be equal as written.
//   - Instance type arguments are matched with MatchExprs. A pattern
//     instance also matches a source type without type arguments when its
//     arguments match an empty list, as in "List[...]", but only when that
//     source type is a type name (an identifier that is not a type
//     parameter, or a qualified identifier), so "_[...]" does not match
//     "[]int" or "*List".
//   - "~T" and "A | B" match the same operator with matching operands.
//
// Every other expression kind matches nothing, and so does a malformed tree:
// a nil node stored in an interface (such as (*ast.Ident)(nil)), a
// selector without a name, or a nil field or field name in a list. Such
// trees report false instead of panicking, since engine.wasm cannot
// recover from a panic.
func MatchType(pat, src ast.Expr, env *Env) bool {
	if env == nil {
		env = &Env{}
	}
	return matchType(pat, src, env)
}

// MatchFields reports whether the source parameter or result list src
// matches the pattern list pat. A nil env behaves like an empty Env.
//
// Both lists are expanded first, so "a, b int" is two elements. Names are
// ignored. A pattern element whose type is Seq matches zero or more source
// elements, including a variadic one. Every other pattern element matches
// exactly one source element with MatchType, so Any never matches a
// variadic element and "...T" matches only a variadic one. A nil list and
// an empty list both have no elements. A list with a nil field, a nil
// field name, or a field without a type matches nothing.
func MatchFields(pat, src *ast.FieldList, env *Env) bool {
	if env == nil {
		env = &Env{}
	}
	return matchFields(pat, src, env)
}

// MatchExprs reports whether the source list src, such as type arguments,
// matches the pattern list pat. Seq elements match zero or more items and
// every other element matches one item with MatchType. A nil env behaves
// like an empty Env.
func MatchExprs(pat, src []ast.Expr, env *Env) bool {
	if env == nil {
		env = &Env{}
	}
	return matchExprs(pat, src, env)
}

func matchFields(pat, src *ast.FieldList, env *Env) bool {
	if !validFields(pat, true) || !validFields(src, true) {
		return false
	}
	return matchExprs(expandFields(pat), expandFields(src), env)
}

func matchExprs(pat, src []ast.Expr, env *Env) bool {
	return matchList(pat, src, env, func(p, s ast.Expr) bool { return matchType(p, s, env) })
}

func matchType(pat, src ast.Expr, env *Env) bool {
	pat, src = unparen(pat), unparen(src)
	if isNil(pat) || isNil(src) {
		// A typed nil such as (*ast.Ident)(nil) matches nothing, not even
		// another nil.
		return pat == nil && src == nil
	}
	switch p := pat.(type) {
	case *ast.Ident:
		return matchIdent(p, src, env)
	case *ast.SelectorExpr:
		s, ok := src.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		px, ok1 := p.X.(*ast.Ident)
		sx, ok2 := s.X.(*ast.Ident)
		if !ok1 || !ok2 || px == nil || sx == nil || p.Sel == nil || s.Sel == nil {
			return false
		}
		return p.Sel.Name == s.Sel.Name && env.sameQualifier(px.Name, sx.Name)
	case *ast.StarExpr:
		s, ok := src.(*ast.StarExpr)
		return ok && matchType(p.X, s.X, env)
	case *ast.ArrayType:
		s, ok := src.(*ast.ArrayType)
		return ok && matchLen(p.Len, s.Len) && matchType(p.Elt, s.Elt, env)
	case *ast.MapType:
		s, ok := src.(*ast.MapType)
		return ok && matchType(p.Key, s.Key, env) && matchType(p.Value, s.Value, env)
	case *ast.ChanType:
		s, ok := src.(*ast.ChanType)
		return ok && p.Dir == s.Dir && matchType(p.Value, s.Value, env)
	case *ast.FuncType:
		s, ok := src.(*ast.FuncType)
		return ok && matchFields(p.Params, s.Params, env) && matchFields(p.Results, s.Results, env)
	case *ast.InterfaceType:
		switch s := src.(type) {
		case *ast.InterfaceType:
			return matchInterface(p, s, env)
		case *ast.Ident:
			return s.Name == "any" && !env.IsTypeParam("any") && isEmptyInterface(p)
		}
		return false
	case *ast.StructType:
		s, ok := src.(*ast.StructType)
		return ok && matchStruct(p, s, env)
	case *ast.IndexExpr, *ast.IndexListExpr:
		pb, pi := splitIndex(pat)
		sb, si := splitIndex(src)
		if len(si) == 0 && !isTypeName(sb, env) {
			// Only a type name can omit its type arguments.
			return false
		}
		return matchType(pb, sb, env) && matchExprs(pi, si, env)
	case *ast.UnaryExpr:
		s, ok := src.(*ast.UnaryExpr)
		return ok && p.Op == token.TILDE && s.Op == token.TILDE && matchType(p.X, s.X, env)
	case *ast.BinaryExpr:
		s, ok := src.(*ast.BinaryExpr)
		return ok && p.Op == token.OR && s.Op == token.OR &&
			matchType(p.X, s.X, env) && matchType(p.Y, s.Y, env)
	case *ast.Ellipsis:
		s, ok := src.(*ast.Ellipsis)
		return ok && matchType(p.Elt, s.Elt, env)
	}
	return false
}

func matchIdent(p *ast.Ident, src ast.Expr, env *Env) bool {
	switch p.Name {
	case AnyName:
		_, variadic := src.(*ast.Ellipsis)
		return !variadic
	case SeqName:
		return false
	}
	if name, ok := TParamName(p); ok {
		s, ok := src.(*ast.Ident)
		if !ok || !env.IsTypeParam(s.Name) {
			return false
		}
		bound, ok := env.Lookup(name)
		return ok && bound == s.Name
	}
	switch s := src.(type) {
	case *ast.Ident:
		return s.Name == p.Name && !env.IsTypeParam(s.Name)
	case *ast.InterfaceType:
		return p.Name == "any" && isEmptyInterface(s)
	}
	return false
}

// matchLen compares array lengths. A nil length is a slice.
func matchLen(p, s ast.Expr) bool {
	switch {
	case isNil(p) || isNil(s):
		return p == nil && s == nil
	case IsAny(p):
		return true
	}
	pt, ok1 := exprText(p)
	st, ok2 := exprText(s)
	return ok1 && ok2 && pt == st
}

func exprText(e ast.Expr) (string, bool) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), e); err != nil {
		return "", false
	}
	return buf.String(), true
}

func matchInterface(p, s *ast.InterfaceType, env *Env) bool {
	if !validFields(p.Methods, true) || !validFields(s.Methods, true) {
		return false
	}
	pl, sl := fieldList(p.Methods), fieldList(s.Methods)
	if len(pl) != len(sl) {
		return false
	}
	for i := range pl {
		if !sameNames(pl[i].Names, sl[i].Names) || !matchType(pl[i].Type, sl[i].Type, env) {
			return false
		}
	}
	return true
}

type structField struct {
	name string
	typ  ast.Expr
	tag  *ast.BasicLit
}

func matchStruct(p, s *ast.StructType, env *Env) bool {
	if !validFields(p.Fields, true) || !validFields(s.Fields, true) {
		return false
	}
	pl, sl := structFields(p), structFields(s)
	if len(pl) != len(sl) {
		return false
	}
	for i := range pl {
		pf, sf := pl[i], sl[i]
		if pf.name != sf.name || !sameTag(pf.tag, sf.tag) || !matchType(pf.typ, sf.typ, env) {
			return false
		}
	}
	return true
}

// structFields expands "a, b int" into one entry per name. An embedded
// field has an empty name.
func structFields(st *ast.StructType) []structField {
	var out []structField
	for _, f := range fieldList(st.Fields) {
		if len(f.Names) == 0 {
			out = append(out, structField{typ: f.Type, tag: f.Tag})
			continue
		}
		for _, n := range f.Names {
			out = append(out, structField{name: n.Name, typ: f.Type, tag: f.Tag})
		}
	}
	return out
}

func sameTag(a, b *ast.BasicLit) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Value == b.Value
}

func sameNames(a, b []*ast.Ident) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func isEmptyInterface(it *ast.InterfaceType) bool {
	return len(fieldList(it.Methods)) == 0
}

// isTypeName reports whether e, without parentheses, is an identifier
// that is not a source type parameter or a qualified identifier: the only
// source types that can be generic and appear without type arguments.
func isTypeName(e ast.Expr, env *Env) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		return x != nil && !env.IsTypeParam(x.Name)
	case *ast.SelectorExpr:
		return x != nil
	}
	return false
}

// validFields reports whether every field of fl and every field name is
// non-nil. When needType is true, every field must also have a type.
func validFields(fl *ast.FieldList, needType bool) bool {
	for _, f := range fieldList(fl) {
		if f == nil || (needType && f.Type == nil) {
			return false
		}
		for _, n := range f.Names {
			if n == nil {
				return false
			}
		}
	}
	return true
}

// isNil reports whether e is nil or a nil pointer of a node type that
// matchType or matchLen dereferences.
func isNil(e ast.Expr) bool {
	switch x := e.(type) {
	case nil:
		return true
	case *ast.Ident:
		return x == nil
	case *ast.SelectorExpr:
		return x == nil
	case *ast.StarExpr:
		return x == nil
	case *ast.ArrayType:
		return x == nil
	case *ast.MapType:
		return x == nil
	case *ast.ChanType:
		return x == nil
	case *ast.FuncType:
		return x == nil
	case *ast.InterfaceType:
		return x == nil
	case *ast.StructType:
		return x == nil
	case *ast.IndexExpr:
		return x == nil
	case *ast.IndexListExpr:
		return x == nil
	case *ast.UnaryExpr:
		return x == nil
	case *ast.BinaryExpr:
		return x == nil
	case *ast.Ellipsis:
		return x == nil
	case *ast.ParenExpr:
		return x == nil
	case *ast.BasicLit:
		return x == nil
	}
	return false
}

// splitIndex splits an instance into its base type and type arguments. Any
// other expression is returned as the base with no arguments.
func splitIndex(e ast.Expr) (ast.Expr, []ast.Expr) {
	switch x := e.(type) {
	case *ast.IndexExpr:
		return x.X, []ast.Expr{x.Index}
	case *ast.IndexListExpr:
		return x.X, x.Indices
	}
	return e, nil
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok || p == nil {
			return e
		}
		e = p.X
	}
}

func (e *Env) sameQualifier(pat, src string) bool {
	if pat == src {
		return true
	}
	q, ok := e.quals[src]
	return ok && q == pat
}
