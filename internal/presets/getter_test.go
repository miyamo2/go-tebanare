package presets

import (
	"go/parser"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

func TestGetterMaxDepth(t *testing.T) {
	src := `
func (u *U) D1() int { return u.a }
func (u *U) D2() int { return u.a.b }
func (u *U) D3() int { return (u.a).b.c }
func (u *U) D4() int { return u.a.b.c.d }
`
	tests := []struct {
		settings string
		want     map[string]bool
	}{
		{"", map[string]bool{"D1": true, "D2": true, "D3": true, "D4": true}},
		{"max_depth: 1", map[string]bool{"D1": true, "D2": false, "D3": false, "D4": false}},
		{"max_depth: 3", map[string]bool{"D1": true, "D2": true, "D3": true, "D4": false}},
	}
	for _, tt := range tests {
		t.Run(tt.settings, func(t *testing.T) {
			checkResults(t, funcResults(t, "getter", tt.settings, src), tt.want)
		})
	}
}

// TestGetterMethodValue checks the exclusion of method values: the first
// selector names a method declared in the same file on the same base type,
// whatever the receiver form.
func TestGetterMethodValue(t *testing.T) {
	src := `
func (u User) Close() error { u.closed = true; return nil }
func (u *User) Closer() func() error { return u.Close }
func (u *User) Cfg() int { return u.cfg.Close }
func (s *Stack[T]) Pop() T { s.n--; return s.top }
func (s Stack[T]) Popper() func() T { return s.Pop }
func (s *Stack[T]) Field() int { return s.Pop.n }
func (o *Other) Pusher() func() T { return o.Pop }
`
	checkResults(t, funcResults(t, "getter", "", src), map[string]bool{
		"Closer": false, "Cfg": true, "Popper": false, "Field": false, "Pusher": true,
	})
}

// TestGetterShapes covers declarations that parse but do not compile.
func TestGetterShapes(t *testing.T) {
	src := `
func (u *U) Two() int { return u.a, u.b }
func (u *U) Empty() int { return }
func (u, v *U) Pair() int { return u.a }
`
	checkResults(t, funcResults(t, "getter", "", src), map[string]bool{
		"Two": false, "Empty": false, "Pair": false,
	})
}

func TestGetterCompileErrors(t *testing.T) {
	p, _ := Lookup("getter")
	s := p.NewSettings().(*GetterSettings)
	s.ExcludePaths = []string{"["}
	if _, err := p.Compile(s); err == nil {
		t.Error("Compile(exclude_paths [) succeeded")
	}
}

func TestGetterIncludeDoc(t *testing.T) {
	p, _ := Lookup("getter")
	for settings, want := range map[string]bool{"": true, "include_doc: true": true, "include_doc: false": false} {
		if r := compileYAML(t, p, settings); r.IncludeDoc != want {
			t.Errorf("settings %q: IncludeDoc = %v, want %v", settings, r.IncludeDoc, want)
		}
	}
}

func TestFieldChain(t *testing.T) {
	tests := []struct {
		expr  string
		depth int
		first string
		ok    bool
	}{
		{"u.a", 1, "a", true},
		{"u.a.b.c", 3, "a", true},
		{"((u).a).b", 2, "a", true},
		{"(u.a.b)", 2, "a", true},
		{"u", 0, "", false},
		{"v.a", 1, "a", false},
		{"u.a[0].b", 1, "b", false},
		{"u.a().b", 1, "b", false},
		{"(*u).a", 1, "a", false},
	}
	for _, tt := range tests {
		e, err := parser.ParseExpr(tt.expr)
		if err != nil {
			t.Fatal(err)
		}
		depth, first, ok := fieldChain(e, "u")
		if depth != tt.depth || first != tt.first || ok != tt.ok {
			t.Errorf("fieldChain(%s) = %d, %q, %v, want %d, %q, %v", tt.expr, depth, first, ok, tt.depth, tt.first, tt.ok)
		}
	}
}

var _ rule.FuncMatcher = (*getterMatcher)(nil)
