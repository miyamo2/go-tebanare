package analyzer

import (
	"testing"
)

func TestPackageLevelValues(t *testing.T) {
	src := `package p

var tests = []tc{
	{name: "legacy", in: 1, want: 2},
	{name: "new", in: 2, want: 3},
	{name: "legacy", in: 3, want: 4},
}

var handler = func() {
	log.Debug("x")
	if ok {
		return
	}
}
`
	set := ruleSet(
		exprRule(t, "legacy", "CompositeLit", `^\{name: "legacy"`),
		stmtRule(t, "ret", "", `^return$`),
		exprRule(t, "debug", "CallExpr", `^log\.Debug\(`),
	)
	res := AnalyzeFile(set, "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{
		`4-4 legacy:{name: "legacy", in: 1, want: 2}`,
		`6-6 legacy:{name: "legacy", in: 3, want: 4}`,
		`10-10 debug:log.Debug("x")`,
		`12-12 ret:return`,
	})
}

func TestExprScope(t *testing.T) {
	src := `package p

import (
	"fmt"
)

type Table struct {
	Index map[string]int
}

type Alias = map[string]int

var typed map[string]int

func Lookup(
	m map[string]int,
) map[string]int {
	f := func(
		m map[string]int,
	) {
	}
	_ = f
	return make(
		map[string]int,
	)
}

func Print() {
	fmt.Println(
		"fmt",
	)
}
`
	set := ruleSet(
		exprRule(t, "map", "MapType", `^map\[string\]int$`),
		exprRule(t, "lit", "BasicLit", `^"fmt"$`),
	)
	res := AnalyzeFile(set, "x.go", []byte(src), DefaultOptions())
	// Only the expressions inside function bodies count: no hit and no
	// line-shared diagnostic comes from the import, the type
	// declarations, the var type, or the signatures.
	check(t, "ranges", show(res.Ranges), []string{
		`24-24 map:map[string]int`,
		`30-30 lit:"fmt"`,
	})
	check(t, "diagnostics", codes(res.Diagnostics), nil)
}

// TestNodeTooLarge checks that a user rule reports the nodes it skipped
// because their normalized text is larger than MaxNodeSize.
func TestNodeTooLarge(t *testing.T) {
	src := "package p\n\nfunc f() {\n\tif ok {\n\t\tlog.Debug(\"a\")\n\t}\n\tif ok {\n\t\tlog.Debug(\"b\")\n\t}\n}\n"
	// The if statements are 31 bytes long.
	res := AnalyzeFile(ruleSet(stmtRule(t, "if", "IfStmt", `^if ok`)), "x.go", []byte(src), Options{MaxNodeSize: 20})
	check(t, "ranges", show(res.Ranges), nil)
	check(t, "diagnostics", codes(res.Diagnostics), []string{"node-too-large if 4"})
	if msg := res.Diagnostics[0].Message; msg != `rule "if" skipped 2 nodes larger than 20 bytes, the first on line 4` {
		t.Errorf("message %q", msg)
	}
}

func TestAnalyzeFileAliasNotResolved(t *testing.T) {
	src := "package p\n\nimport ctx2 \"context\"\n\nfunc (s *S) Run(ctx ctx2.Context) {}\n"
	res := AnalyzeFile(ruleSet(funcRule(t, "run", "func (_) Run(context.Context)")), "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), nil)
	check(t, "diagnostics", codes(res.Diagnostics), []string{"alias-not-resolved run 5"})
}

// TestAnalyzeFileOccupancyExpr runs the occupancy example of plan 4.7 with
// its expr rules.
func TestAnalyzeFileOccupancyExpr(t *testing.T) {
	set := ruleSet(
		stmtRule(t, "ret", "ReturnStmt", `^return x \+ 1$`),
		exprRule(t, "debug", "CallExpr", `^log\.Debug\(`),
		exprRule(t, "legacy", "CompositeLit", `^\{name: "legacy"`),
	)
	res := AnalyzeFile(set, "x.go", []byte(occupancyExample), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{
		`6-6 debug:log.Debug("...")`,
		`8-8 legacy:{name: "legacy", in: 1, want: 2}`,
	})
	check(t, "diagnostics", codes(res.Diagnostics), []string{
		"line-shared ret 4",
		"line-shared debug 5",
	})
}

func TestAnalyzeChangeIndependentExprNodes(t *testing.T) {
	set := ruleSet(
		stmtRule(t, "debug", "ExprStmt", `^log\.Debug\(`),
		exprRule(t, "legacy", "CompositeLit", `^\{name: "legacy"`),
	)
	old := "package p\n\nfunc f() {\n\tlog.Debug(\"a\")\n\tlog.Info(\"b\")\n}\n"
	new := "package p\n\nvar tests = []tc{\n\t{name: \"legacy\"},\n}\n\nfunc f() {\n\tlog.Info(\"a\")\n\tlog.Debug(\"b\")\n}\n"
	res := AnalyzeChange(set, Side{"x.go", []byte(old)}, Side{"x.go", []byte(new)}, DefaultOptions())
	check(t, "old", show(res.Old), []string{`4-4 debug:log.Debug("a")`})
	check(t, "new", show(res.New), []string{`4-4 legacy:{name: "legacy"}`, `9-9 debug:log.Debug("b")`})
}
