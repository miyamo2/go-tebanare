package presets

import (
	"go/ast"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// TestIferrFoldBody checks the exact span of init: fold-body: from the
// return statement to the end of the if statement.
func TestIferrFoldBody(t *testing.T) {
	src := []byte(`package p

func f() error {
	if err := load(); err != nil {
		return err
	}
	if err != nil {
		return err
	}
}
`)
	p, _ := Lookup("iferr")
	f := parseSource(t, "x.go", src)
	var ifs []*ast.IfStmt
	ast.Inspect(f.AST, func(n ast.Node) bool {
		if s, ok := n.(*ast.IfStmt); ok {
			ifs = append(ifs, s)
		}
		return true
	})
	withInit, plain := ifs[0], ifs[1]
	ret := withInit.Body.List[0]

	fold := compileYAML(t, p, "init: fold-body").Node
	span, ok := fold.MatchNode(withInit, f)
	if !ok || span.From != ret.Pos() || span.To != withInit.End() {
		t.Errorf("fold-body: MatchNode = %+v, %v, want {%v %v}, true", span, ok, ret.Pos(), withInit.End())
	}
	if from, to := f.Line(span.From), f.Line(span.To); from != 5 || to != 6 {
		t.Errorf("fold-body: span lines %d-%d, want 5-6", from, to)
	}
	if span, ok := fold.MatchNode(plain, f); !ok || !span.IsZero() {
		t.Errorf("fold-body without init: MatchNode = %+v, %v, want zero span, true", span, ok)
	}

	exclude := compileYAML(t, p, "").Node
	if _, ok := exclude.MatchNode(withInit, f); ok {
		t.Error("init: exclude matches an if statement with an init statement")
	}
	if span, ok := exclude.MatchNode(plain, f); !ok || !span.IsZero() {
		t.Errorf("default: MatchNode = %+v, %v, want zero span, true", span, ok)
	}
	if exclude.Accepts(ret) || !exclude.Accepts(plain) {
		t.Error("Accepts must approve if statements only")
	}
	if _, ok := exclude.MatchNode(ret, f); ok {
		t.Error("MatchNode matches a return statement")
	}
}

// TestIferrCases covers more shapes, written one per line. Some of them
// depend on a layout that gofmt would rewrite, so they are not in testdata.
func TestIferrCases(t *testing.T) {
	tests := []struct {
		settings, code string
		want           bool
	}{
		{"", "if err != nil { return err }", true},
		{"", "if (err != nil) { return err }", true},
		{"", "if (err) != (nil) { return (err) }", true},
		{"", "if err != nil { return new(T), err }", true},
		{"", "if err != nil { return 0, \"\", x.y, T{}, []int{1}, a + b, -n, *p, err }", true},
		{"", "for { if err != nil { return err } }", true},
		{"", "if err != nil { /* why */ return err }", false},
		{"", "if err == nil { return err }", false},
		{"", "if err != x { return err }", false},
		{"", "if err != nil && ok { return err }", false},
		{"", "if r.err != nil { return r.err }", false},
		{"", "if parseErr != nil { return parseErr }", false},
		{"", "if err != nil { continue }", false},
		{"", "if err != nil { return err, nil }", false},
		{"", "if err != nil { return nil, other }", false},
		{"", "if err != nil { return T(x), err }", false},
		{"", "if err != nil { return new(compute()), err }", false},
		{"", "if err != nil { return f(<-ch), err }", false},
		{"", "if err != nil {}", false},
		{"", "if err != nil { { return err } }", false},
		{"", "if err != nil { return err; return err }", false},
		{"init: fold-body", "if err := load(); err != nil { return err }", false},
		{"init: fold-body", "if err := load(); err != nil { return err\n}", false},
		{"init: fold-body", "if err := load(); err != nil {\n\treturn err }", true},
		{"init: fold-body", "if err := load(); err != nil {\n\treturn err } // why", false},
	}
	p, _ := Lookup("iferr")
	for _, tt := range tests {
		r := compileYAML(t, p, tt.settings)
		f := parseSource(t, "x.go", []byte("package p\n\nfunc _() {\n"+tt.code+"\n}\n"))
		nodes := candidates(t, p, f)
		if len(nodes) != 1 {
			t.Fatalf("%q: %d if statements", tt.code, len(nodes))
		}
		if got := matches(r, nodes[0], f); got != tt.want {
			t.Errorf("settings %q: match(%q) = %v, want %v", tt.settings, tt.code, got, tt.want)
		}
	}
}

func TestIferrDecodeErrors(t *testing.T) {
	p, _ := Lookup("iferr")
	tests := []struct{ src, want string }{
		{"names: []", "1:8: names: at least one name is required"},
		{"names: [err, a-b]", `1:14: names[1]: invalid name "a-b": use letters, digits, "_", "*", and "?"`},
		{"names: [a-b, 1]", `1:9: names[0]: invalid name "a-b": use letters, digits, "_", "*", and "?"` +
			"\n1:14: names[1]: expected a string, found integer 1"},
		{"names: [1]", "1:9: names[0]: expected a string, found integer 1"},
		{"names: !!str [err]", "1:8: names: expected a list of strings, found a list tagged !!str"},
		{"names: err", `1:8: names: expected a list of strings, found string "err"`},
		{"init: fold", `1:7: init: unknown value "fold" (valid values: exclude, fold-body)`},
		{"include_doc: true", `1:1: include_doc: unknown setting "include_doc" (available settings: ` +
			`paths, exclude_paths, names, allow_comments, init, allow_bare_return, allow_calls_in_results)`},
	}
	for _, tt := range tests {
		_, err := decodeYAML(t, p, tt.src)
		if err == nil || err.Error() != tt.want {
			t.Errorf("Decode(%q) error = %v, want %q", tt.src, err, tt.want)
		}
	}

	for _, change := range []func(*IferrSettings){
		func(s *IferrSettings) { s.Init = "" },
		func(s *IferrSettings) { s.Names = nil },
		func(s *IferrSettings) { s.Paths = []string{"["} },
	} {
		s := p.NewSettings().(*IferrSettings)
		change(s)
		if _, err := p.Compile(s); err == nil {
			t.Errorf("Compile(%+v) succeeded", s)
		}
	}
}

var _ rule.NodeMatcher = (*iferrMatcher)(nil)
