package analyzer

import (
	"fmt"
	"go/ast"
	"regexp"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// stmtsMatching returns a stmt rule that matches the statements of the
// given kind, or of any kind when kind is "", whose source text matches
// the regular expression re.
func stmtsMatching(t testing.TB, id, kind, re string) *rule.Rule {
	t.Helper()
	rx, err := regexp.Compile(re)
	if err != nil {
		t.Fatalf("rule %s: %v", id, err)
	}
	return &rule.Rule{ID: id, Target: result.TargetStmt, Node: sourceMatcher{kind: kind, re: rx}}
}

type sourceMatcher struct {
	kind string
	re   *regexp.Regexp
}

func (m sourceMatcher) Accepts(n ast.Node) bool {
	_, ok := n.(ast.Stmt)
	return ok && (m.kind == "" || rule.KindOf(n) == m.kind)
}

func (m sourceMatcher) MatchNode(n ast.Node, f *rule.File) (rule.Span, bool) {
	return rule.Span{}, m.re.Match(f.Src[f.Offset(n.Pos()):f.Offset(n.End())])
}

func ruleSet(rules ...*rule.Rule) *rule.Set { return &rule.Set{Rules: rules} }

// codes renders diagnostics as "code rule line".
func codes(ds []result.Diagnostic) []string {
	out := []string{}
	for _, d := range ds {
		s := fmt.Sprintf("%s %s %d", d.Code, d.RuleID, d.Line)
		if d.Side != "" {
			s = d.Side + " " + s
		}
		out = append(out, s)
	}
	return out
}
