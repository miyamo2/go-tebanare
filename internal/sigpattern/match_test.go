package sigpattern

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// parseFile parses Go source that follows a package clause.
func parseFile(t *testing.T, src string) *ast.File {
	t.Helper()
	f, err := goparser.ParseFile(token.NewFileSet(), "x.go", "package p\n"+src, goparser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return f
}

// funcDecl parses a single function declaration.
func funcDecl(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f := parseFile(t, src)
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			return fd
		}
	}
	t.Fatalf("no function in %q", src)
	return nil
}

type matchCase struct {
	pattern string
	match   []string // declarations that match
	miss    []string // declarations that do not match
}

// checkMatches runs each case against its declarations, with the pattern
// and with its canonical form.
func checkMatches(t *testing.T, tests []matchCase) {
	t.Helper()
	for _, tt := range tests {
		p := mustParse(t, tt.pattern)
		q := roundTrip(t, p)
		for _, want := range []bool{true, false} {
			srcs := tt.match
			if !want {
				srcs = tt.miss
			}
			for _, src := range srcs {
				fd := funcDecl(t, src)
				if got := p.Match(fd); got != want {
					t.Errorf("%s\n\tMatch(%s) = %v, want %v", tt.pattern, src, got, want)
				}
				if got := q.Match(fd); got != want {
					t.Errorf("%s\n\tcanonical %s: Match(%s) = %v, want %v", tt.pattern, q, src, got, want)
				}
			}
		}
	}
}

// TestMatchExamples covers every row of the example table in the plan
// (section 4.3). Where the plan elides a signature with "(...)", the
// declarations spell one out.
func TestMatchExamples(t *testing.T) {
	checkMatches(t, []matchCase{
		{"func (_) String() string",
			[]string{"func (u User) String() string", "func (p *Point) String() string"},
			[]string{"func String() string", "func (u User) String() (string, error)"}},
		{"func (*Repository[T]) Save(context.Context, T) error",
			[]string{"func (r *Repository[E]) Save(ctx context.Context, v E) error"},
			[]string{"func (r *Repository[E]) Save(ctx context.Context, v *E) error"}},
		{"func (*Repository[_]) Find*(context.Context, ...) (_, error)",
			[]string{
				"func (r *Repository[T]) FindAll(ctx context.Context) ([]T, error)",
				"func (r *Repository[T]) FindByID(ctx context.Context, id string) (T, error)",
			},
			[]string{"func (r Repository[T]) FindAll(ctx context.Context) ([]T, error)"}},
		{"func (*Cache[_, _]) *",
			[]string{"func (c *Cache[K, V]) Get(k K) (V, bool)", "func (c *Cache[K, V]) Len() int"},
			[]string{"func (c *Cache[T]) Get(k T) T", "func (c *Cache[T]) Len() int"}},
		{"func (*Cache) *",
			[]string{"func (c *Cache) Len() int", "func (c *Cache[T]) Len() int", "func (c *Cache[K, V]) Get(k K) V"},
			nil},
		{"func Map[T, U any]([]T, func(T) U) []U",
			[]string{"func Map[A, B any](xs []A, f func(A) B) []B"},
			[]string{"func Map[A any, B comparable](xs []A, f func(A) B) []B"}},
		{"func Must[T](T, error) T",
			[]string{"func Must[V any](v V, err error) V", "func Must[V comparable](v V, err error) V"},
			[]string{"func Must[V any](v V, err error) *V"}},
		{"func New*",
			[]string{"func New() *T", "func NewServer(addr string) (*Server, error)", "func NewList[T any]() *List[T]"},
			[]string{"func (f *Factory) NewThing() *Thing", "func Create() *T"}},
		{"func (*Mock*) *",
			[]string{
				"func (m *MockUserRepo) Get(ctx context.Context, id string) (*User, error)",
				"func (mr *MockUserRepoMockRecorder) Get(ctx, id any) *gomock.Call",
			},
			[]string{"func (m MockUserRepo) Get(ctx context.Context, id string) (*User, error)"}},
		{"func (_) Close()",
			[]string{"func (c *Conn) Close()"},
			[]string{"func (f *File) Close() error"}},
		{"func Pair[T, U comparable](T, U)",
			[]string{"func Pair[A, B comparable](a A, b B)"},
			[]string{"func Pair[A any, B comparable](a A, b B)"}},
		{"func (_) Get(T) T",
			[]string{"func (c Cache) Get(k T) T"},
			[]string{"func (c *Cache[T]) Get(k T) T"}},
		{"func (_) Printf(string, _)",
			[]string{"func (l *Logger) Printf(format string, arg any)"},
			[]string{"func (l *Logger) Printf(format string, args ...any)"}},
		{"func (_) Printf(string, ..._)",
			[]string{"func (l *Logger) Printf(format string, args ...any)"},
			[]string{"func (l *Logger) Printf(format string, args []any)"}},
		{"func (**Handler) ServeHTTP(...)",
			[]string{"func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)"},
			[]string{"func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)"}},
		{"func (/Handler$/) ServeHTTP(...)",
			[]string{"func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)"},
			[]string{"func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)"}},
	})
}

// TestMatchAliases covers the table of what a single-file parse can decide
// (plan section 4.3): qualifiers are compared as written.
func TestMatchAliases(t *testing.T) {
	tests := []struct {
		src     string
		pattern string
		want    bool
		relaxed bool // MatchQualified with the file's import aliases
	}{
		{"import \"context\"\nfunc F(ctx context.Context)", "func F(context.Context)", true, true},
		{"import ctx2 \"context\"\nfunc F(ctx ctx2.Context)", "func F(context.Context)", false, true},
		{"import ctx2 \"context\"\nfunc F(ctx ctx2.Context)", "func F(ctx2.Context)", true, true},
		{"import . \"context\"\nfunc F(ctx Context)", "func F(context.Context)", false, false},
		{"import \"context\"\ntype Ctx = context.Context\nfunc F(ctx Ctx)", "func F(context.Context)", false, false},
		{"import r \"math/rand/v2\"\nfunc F(g *r.Rand)", "func F(*rand.Rand)", false, true},
	}
	for _, tt := range tests {
		f := parseFile(t, tt.src)
		fd := f.Decls[len(f.Decls)-1].(*ast.FuncDecl)
		p := mustParse(t, tt.pattern)
		if got := p.Match(fd); got != tt.want {
			t.Errorf("%q in %q: Match = %v, want %v", tt.pattern, tt.src, got, tt.want)
		}
		if got := p.MatchQualified(fd, rule.ImportAliases(f)); got != tt.relaxed {
			t.Errorf("%q in %q: MatchQualified = %v, want %v", tt.pattern, tt.src, got, tt.relaxed)
		}
	}
}

func TestMatchReceivers(t *testing.T) {
	checkMatches(t, []matchCase{
		{"func (_) M", []string{"func (t T) M()", "func (t *T) M()", "func (T) M()"}, []string{"func M()"}},
		{"func (*_) M", []string{"func (t *T) M()"}, []string{"func (t T) M()"}},
		{"func (T) M", []string{"func (t T) M()", "func (t T[K]) M()"}, []string{"func (t *T) M()", "func (u U) M()"}},
		{"func (*T) M", []string{"func (t *T) M()", "func (t *(T)) M()"}, []string{"func (t T) M()"}},
		{"func (**) M", []string{"func (x *Anything) M()"}, []string{"func (x Anything) M()"}},
		{"func (_ h*Handler) M", []string{"func (x hxHandler) M()"}, []string{"func (h *Handler) M()"}},
		{"func (h* Handler) M()", []string{"func (h*Handler) M() {}"}, []string{"func (h Handler) M() {}"}},
		{"func (*/^api/) M", []string{"func (h *apiHandler) M()"}, []string{"func (h *myapi) M()", "func (h apiHandler) M()"}},
		{"func (_) M", nil, []string{"func (x []int) M()"}},

		// Receiver type arguments declare type parameters by position.
		{"func (*Cache[K, V]) Get(K) V",
			[]string{"func (c *Cache[A, B]) Get(k A) B"},
			[]string{"func (c *Cache[A, B]) Get(k B) A", "func (c *Cache[A]) Get(k A) A"}},
		{"func (*Cache[..., V]) Get(...) V",
			[]string{"func (c *Cache[V]) Get() V", "func (c *Cache[K, W]) Get(k K) W"},
			[]string{"func (c *Cache[K, W]) Get(k K) K"}},
		{"func (*Pair[K, V]) M()", []string{"func (p *Pair[_, _]) M()"}, nil},
		{"func (*Cache[...]) Len() int", []string{"func (c *Cache) Len() int", "func (c *Cache[K, V]) Len() int"}, nil},
	})
}

func TestMatchTypeParams(t *testing.T) {
	checkMatches(t, []matchCase{
		{"func F[T, U]()", []string{"func F[A any, B comparable]()"}, []string{"func F[A any]()"}},
		{"func F[T any, U]()", []string{"func F[A any, B comparable]()"}, []string{"func F[A comparable, B any]()"}},
		{"func F[T, _, U any]()",
			[]string{"func F[A comparable, B int, C any]()"},
			[]string{"func F[A, B, C comparable]()"}},
		{"func F[...]()", []string{"func F()", "func F[T any]()"}, nil},
		{"func F()", []string{"func F[T any]()"}, nil},
		{"func F[T any](T)", []string{"func F[X any](x X)"}, []string{"func F(x T)", "func F[X any](x T)"}},
		{"func Index[S ~[]E, E comparable](S, E) int",
			[]string{"func Index[X ~[]Y, Y comparable](s X, v Y) int"},
			[]string{"func Index[X ~[]Y, Y comparable](s X, v X) int", "func Index[X ~[]int, Y comparable](s X, v Y) int"}},
		// The binding of T depends on the alignment, which the signature
		// decides.
		{"func F[..., T comparable, ...](T)",
			[]string{"func F[A any, B comparable, C any](x B)"},
			[]string{"func F[A any, B comparable, C any](x C)"}},
		{"func F[T interface{ ~int | ~string }](T)",
			[]string{"func F[X interface{ ~int | ~string }](x X)"},
			[]string{"func F[X ~int | ~string](x X)"}},
		{"func F[T ~int | ~string](T)", []string{"func F[X ~int | ~string](x X)"}, nil},
	})
}

func TestMatchParams(t *testing.T) {
	checkMatches(t, []matchCase{
		{"func F(_)", []string{"func F(x int)"}, []string{"func F(x ...int)", "func F()", "func F(a, b int)"}},
		{"func F(...)", []string{"func F()", "func F(x ...int)", "func F(a int, b ...string)"}, nil},
		{"func F(...int)", []string{"func F(x ...int)"}, []string{"func F(x ...string)", "func F(x []int)"}},
		{"func F(_, error)", []string{"func F(a int, err error)"}, []string{"func F(err error)"}},
		{"func F(_ error)", []string{"func F(err error)"}, []string{"func F(a int, err error)"}},
		{"func F(int, int)", []string{"func F(a, b int)"}, []string{"func F(a int)"}},
		{"func F(..., error)", []string{"func F(err error)", "func F(a, b int, err error)"}, []string{"func F(err error, a int)"}},
		{"func F() ...", []string{"func F()", "func F() (int, error)"}, nil},
		{"func F() (_, ...)", []string{"func F() int", "func F() (n int, err error)"}, []string{"func F()"}},
		{"func F", []string{"func F(a int) error"}, []string{"func G()"}},
	})
}

func TestMatchTypes(t *testing.T) {
	checkMatches(t, []matchCase{
		{"func F(any)", []string{"func F(x any)", "func F(x interface{})"}, []string{"func F(x interface{ M() })"}},
		{"func F(interface{})", []string{"func F(x any)"}, nil},
		{"func F((T))", []string{"func F(x T)", "func F(x ((T)))"}, nil},
		{"func F([N/2]byte)", []string{"func F(b [N/2]byte)", "func F(b [N / 2]byte)"}, []string{"func F(b [N]byte)"}},
		{"func F([_]byte)", []string{"func F(b [4]byte)"}, []string{"func F(b []byte)"}},
		{"func F(<-chan _)", []string{"func F(c <-chan int)"}, []string{"func F(c chan int)", "func F(c chan<- int)"}},
		{"func F(map[string]_)", []string{"func F(m map[string][]int)"}, []string{"func F(m map[int]string)"}},
		{"func F(func(context.Context) error)",
			[]string{"func F(f func(ctx context.Context) error)"},
			[]string{"func F(f func(ctx context.Context))"}},
		{"func F(interface{ Read([]byte) (int, error) })",
			[]string{"func F(r interface{ Read(p []byte) (n int, err error) })"},
			[]string{"func F(r interface{ Write(p []byte) (n int, err error) })"}},
		{"func F(struct{ a, b int `json:\"a\"` })",
			[]string{"func F(s struct{ a int `json:\"a\"`; b int `json:\"a\"` })"},
			[]string{"func F(s struct{ a, b int })"}},
		{"func F(List[int])", []string{"func F(l List[int])"}, []string{"func F(l List)", "func F(l List[string])"}},
		{"func F(List)", []string{"func F(l List)"}, []string{"func F(l List[int])"}},
		{"func F(List[...])", []string{"func F(l List)", "func F(l List[int, string])"}, nil},
	})
}

func TestMatchNil(t *testing.T) {
	var p *Pattern
	if p.Match(&ast.FuncDecl{}) {
		t.Error("nil pattern matched")
	}
	if mustParse(t, "func *").Match(nil) || mustParse(t, "func *").Match(&ast.FuncDecl{}) {
		t.Error("matched an incomplete declaration")
	}
}
