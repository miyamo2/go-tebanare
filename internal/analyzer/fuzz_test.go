package analyzer

import (
	"bytes"
	"flag"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/fuzzvec"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// lineCount returns the number of lines of src; a final newline does not
// start a line.
func lineCount(src []byte) int {
	n := bytes.Count(src, []byte{'\n'})
	if len(src) > 0 && src[len(src)-1] != '\n' {
		n++
	}
	return n
}

func checkRanges(t *testing.T, what string, rs []result.Range, lines int) {
	t.Helper()
	prev := 0
	for _, r := range rs {
		if r.Start < 1 || r.End < r.Start || r.End > lines || r.Start <= prev {
			t.Fatalf("%s: range %d-%d out of order or outside 1-%d: %+v", what, r.Start, r.End, lines, rs)
		}
		if len(r.Hits) == 0 {
			t.Fatalf("%s: range %d-%d has no hits", what, r.Start, r.End)
		}
		prev = r.End
	}
}

// fuzzSet exercises every kind of span: func rules with and without doc
// comments, stmt rules with leading comments, both expr hide modes, and a
// matcher with an explicit span.
func fuzzSet(tb testing.TB) *rule.Set {
	noDoc := funcRule(tb, "new", "func New*")
	noDoc.IncludeDoc = false
	return ruleSet(
		funcRule(tb, "methods", "func (_) *"),
		noDoc,
		withLeadingComments(stmtRule(tb, "stmts", "", `^(return|defer|x|if err)`)),
		exprRule(tb, "calls", "", `^(log|fmt)\.`),
		hideStatement(withLeadingComments(exprRule(tb, "debug", "CallExpr", `Debug|Print`))),
		&rule.Rule{ID: "fold", Target: result.TargetStmt, Node: foldBody{}},
	)
}

var update = flag.Bool("update", false, "rewrite testvectors/fuzz/analyze.json")

// fuzzAnalyzeSeeds returns the seed inputs of FuzzAnalyze.
func fuzzAnalyzeSeeds() [][]byte {
	var seeds [][]byte
	for _, s := range []string{
		occupancyExample, walkSrc, explainSrc, getterOld, occupancySrc,
		string(genSource(60, 3)),
		"package p\n\n//line gen.y:100\nfunc (T) M() {\n\tlog.Debug(\"x\")\n}\n",
		"\xef\xbb\xbfpackage p\r\n\r\nvar s = `a\r\nb` + x\r\nfunc F() { return }\r\n",
		"package p\n\nfunc init() {}\nfunc init() {}\nfunc _() {}\nfunc F() {}\nfunc F() {}\n",
		"package p\n\nfunc f() {\n\t// c\n\tdefer x()\n\tif err := g(); err != nil {\n\t\treturn err\n\t}\n}",
		nestedParens(60), elseIfChain(60), plusChain(400),
		"package p\n\nfunc {\n",
	} {
		seeds = append(seeds, []byte(s))
	}
	return seeds
}

// TestFuzzVectors keeps testvectors/fuzz/analyze.json in step with the
// seeds and the corpus of FuzzAnalyze. The engine tests replay it through
// the wasm build.
func TestFuzzVectors(t *testing.T) {
	corpus, err := fuzzvec.ReadCorpus(filepath.Join("testdata", "fuzz", "FuzzAnalyze"))
	if err != nil {
		t.Fatal(err)
	}
	var f fuzzvec.File
	for _, b := range append(fuzzAnalyzeSeeds(), corpus...) {
		f.Inputs = append(f.Inputs, fuzzvec.NewInput(b))
	}
	fuzzvec.Check(t, "analyze", f, *update)
}

func FuzzAnalyze(f *testing.F) {
	for _, b := range fuzzAnalyzeSeeds() {
		f.Add(b)
	}
	set := fuzzSet(f)
	opt := Options{MaxBracketDepth: 50, MaxElseIfChain: 50, MaxASTDepth: 300, MaxNodeSize: 512}
	f.Fuzz(func(t *testing.T, src []byte) {
		n := lineCount(src)
		file := AnalyzeFile(set, "x.go", src, opt)
		checkRanges(t, "AnalyzeFile", file.Ranges, n)
		for _, fm := range file.Funcs {
			if fm.Start < 1 || fm.End < fm.Start || fm.End > n {
				t.Fatalf("func match %+v outside 1-%d", fm, n)
			}
		}

		// The same source on both sides hides the same lines on both.
		same := AnalyzeChange(set, Side{"x.go", src}, Side{"x.go", src}, opt)
		checkRanges(t, "AnalyzeChange old", same.Old, n)
		if !reflect.DeepEqual(same.Old, same.New) || same.Skipped != file.Skipped {
			t.Fatalf("same source: old %v, new %v, skipped %q", same.Old, same.New, same.Skipped)
		}
		// An added file hides what the file hides on its own.
		added := AnalyzeChange(set, Side{Path: "x.go"}, Side{"x.go", src}, opt)
		if !reflect.DeepEqual(added.New, file.Ranges) || len(added.Old) != 0 {
			t.Fatalf("added file: new %v, want %v", added.New, file.Ranges)
		}

		for _, line := range []int{1, n / 2, n} {
			nodes, err := Explain(set, "x.go", src, line, opt)
			if (err != nil) != (file.Skipped != "") {
				t.Fatalf("Explain error %v with skip %q", err, file.Skipped)
			}
			for _, node := range nodes {
				if node.Line != line || node.EndLine < line || node.EndLine > n || node.Rules == nil {
					t.Fatalf("line %d: node %+v", line, node)
				}
			}
		}
	})
}
