package astmatch

import "go/ast"

// Env is the state of one match: the identifiers that are type parameters
// on the source side, the bindings from pattern type parameters to them,
// and an optional qualifier rewrite for relaxed matching.
//
// The zero value is an Env without source type parameters.
type Env struct {
	tparams map[string]bool
	binds   []binding
	quals   map[string]string
}

type binding struct{ pat, src string }

// NewEnv returns an Env whose source type parameters are srcTypeParams.
// The blank identifier "_" is never recorded, because source code cannot
// refer to a type parameter named "_".
func NewEnv(srcTypeParams []*ast.Ident) *Env {
	e := &Env{}
	for _, id := range srcTypeParams {
		if id == nil || id.Name == "_" {
			continue
		}
		if e.tparams == nil {
			e.tparams = map[string]bool{}
		}
		e.tparams[id.Name] = true
	}
	return e
}

// IsTypeParam reports whether name is a source type parameter.
func (e *Env) IsTypeParam(name string) bool { return e.tparams[name] }

// Bind binds the pattern type parameter pattern to the source type
// parameter source. It reports false when source is not a source type
// parameter, when pattern is already bound to a different name, or when
// source is already bound to a different pattern name. Binding to "_"
// always succeeds and never conflicts; such a binding matches nothing,
// since source code cannot refer to "_".
func (e *Env) Bind(pattern, source string) bool {
	if source != "_" && !e.tparams[source] {
		return false
	}
	for _, b := range e.binds {
		if b.pat == pattern {
			return b.src == source
		}
		if b.src == source && source != "_" {
			return false
		}
	}
	e.binds = append(e.binds, binding{pat: pattern, src: source})
	return true
}

// Lookup returns the source type parameter bound to pattern.
func (e *Env) Lookup(pattern string) (source string, ok bool) {
	for _, b := range e.binds {
		if b.pat == pattern {
			return b.src, true
		}
	}
	return "", false
}

// Snapshot returns a token for Restore.
func (e *Env) Snapshot() int { return len(e.binds) }

// Restore removes the bindings made after the Snapshot call that returned
// snap.
func (e *Env) Restore(snap int) {
	if snap >= 0 && snap < len(e.binds) {
		e.binds = e.binds[:snap]
	}
}

// Qualifiers sets a rewrite for package qualifiers on the source side. A
// source qualifier q then also matches the pattern qualifier m[q]. The
// analyzer uses it only to explain misses caused by import aliases. A nil
// map turns the rewrite off.
func (e *Env) Qualifiers(m map[string]string) { e.quals = m }
