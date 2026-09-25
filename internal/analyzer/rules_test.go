package analyzer

import (
	"fmt"
	"go/ast"
	"regexp"
	"slices"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// funcsWhere returns a func rule with include_doc true that matches the
// declarations for which match reports true.
func funcsWhere(id string, match func(fd *ast.FuncDecl) bool) *rule.Rule {
	return &rule.Rule{ID: id, Target: result.TargetFunc, IncludeDoc: true, Func: rule.FuncMatcherFunc(
		func(fd *ast.FuncDecl, _ *rule.File) bool { return match(fd) })}
}

// funcsNamed returns a func rule with include_doc true that matches the
// functions and methods named in names.
func funcsNamed(id string, names ...string) *rule.Rule {
	return funcsWhere(id, func(fd *ast.FuncDecl) bool { return slices.Contains(names, fd.Name.Name) })
}

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
