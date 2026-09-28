package nodematch

import (
	"errors"
	"go/ast"
	"slices"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

func mustNew(t *testing.T, target result.Target, kinds, regex, notRegex []string) *Matcher {
	t.Helper()
	m, err := New(target, kinds, regex, notRegex)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestAccepts(t *testing.T) {
	nodes := []ast.Node{
		&ast.IfStmt{}, &ast.AssignStmt{}, &ast.DeferStmt{}, &ast.ExprStmt{},
		&ast.BlockStmt{}, &ast.EmptyStmt{}, &ast.CallExpr{}, &ast.CompositeLit{},
		&ast.Ident{}, &ast.BasicLit{}, &ast.FuncDecl{}, &ast.Field{}, nil,
	}
	tests := []struct {
		name   string
		target result.Target
		kinds  []string
		want   []string
	}{
		{
			name:   "stmt default excludes BlockStmt and EmptyStmt",
			target: result.TargetStmt,
			want:   []string{"IfStmt", "AssignStmt", "DeferStmt", "ExprStmt"},
		},
		{
			name:   "expr default excludes Ident and BasicLit",
			target: result.TargetExpr,
			want:   []string{"CallExpr", "CompositeLit"},
		},
		{
			name:   "empty list means default",
			target: result.TargetExpr,
			kinds:  []string{},
			want:   []string{"CallExpr", "CompositeLit"},
		},
		{
			name:   "explicit stmt kinds",
			target: result.TargetStmt,
			kinds:  []string{"AssignStmt", "DeferStmt"},
			want:   []string{"AssignStmt", "DeferStmt"},
		},
		{
			name:   "explicit kinds may name excluded defaults",
			target: result.TargetStmt,
			kinds:  []string{"BlockStmt", "EmptyStmt", "BlockStmt"},
			want:   []string{"BlockStmt", "EmptyStmt"},
		},
		{
			name:   "explicit expr kinds",
			target: result.TargetExpr,
			kinds:  []string{"Ident", "BasicLit"},
			want:   []string{"Ident", "BasicLit"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustNew(t, tt.target, tt.kinds, []string{"x"}, nil)
			for _, n := range nodes {
				want := slices.Contains(tt.want, rule.KindOf(n))
				if got := m.Accepts(n); got != want {
					t.Errorf("Accepts(%T) = %v, want %v", n, got, want)
				}
			}
		})
	}
}

func TestNewErrors(t *testing.T) {
	tests := []struct {
		name     string
		target   result.Target
		kinds    []string
		regex    []string
		notRegex []string
		want     []string // Error() of each wrapped *Error
	}{
		{
			name:   "unknown stmt kind",
			target: result.TargetStmt,
			kinds:  []string{"IfStmt", "IfStatement"},
			regex:  []string{"x"},
			want: []string{`kind[1]: unknown stmt kind "IfStatement" (valid kinds: ` +
				strings.Join(rule.StmtKinds, ", ") + ")"},
		},
		{
			name:   "expr kind in a stmt rule",
			target: result.TargetStmt,
			kinds:  []string{"CallExpr"},
			regex:  []string{"x"},
			want: []string{`kind[0]: unknown stmt kind "CallExpr" (valid kinds: ` +
				strings.Join(rule.StmtKinds, ", ") + ")"},
		},
		{
			name:   "stmt kind in an expr rule",
			target: result.TargetExpr,
			kinds:  []string{"ExprStmt"},
			regex:  []string{"x"},
			want: []string{`kind[0]: unknown expr kind "ExprStmt" (valid kinds: ` +
				strings.Join(rule.ExprKinds, ", ") + ")"},
		},
		{
			name:   "regex compile error names the index",
			target: result.TargetStmt,
			regex:  []string{`^a$`, `^ctx, span := \w+\.Start(ctx, .+$`},
			want:   []string{"regex[1]: error parsing regexp: missing closing ): `^ctx, span := \\w+\\.Start(ctx, .+$`"},
		},
		{
			name:     "not_regex compile error",
			target:   result.TargetExpr,
			regex:    []string{`^log\.`},
			notRegex: []string{`[z-a]`},
			want:     []string{"not_regex[0]: error parsing regexp: invalid character class range: `z-a`"},
		},
		{
			name:     "too deeply nested",
			target:   result.TargetExpr,
			regex:    []string{strings.Repeat("(", 1001) + "a" + strings.Repeat(")", 1001)},
			notRegex: []string{strings.Repeat("(", 201)},
			want: []string{
				`regex[0]: the expression nests too deeply (estimated depth 6010, more than the limit of 600)`,
				`not_regex[0]: the expression nests too deeply (estimated depth 1210, more than the limit of 600)`,
			},
		},
		{
			name:   "regex required",
			target: result.TargetExpr,
			want:   []string{"regex: at least one regular expression is required"},
		},
		{
			name:     "every error is reported",
			target:   result.TargetExpr,
			kinds:    []string{"Nope"},
			regex:    []string{`(`, `ok`, `)`},
			notRegex: []string{`*`},
			want: []string{
				`kind[0]: unknown expr kind "Nope" (valid kinds: ` + strings.Join(rule.ExprKinds, ", ") + ")",
				"regex[0]: error parsing regexp: missing closing ): `(`",
				"regex[2]: error parsing regexp: unexpected ): `)`",
				"not_regex[0]: error parsing regexp: missing argument to repetition operator: `*`",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := New(tt.target, tt.kinds, tt.regex, tt.notRegex)
			if err == nil {
				t.Fatalf("New returned %v, want an error", m)
			}
			if got := err.Error(); got != strings.Join(tt.want, "\n") {
				t.Errorf("Error():\n got %s\nwant %s", got, strings.Join(tt.want, "\n"))
			}
			var first *Error
			if !errors.As(err, &first) || first.Error() != tt.want[0] {
				t.Errorf("errors.As found %v, want %s", first, tt.want[0])
			}
			joined, ok := err.(interface{ Unwrap() []error })
			if !ok || len(joined.Unwrap()) != len(tt.want) {
				t.Errorf("error wraps %v, want %d errors", err, len(tt.want))
			}
		})
	}
}

func TestNewInvalidTarget(t *testing.T) {
	if _, err := New(result.TargetFunc, nil, []string{"x"}, nil); err == nil {
		t.Error("New(TargetFunc) returned no error")
	}
}

func TestMatchText(t *testing.T) {
	tests := []struct {
		name     string
		regex    []string
		notRegex []string
		text     string
		want     bool
	}{
		{"first regex", []string{`^a$`, `^b$`}, nil, "a", true},
		{"second regex", []string{`^a$`, `^b$`}, nil, "b", true},
		{"no regex", []string{`^a$`, `^b$`}, nil, "ab", false},
		{"search semantics", []string{`Debug`}, nil, `log.Debug("x")`, true},
		{"empty regex matches everything", []string{``}, nil, "anything", true},
		{"not_regex excludes", []string{`^a`}, []string{`b`}, "ab", false},
		{"any not_regex excludes", []string{`^a`}, []string{`x`, `b`}, "ab", false},
		{"not_regex without match", []string{`^a`}, []string{`x`, `y`}, "ab", true},
		{"not_regex alone does not match", []string{`^z`}, []string{`b`}, "ab", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustNew(t, result.TargetExpr, nil, tt.regex, tt.notRegex)
			if got := m.MatchText(tt.text); got != tt.want {
				t.Errorf("MatchText(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestMatchTextDebugLog uses the debug-log rule from the example config in
// plan section 4.2.
func TestMatchTextDebugLog(t *testing.T) {
	m := mustNew(t, result.TargetExpr, []string{"CallExpr"},
		[]string{`^(log|slog|logger)\.Debug\w*\(`}, []string{`(?i)password|token|secret`})
	tests := []struct {
		text string
		want bool
	}{
		{`log.Debug("find user", "id", id)`, true},
		{`slog.DebugContext(ctx, "cache hit")`, true},
		{`logger.Debugf("%d rows", n)`, true},
		{`logger.Debugf("token=%s", tok)`, false},
		{`slog.DebugContext(ctx, "Password reset", "user", u)`, false},
		{`log.Debug("rotate", "SECRET", s)`, false},
		{`log.Info("find user")`, false},
		{`mylog.Debug("x")`, false},
	}
	for _, tt := range tests {
		if got := m.MatchText(tt.text); got != tt.want {
			t.Errorf("MatchText(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
