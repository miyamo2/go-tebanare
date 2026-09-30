package analyzer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

const explainSrc = `package p

type T struct{ M map[string]int }

var _ = 1
func (u *User) Name() string { return u.name }

func f() {
	x := compute(log.Debug("..."))
	_ = x
}
`

// describe renders nodes as "Kind line-endLine inScope [rules] text".
func describe(nodes []NodeInfo) []string {
	out := []string{}
	for _, n := range nodes {
		out = append(out, fmt.Sprintf("%s %d-%d %v %v %s", n.Kind, n.Line, n.EndLine, n.InScope, n.Rules, n.Text))
	}
	return out
}

func TestExplain(t *testing.T) {
	set := ruleSet(
		funcRule(t, "getter", "func (*User) Name() string"),
		funcRule(t, "other", "func (*User) Other()"),
		exprRule(t, "debug", "CallExpr", `^log\.Debug\(`),
		exprRule(t, "map", "MapType", `^map`),
		stmtRule(t, "assign", "AssignStmt", `compute`),
	)
	tests := []struct {
		line int
		want []string
	}{
		{3, []string{
			"Ident 3-3 false [] T",
			"StructType 3-3 false [] struct{ M map[string]int }",
			"Ident 3-3 false [] M",
			"MapType 3-3 false [map] map[string]int",
			"Ident 3-3 false [] string",
			"Ident 3-3 false [] int",
		}},
		{5, []string{"Ident 5-5 false [] _", "BasicLit 5-5 true [] 1"}},
		{6, []string{
			"FuncDecl 6-6 false [getter] func (u *User) Name() string { return u.name }",
			"Ident 6-6 false [] u",
			"StarExpr 6-6 false [] *User",
			"Ident 6-6 false [] User",
			"Ident 6-6 false [] Name",
			"FuncType 6-6 false [] func() string",
			"Ident 6-6 false [] string",
			"BlockStmt 6-6 false [] { return u.name }",
			"ReturnStmt 6-6 true [] return u.name",
			"SelectorExpr 6-6 true [] u.name",
			"Ident 6-6 true [] u",
			"Ident 6-6 true [] name",
		}},
		{9, []string{
			`AssignStmt 9-9 true [assign] x := compute(log.Debug("..."))`,
			"Ident 9-9 true [] x",
			`CallExpr 9-9 true [] compute(log.Debug("..."))`,
			"Ident 9-9 true [] compute",
			`CallExpr 9-9 true [debug] log.Debug("...")`,
			"SelectorExpr 9-9 true [] log.Debug",
			"Ident 9-9 true [] log",
			"Ident 9-9 true [] Debug",
			`BasicLit 9-9 true [] "..."`,
		}},
		{8, []string{
			`FuncDecl 8-11 false [] func f() { x := compute(log.Debug("...")) _ = x }`,
			"Ident 8-8 false [] f",
			"FuncType 8-8 false [] func()",
			`BlockStmt 8-11 false [] { x := compute(log.Debug("...")) _ = x }`,
		}},
		{0, []string{}},
		{99, []string{}},
	}
	for _, tt := range tests {
		nodes, err := Explain(set, "x.go", []byte(explainSrc), tt.line, DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		check(t, fmt.Sprintf("line %d", tt.line), describe(nodes), tt.want)
	}
}

func TestExplainDocLine(t *testing.T) {
	src := "package p\n\n// F does it.\nfunc F() {}\n"
	nodes, err := Explain(nil, "x.go", []byte(src), 3, DefaultOptions())
	if err != nil || len(nodes) != 0 {
		t.Errorf("doc line: got %v, %v, want no nodes", nodes, err)
	}
	nodes, err = Explain(nil, "x.go", []byte(src), 4, DefaultOptions())
	if err != nil || len(nodes) == 0 || nodes[0].Kind != "FuncDecl" || nodes[0].Line != 4 || nodes[0].EndLine != 4 {
		t.Errorf("func line: got %v, %v, want a FuncDecl on line 4", nodes, err)
	}
}

func TestExplainLargeNode(t *testing.T) {
	nodes, err := Explain(nil, "x.go", []byte(explainSrc), 8, Options{MaxNodeSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	check(t, "nodes", describe(nodes), []string{
		"FuncDecl 8-11 false [] ",
		"Ident 8-8 false [] f",
		"FuncType 8-8 false [] func()",
		"BlockStmt 8-11 false [] ",
	})
	b, err := json.Marshal(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("JSON has null: %s", b)
	}
}

func TestExplainSkip(t *testing.T) {
	for _, tt := range []struct {
		path, src string
		want      result.SkipReason
		msg       string
	}{
		{"x.go", "package p\n\nfunc {\n", result.SkipParseError, "parse-error: 3:6: expected 'IDENT', found '{'"},
		{"x.txt", "package p\n", result.SkipNotTarget, "not-target: x.txt is not selected by files.include and files.exclude"},
		{"x.go", nestedParens(300), result.SkipTooDeep, "too-deep: 3:"},
	} {
		_, err := Explain(nil, tt.path, []byte(tt.src), 1, DefaultOptions())
		var se *SkipError
		if !errors.As(err, &se) || se.Reason != tt.want || !strings.HasPrefix(err.Error(), tt.msg) {
			t.Errorf("%s: got %v, want a *SkipError %s starting with %q", tt.path, err, tt.want, tt.msg)
		}
	}
}

// TestAttachedComments checks that the comments go/ast attaches to a
// declaration change neither the text that rules match nor the text that
// Explain shows.
func TestAttachedComments(t *testing.T) {
	src := "package p\n\nfunc F() {\n\tvar x = 1 // trailing\n\t_ = x\n}\n"
	set := ruleSet(
		stmtRule(t, "var", "DeclStmt", `^var x = 1$`),
		stmtRule(t, "comment", "DeclStmt", `trailing`),
	)
	res := AnalyzeFile(set, "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{"4-4 var:var x = 1"})
	nodes, err := Explain(set, "x.go", []byte(src), 4, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	check(t, "explain", describe(nodes), []string{
		"DeclStmt 4-4 true [var] var x = 1",
		"Ident 4-4 true [] x",
		"BasicLit 4-4 true [] 1",
	})
}
