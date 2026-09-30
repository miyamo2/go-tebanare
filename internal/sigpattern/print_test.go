package sigpattern

import "testing"

// roundTrip checks that the canonical form of p parses to a pattern with
// the same canonical form, and returns that pattern.
func roundTrip(t *testing.T, p *Pattern) *Pattern {
	t.Helper()
	s := p.String()
	q, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) of the canonical form: %v", s, err)
	}
	if got := q.String(); got != s {
		t.Fatalf("canonical form is not stable: %q, then %q", s, got)
	}
	return q
}

func TestString(t *testing.T) {
	tests := []struct{ src, want string }{
		// Receivers, names, and omissions.
		{"func (u User) String() string", "func (User) String() string"},
		{"func (_) Close() ...", "func (_) Close() ..."},
		{"func (x *_) Close() (...)", "func (*_) Close() ..."},
		{"func (h **Handler)ServeHTTP(...)", "func (**Handler) ServeHTTP(...)"},
		{"func (**) *", "func (**) *"},
		{"func (x h*Handler) M", "func (_ h*Handler) M"},
		{"func (x Mock**) M", "func (_ Mock**) M"},
		{"func (x Mock*) M", "func (Mock*) M"},
		{"func (h* Handler) M", "func (*Handler) M"},
		{"func (*/a\\/b/[K, _, ...]) /^Get/", "func (*/a\\/b/[K, _, ...]) /^Get/"},
		{"func (c Cache[K,]) M", "func (Cache[K]) M"},
		{"func Get?", "func Get?"},
		{"func *map*", "func *map*"},
		{"func (_) *() _", "func (_) *() _"},

		// Type parameters: each element gets its own constraint.
		{"func Map[T, U any]([]T, func(T) U) []U", "func Map[T any, U any]([]T, func(T) U) []U"},
		{"func F[T, _, U any, ...]", "func F[T, _, U any, ...]"},
		{"func F[S ~[]E, E any](S) E", "func F[S ~[]E, E any](S) E"},
		{"func F[T interface{ ~int | ~string }]()", "func F[T interface{ ~int | ~string }]()"},

		// Parameters and results: names are dropped, grouped names are
		// expanded, parentheses stay.
		{"func F(ctx context.Context, a, b int, opts ...Option) (n int, err error)",
			"func F(context.Context, int, int, ...Option) (int, error)"},
		{"func F(_ error)", "func F(error)"},
		{"func F(_, error)", "func F(_, error)"},
		{"func F(a, ..., b int)", "func F(int, ..., int)"},
		{"func F() (int)", "func F() int"},
		{"func F() ((int))", "func F() ((int))"},
		{"func F() ()", "func F()"},
		{"func F((int), *(T))", "func F((int), *(T))"},
		{"func F() func() ...", "func F() func() ..."},
		{"func F(func(), ...)", "func F(func(), ...)"},

		// Types.
		{"func F(x [N/2]T, y [_]int, z []int, m map[K]V)", "func F([N / 2]T, [_]int, []int, map[K]V)"},
		{"func F(chan (<-chan int), chan <-chan int, <-chan <-chan int)",
			"func F(chan (<-chan int), chan<- chan int, <-chan <-chan int)"},
		{"func F(List[...], pkg.Map[_, int], List[int,])", "func F(List[...], pkg.Map[_, int], List[int])"},
		{"func F(interface{}, interface{ M(int) error; io.Reader }, interface{ M() ... })",
			"func F(interface{}, interface{ M(int) error; io.Reader }, interface{ M() ... })"},
		{"func F(struct{}, struct {\n\ta, b int `json:\"a\"`\n\t*pkg.T\n\t_ int\n})",
			"func F(struct{}, struct{ a, b int `json:\"a\"`; *pkg.T; _ int })"},
		{"func F(x [len(func(){ a(); b() })]int)", "func F([len(func() {\n\ta()\n\tb()\n})]int)"},
	}
	for _, tt := range tests {
		p := mustParse(t, tt.src)
		if got := p.String(); got != tt.want {
			t.Errorf("%q: String() = %q, want %q", tt.src, got, tt.want)
		}
		roundTrip(t, p)
	}
}
