package rule

import (
	"go/ast"
	"testing"
)

func TestSetIsTarget(t *testing.T) {
	tests := []struct {
		set  Set
		path string
		want bool
	}{
		{Set{}, "main.go", true},
		{Set{}, "a/b/c.go", true},
		{Set{}, "a/b/c.txt", false},
		{Set{Include: []string{"**/*"}}, "README.md", false},
		{Set{Exclude: []string{"vendor/**"}}, "vendor/x/y.go", false},
		{Set{Exclude: []string{"vendor/**"}}, "internal/vendor.go", true},
		{Set{Include: []string{"internal/**"}}, "cmd/main.go", false},
		{Set{Include: []string{"internal/**"}}, "internal/a/b.go", true},
		{Set{}, "A/B.GO", false},
	}
	for _, tt := range tests {
		if got := tt.set.IsTarget(tt.path); got != tt.want {
			t.Errorf("%+v.IsTarget(%q) = %v, want %v", tt.set, tt.path, got, tt.want)
		}
	}
}

func TestRuleAppliesTo(t *testing.T) {
	tests := []struct {
		r    Rule
		path string
		want bool
	}{
		{Rule{}, "x.go", true},
		{Rule{Paths: []string{"**/mock_*.go"}}, "a/mock_repo.go", true},
		{Rule{Paths: []string{"**/mock_*.go"}}, "a/repo.go", false},
		{Rule{ExcludePaths: []string{"a/**"}}, "a/repo.go", false},
		{Rule{Paths: []string{"**"}, ExcludePaths: []string{"a/**"}}, "b/repo.go", true},
		{Rule{Paths: []string{"[bad"}}, "x.go", false},
	}
	for _, tt := range tests {
		if got := tt.r.AppliesTo(tt.path); got != tt.want {
			t.Errorf("AppliesTo(%q) with paths %q exclude %q = %v, want %v",
				tt.path, tt.r.Paths, tt.r.ExcludePaths, got, tt.want)
		}
	}
}

func TestSpanIsZero(t *testing.T) {
	if !(Span{}).IsZero() {
		t.Error("zero Span is not zero")
	}
	if (Span{From: 1}).IsZero() {
		t.Error("Span{From: 1} is zero")
	}
}

func TestFuncMatcherFunc(t *testing.T) {
	var m FuncMatcher = FuncMatcherFunc(func(fd *ast.FuncDecl, _ *File) bool {
		return fd.Name.Name == "F"
	})
	if !m.MatchFunc(&ast.FuncDecl{Name: ast.NewIdent("F")}, nil) {
		t.Error("FuncMatcherFunc did not call the function")
	}
}
