package analyzer

import (
	"go/ast"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

func TestAnalyzeFileDoc(t *testing.T) {
	src := `package p

// String returns the name.
func (u User) String() string { return u.name }

// String returns the name.
//
//go:generate stringer
func (p *Point) String() string { return p.name }

//nolint:errcheck
func (e Enum) String() string { return "e" }

//line gen.y:100
func (g Gen) String() string {
	return "g"
}

// String is exported to C.
//
//export String
func (c C) String() string { return "c" }
`
	withDoc := funcsNamed("stringer", "String")
	res := AnalyzeFile(ruleSet(withDoc), "x.go", []byte(src), DefaultOptions())
	check(t, "include_doc true", show(res.Ranges), []string{
		"3-4 stringer:func (User) String",
		"9-9 stringer:func (*Point) String",
		"12-12 stringer:func (Enum) String",
		// Real lines of the file, not gen.y:100.
		"15-17 stringer:func (Gen) String",
		"22-22 stringer:func (C) String",
	})
	withDoc.IncludeDoc = false
	res = AnalyzeFile(ruleSet(withDoc), "x.go", []byte(src), DefaultOptions())
	check(t, "include_doc false", show(res.Ranges), []string{
		"4-4 stringer:func (User) String",
		"9-9 stringer:func (*Point) String",
		"12-12 stringer:func (Enum) String",
		"15-17 stringer:func (Gen) String",
		"22-22 stringer:func (C) String",
	})
	want := []FuncMatch{{Key: "User.String", RuleID: "stringer", Start: 4, End: 4}}
	if got := res.Funcs[0]; got.Key != want[0].Key || got.Start != 4 || got.End != 4 || got.Hit.Node != "FuncDecl" {
		t.Errorf("Funcs[0] = %+v, want %+v", got, want[0])
	}
}

func TestAnalyzeFileLineDirective(t *testing.T) {
	// Positions after a //line directive map to gen.y; the ranges must
	// still use the lines of the file.
	src := "package p\n\n//line gen.y:100\nvar x = 1\n\nfunc f() {\n\tlog.Debug(\"x\")\n}\n"
	res := AnalyzeFile(ruleSet(stmtsMatching(t, "debug", "ExprStmt", `^log\.Debug`)), "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{`7-7 debug:log.Debug("x")`})
}

func TestAnalyzeFileKeys(t *testing.T) {
	src := `package p

func init() {}

func init() {}

func _() {}

func (T) _() {}

func (T) _() {}

func F() {}

func F() {}

func (t *T) M() {}

func (t T[K]) N() {}

func (t []int) Odd() {}
`
	all := &rule.Rule{ID: "all", Target: result.TargetFunc, Func: rule.FuncMatcherFunc(
		func(*ast.FuncDecl, *rule.File) bool { return true })}
	res := AnalyzeFile(ruleSet(all), "x.go", []byte(src), DefaultOptions())
	wantDecls := map[string]int{
		"init#0": 1, "init#1": 1, "_#0": 1, "T._#0": 1, "T._#1": 1,
		"F": 2, "T.M": 1, "T.N": 1, "([]int).Odd": 1,
	}
	if len(res.Decls) != len(wantDecls) {
		t.Errorf("Decls = %v, want %v", res.Decls, wantDecls)
	}
	for k, v := range wantDecls {
		if res.Decls[k] != v {
			t.Errorf("Decls[%q] = %d, want %d", k, res.Decls[k], v)
		}
	}
	var keys []string
	for _, f := range res.Funcs {
		keys = append(keys, f.Key)
	}
	check(t, "funcs", keys, []string{"init#0", "init#1", "_#0", "T._#0", "T._#1", "F", "F", "T.M", "T.N", "([]int).Odd"})
	// The duplicated F is left out of the ranges.
	check(t, "ranges", show(res.Ranges), []string{
		"3-11 all:func init, all:func _, all:func (T) _",
		"17-21 all:func (*T) M, all:func (T[K]) N, all:func ([]int) Odd",
	})
	check(t, "diagnostics", codes(res.Diagnostics), []string{"duplicate-decl  13"})
}

func TestAnalyzeFileFuncLineShared(t *testing.T) {
	src := "package p\n\nvar x = 1; func F() {}\n\n// G does it.\nfunc G() {}; var y = 2\n\nfunc H() {}\n"
	functions := funcsWhere("f", func(fd *ast.FuncDecl) bool { return fd.Recv == nil })
	res := AnalyzeFile(ruleSet(functions), "x.go", []byte(src), DefaultOptions())
	check(t, "ranges", show(res.Ranges), []string{"8-8 f:func H"})
	check(t, "diagnostics", codes(res.Diagnostics), []string{"line-shared f 3", "line-shared f 6"})
	if len(res.Funcs) != 1 || res.Funcs[0].Key != "H" {
		t.Errorf("Funcs = %+v, want only H", res.Funcs)
	}
}
