package analyzer

import (
	"fmt"
	"go/ast"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// NodeInfo describes one node for Explain.
type NodeInfo struct {
	// Kind is the go/ast type name, such as "CallExpr".
	Kind    string `json:"kind"`
	Line    int    `json:"line"`
	EndLine int    `json:"endLine"`
	// Text is the normalized text, or "" when the node is larger than
	// Options.MaxNodeSize.
	Text string `json:"text"`
	// InScope reports whether stmt and expr rules consider the node: the
	// statements and expressions in their scope (plan 4.6). It is false
	// for a FuncDecl, which only func rules consider; Rules lists the func
	// rules that match it.
	InScope bool `json:"inScope"`
	// Rules lists the ids of the rules that apply to the path and whose
	// matcher matches the node, ignoring scope, the occupancy check, and
	// pairing.
	Rules []string `json:"rules"`
}

// SkipError is the error Explain returns for a file it does not analyze.
type SkipError struct {
	Reason result.SkipReason
	// Line and Column locate the problem. They are 0 when unknown.
	Line, Column int
	Msg          string
}

// Error returns "<reason>: <line>:<column>: <message>", leaving out the
// position when it is unknown.
func (e *SkipError) Error() string {
	switch {
	case e.Line > 0 && e.Column > 0:
		return fmt.Sprintf("%s: %d:%d: %s", e.Reason, e.Line, e.Column, e.Msg)
	case e.Line > 0:
		return fmt.Sprintf("%s: %d: %s", e.Reason, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Reason, e.Msg)
}

// Explain returns every FuncDecl, statement, and expression that starts
// on line, outermost first, to help write rules. It returns a *SkipError
// when the file is not a target or fails the checks of AnalyzeFile.
func Explain(set *rule.Set, path string, src []byte, line int, opt Options) ([]NodeInfo, error) {
	if set == nil {
		set = &rule.Set{}
	}
	opt = opt.withDefaults()
	var p *parsed
	var sk *skip
	if set.IsTarget(path) {
		p, sk = prepare(path, src, opt)
	} else {
		sk = notTarget(path)
	}
	if sk != nil {
		return nil, &SkipError{Reason: sk.reason, Line: sk.line, Column: sk.column, Msg: sk.msg}
	}

	var rules []*rule.Rule
	for _, r := range set.Rules {
		if r != nil && r.AppliesTo(path) {
			rules = append(rules, r)
		}
	}
	out := []NodeInfo{}
	walk(p.file, func(n ast.Node, sc scope, bodyBlock bool, _ []ast.Node) bool {
		start, end := p.rf.Line(n.Pos()), p.rf.Line(n.End()-1)
		if end < line || start > line {
			return false
		}
		if start != line {
			return true
		}
		info := NodeInfo{Kind: rule.KindOf(n), Line: start, EndLine: end, Rules: []string{}}
		switch n := n.(type) {
		case *ast.FuncDecl:
			for _, r := range rules {
				if r.Target == result.TargetFunc && r.Func != nil && r.Func.MatchFunc(n, p.rf) {
					info.Rules = append(info.Rules, r.ID)
				}
			}
		case ast.Stmt:
			info.InScope = isStmtCandidate(n, sc, bodyBlock)
			info.Rules = matchingNodeRules(rules, result.TargetStmt, n, p.rf, info.Rules)
		case ast.Expr:
			info.InScope = isExprCandidate(n, sc)
			info.Rules = matchingNodeRules(rules, result.TargetExpr, n, p.rf, info.Rules)
		default:
			return true
		}
		info.Text, _ = p.cache.Text(n)
		out = append(out, info)
		return true
	})
	return out, nil
}

// matchingNodeRules appends to ids the ids of the rules with the given
// target that match n.
func matchingNodeRules(rules []*rule.Rule, target result.Target, n ast.Node, f *rule.File, ids []string) []string {
	for _, r := range rules {
		if r.Target != target || r.Node == nil || !r.Node.Accepts(n) {
			continue
		}
		if _, ok := r.Node.MatchNode(n, f); ok {
			ids = append(ids, r.ID)
		}
	}
	return ids
}
