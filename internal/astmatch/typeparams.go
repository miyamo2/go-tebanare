package astmatch

import "go/ast"

// maxAlignments caps the number of complete alignments that
// MatchTypeParamsThen tries. A pattern with several Seq elements against a
// long source list could otherwise take exponential time. Reaching the cap
// reports no match.
const maxAlignments = 4096

// MatchTypeParams reports whether the source type parameter list src
// matches the pattern list pat. On success the pattern names stay bound
// in env. It is MatchTypeParamsThen with a nil continuation.
func MatchTypeParams(pat, src *ast.FieldList, env *Env) bool {
	return MatchTypeParamsThen(pat, src, env, nil)
}

// MatchTypeParamsThen matches type parameter lists and then runs a
// continuation. A nil env behaves like an empty Env.
//
// Both lists are expanded, so "[K, V any]" is two elements. Each pattern
// field names one element:
//
//   - SeqName matches zero or more source type parameters.
//   - AnyName matches one source type parameter.
//   - Any other name N matches one source type parameter S, and
//     env.Bind(N, S) must succeed.
//
// A pattern element with a nil Type accepts any constraint. Otherwise its
// Type must match the source constraint with MatchType.
//
// For each alignment of pat against src, in order, all names are bound
// first and the constraints are compared afterwards, so a constraint can
// refer to a parameter declared later ("[S ~[]E, E any]"). When the
// constraints match, then is called with the bindings in place. The first
// alignment for which then reports true (a nil then counts as true) wins
// and its bindings stay in env. Otherwise env is restored and the result is
// false, also after maxAlignments alignments.
func MatchTypeParamsThen(pat, src *ast.FieldList, env *Env, then func() bool) bool {
	if env == nil {
		env = &Env{}
	}
	m := &tpMatch{
		pat:    expandTypeParams(pat),
		src:    expandTypeParams(src),
		env:    env,
		then:   then,
		budget: maxAlignments,
	}
	m.at = make([]int, len(m.pat))
	m.need = make([]int, len(m.pat)+1)
	m.seq = make([]bool, len(m.pat)+1)
	for i := len(m.pat) - 1; i >= 0; i-- {
		m.need[i], m.seq[i] = m.need[i+1], m.seq[i+1]
		if m.pat[i].name == SeqName {
			m.seq[i] = true
		} else {
			m.need[i]++
		}
	}
	return m.align(0, 0)
}

type tpElem struct {
	name string
	typ  ast.Expr
}

type tpMatch struct {
	pat, src []tpElem
	env      *Env
	then     func() bool
	budget   int
	at       []int  // at[i] is the source index aligned with pat[i]
	need     []int  // need[i] counts the non-Seq elements in pat[i:]
	seq      []bool // seq[i] reports whether pat[i:] contains a Seq
}

func expandTypeParams(fl *ast.FieldList) []tpElem {
	var out []tpElem
	for _, f := range fieldList(fl) {
		if len(f.Names) == 0 {
			out = append(out, tpElem{typ: f.Type})
			continue
		}
		for _, n := range f.Names {
			out = append(out, tpElem{name: n.Name, typ: f.Type})
		}
	}
	return out
}

func (m *tpMatch) align(i, j int) bool {
	rest := len(m.src) - j
	if m.budget <= 0 || rest < m.need[i] || (!m.seq[i] && rest != m.need[i]) {
		return false
	}
	if i == len(m.pat) {
		return m.complete()
	}
	if m.pat[i].name != SeqName {
		m.at[i] = j
		return m.align(i+1, j+1)
	}
	for k := j; k <= len(m.src); k++ {
		if m.align(i+1, k) {
			return true
		}
	}
	return false
}

// complete checks one full alignment.
func (m *tpMatch) complete() bool {
	m.budget--
	snap := m.env.Snapshot()
	ok := true
	for i, p := range m.pat {
		if p.name != SeqName && p.name != AnyName && p.name != "" {
			ok = ok && m.env.Bind(p.name, m.src[m.at[i]].name)
		}
	}
	for i, p := range m.pat {
		if p.name != SeqName && p.typ != nil {
			ok = ok && matchType(p.typ, m.src[m.at[i]].typ, m.env)
		}
	}
	if ok && (m.then == nil || m.then()) {
		return true
	}
	m.env.Restore(snap)
	return false
}
