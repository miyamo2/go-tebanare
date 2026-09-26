package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// analysis runs the rules of a set against one parsed file.
type analysis struct {
	*parsed
	set *rule.Set
	opt Options
	out *fileAnalysis
}

// run applies the rules that apply to the path: func rules to every
// top-level function declaration and stmt rules to the statements in
// their scope (plan 4.6).
func (a *analysis) run() {
	var funcRules, stmtRules []int
	for i, r := range a.set.Rules {
		if r == nil || !r.AppliesTo(a.path) {
			continue
		}
		switch {
		case r.Target == result.TargetFunc && r.Func != nil:
			funcRules = append(funcRules, i)
		case r.Target == result.TargetStmt && r.Node != nil:
			stmtRules = append(stmtRules, i)
		}
	}

	keys := pairingKeys(a.file)
	var decls []*ast.FuncDecl
	for _, d := range a.file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			decls = append(decls, fd)
			a.out.decls[keys[fd]]++
		}
	}
	reported := map[string]bool{}
	for _, fd := range decls {
		k := keys[fd]
		if n := a.out.decls[k]; n > 1 && !reported[k] {
			reported[k] = true
			a.diag(result.CodeDuplicateDecl, "", a.rf.Line(fd.Pos()),
				fmt.Sprintf("%s is declared %d times; func rules hide none of these declarations", k, n))
		}
		a.matchFunc(fd, k, funcRules)
	}

	if len(stmtRules) == 0 {
		return
	}
	walk(a.file, func(n ast.Node, sc scope, bodyBlock bool) bool {
		if isStmtCandidate(n, sc, bodyBlock) {
			for _, i := range stmtRules {
				a.matchNode(n, i)
			}
		}
		return true
	})
}

// matchFunc applies the func rules to fd.
func (a *analysis) matchFunc(fd *ast.FuncDecl, key string, rules []int) {
	for _, i := range rules {
		r := a.set.Rules[i]
		if !r.Func.MatchFunc(fd, a.rf) {
			continue
		}
		from, to := funcRange(fd, r.IncludeDoc)
		hit := result.Hit{RuleID: r.ID, Target: r.Target, Node: "FuncDecl", Label: funcLabel(fd), Preset: r.Preset}
		s, ok := a.accept(i, from, to, hit)
		if !ok {
			s = span{start: a.rf.Line(from), end: a.rf.Line(to - 1), rule: i, hit: hit}
		}
		a.out.funcs = append(a.out.funcs, funcHit{key: key, shared: !ok, span: s})
	}
}

// matchNode applies the stmt rule with index i to n.
func (a *analysis) matchNode(n ast.Node, i int) {
	r := a.set.Rules[i]
	if !r.Node.Accepts(n) {
		return
	}
	sp, ok := r.Node.MatchNode(n, a.rf)
	if !ok {
		return
	}
	label := rule.KindOf(n)
	if text, ok := a.cache.Text(n); ok {
		label = nodeLabel(text)
	}
	hit := result.Hit{RuleID: r.ID, Target: r.Target, Node: rule.KindOf(n), Label: label, Preset: r.Preset}
	from, to := n.Pos(), n.End()
	if !sp.IsZero() {
		from, to = sp.From, sp.To
	}
	if s, ok := a.accept(i, from, to, hit); ok {
		a.out.nodes = append(a.out.nodes, s)
	}
}

// accept turns the source range [from, to) of a match into a span. The
// range must pass the line occupancy check of plan 4.7; otherwise accept
// adds a line-shared diagnostic and reports false.
func (a *analysis) accept(i int, from, to token.Pos, hit result.Hit) (span, bool) {
	start, end, ok := offsets(a.fset, from, to)
	if !ok {
		return span{}, false
	}
	if ok, line := a.info.occupies(start, end); !ok {
		a.diag(result.CodeLineShared, hit.RuleID, line,
			fmt.Sprintf("rule %q matched %s; line %d also holds other code, so it stays visible", hit.RuleID, hit.Label, line))
		return span{}, false
	}
	return span{
		start: a.rf.Line(from), end: a.rf.Line(to - 1),
		off: start, endOff: end, rule: i, hit: hit,
	}, true
}

func (a *analysis) diag(code, ruleID string, line int, msg string) {
	a.out.diags = append(a.out.diags, result.Diagnostic{
		Severity: result.SeverityInfo,
		Code:     code,
		Message:  msg,
		RuleID:   ruleID,
		Line:     line,
	})
}
