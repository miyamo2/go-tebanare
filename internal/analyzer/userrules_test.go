package analyzer

import (
	"go/ast"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/nodematch"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
	"github.com/miyamo2/go-tebanare/internal/sigpattern"
)

// The helpers in this file build user rules the way the config package
// does: func rules from signature patterns, and stmt and expr rules from
// kinds and regular expressions of normalized text.

// patternMatcher adapts a sigpattern.Pattern to rule.FuncMatcher and
// rule.RelaxedFuncMatcher, as the config package does.
type patternMatcher struct{ p *sigpattern.Pattern }

func (m patternMatcher) MatchFunc(fd *ast.FuncDecl, _ *rule.File) bool { return m.p.Match(fd) }

func (m patternMatcher) MatchFuncRelaxed(fd *ast.FuncDecl, _ *rule.File, q map[string]string) bool {
	return m.p.MatchQualified(fd, q)
}

// funcRule returns a func rule with include_doc true.
func funcRule(t testing.TB, id, pattern string) *rule.Rule {
	t.Helper()
	p, err := sigpattern.Parse(pattern)
	if err != nil {
		t.Fatalf("pattern %q: %v", pattern, err)
	}
	return &rule.Rule{ID: id, Target: result.TargetFunc, IncludeDoc: true, Func: patternMatcher{p}}
}

// nodeRule returns a stmt or expr rule. An empty kind means the default
// kinds.
func nodeRule(t testing.TB, id string, target result.Target, kind, regex string) *rule.Rule {
	t.Helper()
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	}
	m, err := nodematch.New(target, kinds, []string{regex}, nil)
	if err != nil {
		t.Fatalf("rule %s: %v", id, err)
	}
	return &rule.Rule{ID: id, Target: target, Node: m}
}

func stmtRule(t testing.TB, id, kind, regex string) *rule.Rule {
	t.Helper()
	return nodeRule(t, id, result.TargetStmt, kind, regex)
}

func exprRule(t testing.TB, id, kind, regex string) *rule.Rule {
	t.Helper()
	return nodeRule(t, id, result.TargetExpr, kind, regex)
}
