package astmatch

import "go/ast"

// matchList reports whether the list src matches the pattern list pat. A
// Seq element of pat matches zero or more elements of src, and any other
// element matches one element for which eq reports true.
//
// matchList uses the greedy glob algorithm: on a mismatch it retries from
// the most recent Seq, which then absorbs one more source element. The
// result is exact as long as eq does not depend on how earlier elements
// were aligned (eq must not add bindings), and the work is bounded by
// len(pat)*len(src) calls to eq.
func matchList(pat, src []ast.Expr, env *Env, eq func(p, s ast.Expr) bool) bool {
	i, j := 0, 0
	star, starJ, snap := -1, 0, 0
	for j < len(src) {
		switch {
		case i < len(pat) && IsSeq(pat[i]):
			star, starJ, snap = i, j, env.Snapshot()
			i++
		case i < len(pat) && eq(pat[i], src[j]):
			i++
			j++
		case star >= 0:
			env.Restore(snap)
			starJ++
			i, j = star+1, starJ
		default:
			return false
		}
	}
	for i < len(pat) && IsSeq(pat[i]) {
		i++
	}
	return i == len(pat)
}

// expandFields returns the type of each element of fl, so "a, b int"
// gives two elements.
func expandFields(fl *ast.FieldList) []ast.Expr {
	var out []ast.Expr
	for _, f := range fieldList(fl) {
		for range max(len(f.Names), 1) {
			out = append(out, f.Type)
		}
	}
	return out
}

func fieldList(fl *ast.FieldList) []*ast.Field {
	if fl == nil {
		return nil
	}
	return fl.List
}
