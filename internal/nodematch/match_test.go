package nodematch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/canon"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

const matchSrc = `package p

import f "fmt"

func (s *Service) Find(ctx context.Context, id string) (*User, error) {
	ctx, span := tracer.Start(ctx, "Service.Find",
		trace.WithAttributes(attribute.String("id", id)))
	defer span.End()

	log.Debug("find user",
		"id", id)
	logger.Debugf("token=%s", s.token)
	slog.DebugContext(ctx, "Password reset")
	log.Info("find")
	f.Println("aliased fmt")
	if u, ok := s.cache[id]; ok {
		logger.Debug("cache hit")
		return u, nil
	}
	return s.repo.Find(ctx, id)
}
`

// matches returns "line: text" for every node in matchSrc that m accepts
// and matches, using a canon.Cache with the given size limit. The text is
// the one MatchNode matched against.
func matches(t *testing.T, m *Matcher, limit int) []string {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "x.go", matchSrc, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	cache := canon.NewCache(fset, limit)
	f := &rule.File{Path: "x.go", Fset: fset, AST: af, Src: []byte(matchSrc), Canon: cache.Text}
	var got []string
	ast.Inspect(af, func(n ast.Node) bool {
		if n == nil || !m.Accepts(n) {
			return true
		}
		span, ok := m.MatchNode(n, f)
		if !span.IsZero() {
			t.Errorf("MatchNode returned span %v, want zero", span)
		}
		if ok {
			text, _ := f.Text(n)
			got = append(got, fmt.Sprintf("%d: %s", f.Line(n.Pos()), text))
		}
		return true
	})
	return got
}

func TestMatchNode(t *testing.T) {
	tracing := []string{`^ctx, span := \w+\.Start\(ctx, .+\)$`, `^defer span\.End\(\)$`}
	debugLog := []string{`^(log|slog|logger)\.Debug\w*\(`}
	tests := []struct {
		name     string
		target   result.Target
		kinds    []string
		regex    []string
		notRegex []string
		limit    int
		want     []string
	}{
		{
			name:   "tracing",
			target: result.TargetStmt,
			kinds:  []string{"AssignStmt", "DeferStmt"},
			regex:  tracing,
			want: []string{
				`6: ctx, span := tracer.Start(ctx, "Service.Find", trace.WithAttributes(attribute.String("id", id)))`,
				`8: defer span.End()`,
			},
		},
		{
			name:   "node over the size limit is not matched",
			target: result.TargetStmt,
			kinds:  []string{"AssignStmt", "DeferStmt"},
			regex:  tracing,
			limit:  len("defer span.End()"),
			want:   []string{`8: defer span.End()`},
		},
		{
			name:     "debug-log",
			target:   result.TargetExpr,
			kinds:    []string{"CallExpr"},
			regex:    debugLog,
			notRegex: []string{`(?i)password|token|secret`},
			want:     []string{`10: log.Debug("find user", "id", id)`, `17: logger.Debug("cache hit")`},
		},
		{
			name:   "import aliases are not resolved",
			target: result.TargetExpr,
			kinds:  []string{"CallExpr"},
			regex:  []string{`^fmt\.`},
			want:   nil,
		},
		{
			name:   "the text keeps the alias as written",
			target: result.TargetExpr,
			kinds:  []string{"CallExpr"},
			regex:  []string{`^f\.`},
			want:   []string{`15: f.Println("aliased fmt")`},
		},
		{
			name:   "default stmt kinds skip the function body block",
			target: result.TargetStmt,
			regex:  []string{`^\{`, `^return u, nil$`},
			want:   []string{`18: return u, nil`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustNew(t, tt.target, tt.kinds, tt.regex, tt.notRegex)
			if got := matches(t, m, tt.limit); !slices.Equal(got, tt.want) {
				t.Errorf("matches:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestMatchNodeTextUnavailable(t *testing.T) {
	m := mustNew(t, result.TargetExpr, nil, []string{``}, nil)
	n := &ast.CallExpr{Fun: &ast.Ident{Name: "f"}}
	for _, f := range []*rule.File{nil, {}} {
		if _, ok := m.MatchNode(n, f); ok {
			t.Errorf("MatchNode with file %v matched, want no match", f)
		}
	}
}
