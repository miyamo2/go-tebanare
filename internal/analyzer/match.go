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

	aliases     map[string]string
	aliasesDone bool
	// tooLarge counts, per rule index, the nodes a user rule could not
	// see because they exceed Options.MaxNodeSize.
	tooLarge map[int]*largeNodes
}

type largeNodes struct{ count, line int }

// run applies the rules that apply to the path: func rules to every
// top-level function declaration, stmt and expr rules to the nodes in
// their scope (plan 4.6).
func (a *analysis) run() {
	var funcRules, stmtRules, exprRules []int
	for i, r := range a.set.Rules {
		if r == nil || !r.AppliesTo(a.path) {
			continue
		}
		switch {
		case r.Target == result.TargetFunc && r.Func != nil:
			funcRules = append(funcRules, i)
		case r.Target == result.TargetStmt && r.Node != nil:
			stmtRules = append(stmtRules, i)
		case r.Target == result.TargetExpr && r.Node != nil:
			exprRules = append(exprRules, i)
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

	if len(stmtRules) == 0 && len(exprRules) == 0 {
		return
	}
	walk(a.file, func(n ast.Node, sc scope, bodyBlock bool, ancestors []ast.Node) bool {
		if isStmtCandidate(n, sc, bodyBlock) {
			for _, i := range stmtRules {
				a.matchNode(n, i, ancestors)
			}
		}
		if isExprCandidate(n, sc) {
			for _, i := range exprRules {
				a.matchNode(n, i, ancestors)
			}
		}
		return true
	})
	for i, r := range a.set.Rules {
		if l := a.tooLarge[i]; l != nil {
			a.diag(result.CodeNodeTooLarge, r.ID, l.line, fmt.Sprintf(
				"rule %q skipped %d nodes larger than %d bytes, the first on line %d", r.ID, l.count, a.opt.MaxNodeSize, l.line))
		}
	}
}

// matchFunc applies the func rules to fd.
func (a *analysis) matchFunc(fd *ast.FuncDecl, key string, rules []int) {
	for _, i := range rules {
		r := a.set.Rules[i]
		if !r.Func.MatchFunc(fd, a.rf) {
			a.explainMiss(fd, r)
			continue
		}
		from, to := funcRange(fd, r.IncludeDoc)
		hit := result.Hit{RuleID: r.ID, Target: r.Target, Node: "FuncDecl", Label: funcLabel(fd), Preset: r.Preset}
		s, ok := a.accept(i, from, to, false, hit)
		if !ok {
			s = span{start: a.rf.Line(from), end: a.rf.Line(to - 1), rule: i, hit: hit}
		}
		a.out.funcs = append(a.out.funcs, funcHit{key: key, shared: !ok, span: s})
	}
}

// explainMiss adds an alias-not-resolved diagnostic when r would match fd
// if the import aliases of the file named the guessed packages. The guess
// never decides what is hidden.
func (a *analysis) explainMiss(fd *ast.FuncDecl, r *rule.Rule) {
	relaxed, ok := r.Func.(rule.RelaxedFuncMatcher)
	if !ok {
		return
	}
	if !a.aliasesDone {
		a.aliases, a.aliasesDone = rule.ImportAliases(a.file), true
	}
	if len(a.aliases) == 0 || !relaxed.MatchFuncRelaxed(fd, a.rf, a.aliases) {
		return
	}
	a.diag(result.CodeAliasNotResolved, r.ID, a.rf.Line(fd.Pos()),
		fmt.Sprintf("%s would match rule %q if its import aliases were resolved; patterns compare package qualifiers as written", funcLabel(fd), r.ID))
}

// matchNode applies the stmt or expr rule with index i to n.
func (a *analysis) matchNode(n ast.Node, i int, ancestors []ast.Node) {
	r := a.set.Rules[i]
	if !r.Node.Accepts(n) {
		return
	}
	sp, ok := r.Node.MatchNode(n, a.rf)
	if !ok {
		// User rules match the normalized text, which large nodes lack.
		// Preset matchers work on the syntax tree.
		if _, fits := a.cache.Text(n); !fits && r.Preset == "" {
			a.noteTooLarge(i, n)
		}
		return
	}
	label := rule.KindOf(n)
	if text, ok := a.cache.Text(n); ok {
		label = nodeLabel(text)
	}
	hit := result.Hit{RuleID: r.ID, Target: r.Target, Node: rule.KindOf(n), Label: label, Preset: r.Preset}
	from, to := n.Pos(), n.End()
	switch {
	case !sp.IsZero():
		from, to = sp.From, sp.To
	case r.Target == result.TargetExpr && r.Hide == rule.HideStatement:
		s := statementFor(n.(ast.Expr), ancestors)
		if s == nil {
			a.diag(result.CodeStatementNotSimple, r.ID, a.rf.Line(n.Pos()),
				fmt.Sprintf("rule %q matched %s; hide: statement hides only a simple statement whose other parts are identifiers or literals", r.ID, label))
			return
		}
		from, to = s.Pos(), s.End()
	}
	if s, ok := a.accept(i, from, to, r.IncludeLeadingComments, hit); ok {
		a.out.nodes = append(a.out.nodes, s)
	}
}

// accept turns the source range [from, to) of a match into a span. With
// leading set, the range grows over the comment group directly above it
// (see leadingComment). The range must pass the line occupancy check of
// plan 4.7; otherwise accept adds a line-shared diagnostic and reports
// false.
func (a *analysis) accept(i int, from, to token.Pos, leading bool, hit result.Hit) (span, bool) {
	start, end, ok := offsets(a.fset, from, to)
	if !ok {
		return span{}, false
	}
	if leading {
		if g := leadingComment(a.rf, a.info, a.rf.Line(from)); g != nil {
			from = g.Pos()
			start = a.rf.Offset(from)
		}
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

func (a *analysis) noteTooLarge(i int, n ast.Node) {
	if a.tooLarge == nil {
		a.tooLarge = map[int]*largeNodes{}
	}
	l := a.tooLarge[i]
	if l == nil {
		l = &largeNodes{line: a.rf.Line(n.Pos())}
		a.tooLarge[i] = l
	}
	l.count++
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
