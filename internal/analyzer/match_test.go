package analyzer

import (
	"go/ast"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

func TestMergeAcrossBlankLines(t *testing.T) {
	src := `package p

// Name returns the name.
func (u *User) Name() string { return u.name }

func (u *User) Email() string { return u.email }
func (u *User) Save() error {
	log.Debug("save")

	log.Debug("done")
	return nil
}
`
	set := ruleSet(
		funcsNamed("getter", "Name", "Email"),
		stmtsMatching(t, "debug", "ExprStmt", `^log\.Debug\(`),
	)
	res := AnalyzeFile(set, "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{
		"3-6 getter:func (*User) Name, getter:func (*User) Email",
		`8-10 debug:log.Debug("save"), debug:log.Debug("done")`,
	})
}

// TestLargeNodeLabel checks that a stmt hit on a node larger than
// MaxNodeSize is labeled with the node kind.
func TestLargeNodeLabel(t *testing.T) {
	src := "package p\n\nfunc f() {\n\tif ok {\n\t\tlog.Debug(\"a\")\n\t}\n\tif ok {\n\t\tlog.Debug(\"b\")\n\t}\n\tlog.Debug(\"c\")\n}\n"
	set := ruleSet(
		stmtsMatching(t, "debug", "ExprStmt", `^log\.Debug`),
		&rule.Rule{ID: "fold", Preset: "fold", Target: result.TargetStmt, Node: foldBody{}},
	)
	// The if statements are 31 bytes long.
	res := AnalyzeFile(set, "x.go", []byte(src), Options{MaxNodeSize: 20})
	check(t, "ranges", show(res.Ranges), []string{
		`5-6 fold:IfStmt, debug:log.Debug("a")`,
		`8-10 fold:IfStmt, debug:log.Debug("b"), debug:log.Debug("c")`,
	})
	check(t, "diagnostics", codes(res.Diagnostics), nil)
}

// foldBody hides an if statement from its return statement to its end,
// like the iferr preset with init: fold-body.
type foldBody struct{}

func (foldBody) Accepts(n ast.Node) bool { _, ok := n.(*ast.IfStmt); return ok }

func (foldBody) MatchNode(n ast.Node, _ *rule.File) (rule.Span, bool) {
	s := n.(*ast.IfStmt)
	if len(s.Body.List) != 1 {
		return rule.Span{}, false
	}
	return rule.Span{From: s.Body.List[0].Pos(), To: s.End()}, true
}

func TestAnalyzeFileExplicitSpan(t *testing.T) {
	src := "package p\n\nfunc f() error {\n\tif err := g(); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n"
	r := &rule.Rule{ID: "iferr", Preset: "iferr", Target: result.TargetStmt, Node: foldBody{}}
	res := AnalyzeFile(ruleSet(r), "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{"5-6 iferr:if err := g(); err != nil { return err }"})
	if h := res.Ranges[0].Hits[0]; h.Preset != "iferr" || h.Node != "IfStmt" || h.Target != result.TargetStmt {
		t.Errorf("hit = %+v", h)
	}
}
