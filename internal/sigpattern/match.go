package sigpattern

import (
	"go/ast"

	"github.com/miyamo2/go-tebanare/internal/astmatch"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// Match reports whether fd matches the pattern. Package qualifiers match
// only when they are spelled the same way.
func (p *Pattern) Match(fd *ast.FuncDecl) bool {
	return p.match(fd, nil)
}

// MatchQualified is Match with a relaxed rule for package qualifiers: a
// source qualifier q also matches the pattern qualifier qualifiers[q]. The
// analyzer passes the guesses from rule.ImportAliases to explain misses
// caused by import aliases. A relaxed match must never decide what is
// hidden.
func (p *Pattern) MatchQualified(fd *ast.FuncDecl, qualifiers map[string]string) bool {
	return p.match(fd, qualifiers)
}

// match checks the receiver, then the name, then the type parameters, and
// then the signature.
func (p *Pattern) match(fd *ast.FuncDecl, quals map[string]string) bool {
	if p == nil || fd == nil || fd.Name == nil || fd.Type == nil {
		return false
	}
	if !p.matchRecv(fd) || !p.name.match(fd.Name.Name) {
		return false
	}
	recvParams := rule.RecvTypeParams(fd)
	srcParams := append([]*ast.Ident(nil), recvParams...)
	if fd.Type.TypeParams != nil {
		for _, f := range fd.Type.TypeParams.List {
			srcParams = append(srcParams, f.Names...)
		}
	}
	env := astmatch.NewEnv(srcParams)
	env.Qualifiers(quals)
	sig := func() bool {
		return p.sig == nil ||
			astmatch.MatchFields(p.sig.Params, fd.Type.Params, env) &&
				astmatch.MatchFields(p.sig.Results, fd.Type.Results, env)
	}
	switch {
	case p.recv != nil && p.recv.args != nil:
		src := &ast.FieldList{}
		for _, id := range recvParams {
			src.List = append(src.List, &ast.Field{Names: []*ast.Ident{id}})
		}
		return astmatch.MatchTypeParamsThen(p.recv.args, src, env, sig)
	case p.tparams != nil:
		return astmatch.MatchTypeParamsThen(p.tparams, fd.Type.TypeParams, env, sig)
	}
	return sig()
}

func (p *Pattern) matchRecv(fd *ast.FuncDecl) bool {
	r := p.recv
	if r == nil {
		return fd.Recv == nil
	}
	base, pointer, ok := rule.RecvBase(fd)
	switch {
	case !ok:
		return false
	case r.anyType:
		return pointer || !r.pointer
	}
	return pointer == r.pointer && r.name.match(base)
}
