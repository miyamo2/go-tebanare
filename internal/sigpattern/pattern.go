package sigpattern

import "go/ast"

// Pattern is a parsed signature pattern.
type Pattern struct {
	recv    *receiver      // nil: plain functions only
	name    name           // function name
	tparams *ast.FieldList // nil: any type parameters
	sig     *ast.FuncType  // nil: any parameters and results
}

type receiver struct {
	// anyType is set by "_". Without pointer it accepts value and
	// pointer receivers.
	anyType bool
	pointer bool
	name    name           // base type name when !anyType
	args    *ast.FieldList // nil: any type arguments
}

// NameRegexps returns every regular expression used as a name, receiver
// first, with "\/" unescaped. Callers use it to warn about expressions
// that are not anchored.
func (p *Pattern) NameRegexps() []string {
	var out []string
	if p.recv != nil && p.recv.name.regexp {
		out = append(out, p.recv.name.text)
	}
	if p.name.regexp {
		out = append(out, p.name.text)
	}
	return out
}
