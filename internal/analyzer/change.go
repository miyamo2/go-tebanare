package analyzer

import (
	"fmt"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// Side is one version of a changed file.
type Side struct {
	// Path is the slash-separated path relative to the repository root.
	Path string
	// Src is the content of the file. It is nil when the file does not
	// exist on this side: the old side of an added file or the new side
	// of a deleted file.
	Src []byte
}

// AnalyzeChange analyzes both sides of a changed file (plan 4.8).
//
// When a present side is not a target of the set, or when either side is
// skipped, nothing is hidden on either side. Stmt matches count on each
// side on its own. Func matches are paired by rule and
// declaration key: a declaration is hidden when it matches on both sides
// or when the other side does not declare its key. When one side matches
// and the other declares the key without matching (for example because
// the rule's paths exclude that side), neither side hides it and a
// match-changed diagnostic reports the change. A match whose lines hold
// other code counts as a match for the pairing, but neither it nor its
// pair on the other side is hidden. Keys declared more than once on a side
// are never hidden.
func AnalyzeChange(set *rule.Set, oldSide, newSide Side, opt Options) result.ChangeResult {
	res := analyzeChange(set, [2]Side{oldSide, newSide}, opt)
	res.Normalize()
	return res
}

var sideNames = [2]string{result.SideOld, result.SideNew}

func analyzeChange(set *rule.Set, sides [2]Side, opt Options) result.ChangeResult {
	var res result.ChangeResult
	if set == nil {
		set = &rule.Set{}
	}
	present := [2]bool{sides[0].Src != nil, sides[1].Src != nil}
	targets, other := 0, -1
	for i, s := range sides {
		switch {
		case !present[i]:
		case set.IsTarget(s.Path):
			targets++
		case other < 0:
			other = i
		}
	}
	if other >= 0 {
		res.Skipped = result.SkipNotTarget
		// A diagnostic only helps when the other side is a target, as
		// in a rename across the file filters.
		if targets > 0 {
			res.Diagnostics = append(res.Diagnostics, sideSkipped(notTarget(sides[other].Path), other))
		}
		return res
	}

	var fa [2]*fileAnalysis
	for i, s := range sides {
		if !present[i] {
			continue
		}
		fa[i] = analyzeFile(set, s.Path, s.Src, opt)
		if sk := fa[i].skip; sk != nil {
			res.Skipped = sk.reason
			res.Diagnostics = append(res.Diagnostics, sideSkipped(sk, i))
			return res
		}
	}

	for i := range fa {
		if fa[i] == nil {
			continue
		}
		for _, d := range fa[i].diags {
			d.Side = sideNames[i]
			res.Diagnostics = append(res.Diagnostics, d)
		}
	}
	hidden, diags := pairFuncs(set, fa)
	res.Diagnostics = append(res.Diagnostics, diags...)
	for i := range fa {
		if fa[i] == nil {
			continue
		}
		spans := append(fa[i].nodes[:len(fa[i].nodes):len(fa[i].nodes)], hidden[i]...)
		if i == 0 {
			res.Old = mergeSpans(spans, fa[i].blank)
		} else {
			res.New = mergeSpans(spans, fa[i].blank)
		}
	}
	return res
}

func sideSkipped(sk *skip, side int) result.Diagnostic {
	d := skipDiagnostic(sk)
	d.Side = sideNames[side]
	d.Message = sideNames[side] + " side " + d.Message
	return d
}

// pairFuncs decides which func matches to hide on each side (plan 4.8).
// A nil entry of fa is an absent side, which declares nothing.
func pairFuncs(set *rule.Set, fa [2]*fileAnalysis) (hidden [2][]span, diags []result.Diagnostic) {
	declared := func(side int, key string) int {
		if fa[side] == nil {
			return 0
		}
		return fa[side].decls[key]
	}
	for ri, r := range set.Rules {
		if r == nil || r.Target != result.TargetFunc {
			continue
		}
		var matches [2]map[string]funcHit
		var keys []string
		for side := range fa {
			if fa[side] == nil {
				continue
			}
			for _, f := range fa[side].funcs {
				if f.rule != ri {
					continue
				}
				if matches[side] == nil {
					matches[side] = map[string]funcHit{}
				}
				if _, ok := matches[0][f.key]; !ok {
					if _, ok := matches[1][f.key]; !ok {
						keys = append(keys, f.key)
					}
				}
				matches[side][f.key] = f
			}
		}
		for _, k := range keys {
			if declared(0, k) > 1 || declared(1, k) > 1 {
				continue
			}
			m0, ok0 := matches[0][k]
			m1, ok1 := matches[1][k]
			switch {
			case ok0 && ok1:
				// When a side's match shares its lines, the line-shared
				// diagnostic explains why both sides stay visible.
				if !m0.shared && !m1.shared {
					hidden[0] = append(hidden[0], m0.span)
					hidden[1] = append(hidden[1], m1.span)
				}
			case ok0 && declared(1, k) > 0:
				diags = append(diags, matchChanged(r, k, m0, 0))
			case ok1 && declared(0, k) > 0:
				diags = append(diags, matchChanged(r, k, m1, 1))
			case ok0 && !m0.shared:
				hidden[0] = append(hidden[0], m0.span)
			case ok1 && !m1.shared:
				hidden[1] = append(hidden[1], m1.span)
			}
		}
	}
	return hidden, diags
}

func matchChanged(r *rule.Rule, key string, m funcHit, side int) result.Diagnostic {
	return result.Diagnostic{
		Severity: result.SeverityInfo,
		Code:     result.CodeMatchChanged,
		Message: fmt.Sprintf("rule %q matches %s only on the %s side, so both sides stay visible",
			r.ID, key, sideNames[side]),
		Side:   sideNames[side],
		RuleID: r.ID,
		Line:   m.start,
	}
}
