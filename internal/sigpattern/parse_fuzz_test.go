package sigpattern

import (
	"errors"
	"flag"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/fuzzvec"
)

// fuzzDecls are the declarations every fuzzed pattern is matched against.
const fuzzDecls = `
func F() {}
func New[T any](x T, opts ...Option) (*List[T], error) { return nil, nil }
func (u User) String() string { return "" }
func (c *Cache[K, V]) Get(ctx context.Context, k K) (V, bool) { var v V; return v, false }
func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
func Index[S ~[]E, E comparable](s S, v E) int { return 0 }
func G(a [N/2]int, m map[string]<-chan any, f func(int) error, i interface{ M() }, s struct{ a int "t" }) {}
`

var update = flag.Bool("update", false, "rewrite testvectors/fuzz/sigpattern.json")

// fuzzParseSeeds are the seed inputs of FuzzParse.
var fuzzParseSeeds = []string{
	"func (_) String() string",
	"func (*Repository[T]) Save(context.Context, T) error",
	"func (*Repository[_]) Find*(context.Context, ...) (_, error)",
	"func (*Cache[_, _]) *",
	"func Map[T, U any]([]T, func(T) U) []U",
	"func Must[T](T, error) T",
	"func New*",
	"func (*Mock*) *",
	"func (_) Close() ...",
	"func Pair[T, U comparable](T, U)",
	"func (_) Printf(string, ..._)",
	"func (**Handler) ServeHTTP(...)",
	"func (/Handler$/) ServeHTTP(...)",
	`func (h */a\/b/[K, ...]) /^Get/`,
	"func Get?[T, _, U any, ...](T, ..., U) (_, error)",
	"func F[S ~[]E, E interface{ ~int | ~string; M() E }](S) E",
	"func F(x [N/2]int, y [_]int, m map[K]V, c chan (<-chan int), d chan<- <-chan int)",
	"func F(struct{ a, b int `json:\"a\"`; *pkg.T }, interface{ io.Reader; M() ... })",
	"func F(a, ..., b int) ((int))",
	"func F(List[...], pkg.Map[_, int], func() ..., func(), ...)",
	"func F(\n\tx int,\n\ty string,\n) (\n\tint, error)",
	"func (*Repository[_]) Find*(, ...) (_, error)",
	"func F(x [len(\"]\")]int)",
	"func (h*Handler) M",
	"func (_ h*Handler) M",
	"func (h* Handler) M",
	"func F?\xef\xbb\xbf",
	"func /a/\xef\xbb\xbf",
	"func F(x [1]",
	"func *map*",
	// More than ten errors in an array length, where the default mode of
	// go/parser gives up with a panic.
	"func F([f(a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn)]int)",
	// Deeper than the nesting limit of regexp/syntax.
	"func (/" + strings.Repeat("(", 1001) + "a" + strings.Repeat(")", 1001) + "/) F()",
}

// TestFuzzVectors keeps testvectors/fuzz/sigpattern.json in step with the
// seeds and the corpus of FuzzParse. Each input becomes a config with one
// func rule, and the engine tests replay the configs through the wasm
// build and analyze fuzzDecls with each of them.
func TestFuzzVectors(t *testing.T) {
	corpus, err := fuzzvec.ReadCorpus(filepath.Join("testdata", "fuzz", "FuzzParse"))
	if err != nil {
		t.Fatal(err)
	}
	var inputs [][]byte
	for _, s := range fuzzParseSeeds {
		inputs = append(inputs, []byte(s))
	}
	f := fuzzvec.File{Source: "package p\n" + fuzzDecls}
	for _, b := range append(inputs, corpus...) {
		cfg := fuzzvec.PatternConfig(string(b))
		var doc struct {
			Rules []struct{ Func string }
		}
		if err := yaml.Unmarshal([]byte(cfg), &doc); err != nil || len(doc.Rules) != 1 ||
			doc.Rules[0].Func != strings.ToValidUTF8(string(b), "\uFFFD") {
			t.Fatalf("PatternConfig(%q) = %q does not hold the pattern: %+v, %v", b, cfg, doc, err)
		}
		f.Inputs = append(f.Inputs, fuzzvec.NewInput([]byte(cfg)))
	}
	fuzzvec.Check(t, "sigpattern", f, *update)
}

func FuzzParse(f *testing.F) {
	for _, s := range fuzzParseSeeds {
		f.Add(s)
	}
	file := parseFileNoT(fuzzDecls)
	quals := map[string]string{"http": "net", "context": "ctx"}
	f.Fuzz(func(t *testing.T, src string) {
		p, err := Parse(src)
		if err != nil {
			var e *Error
			if !errors.As(err, &e) || p != nil {
				t.Fatalf("Parse(%q) = %v, %T", src, p, err)
			}
			if e.Offset < 0 || e.Offset > len(src) || e.Line < 1 || e.Column < 1 {
				t.Fatalf("Parse(%q): bad position %+v", src, e)
			}
			if c := e.Caret(""); strings.Count(c, "\n") != 1 {
				t.Fatalf("Parse(%q): caret %q is not two lines", src, c)
			}
			return
		}
		s := p.String()
		q, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q) canonical form %q: %v", src, s, err)
		}
		if got := q.String(); got != s {
			t.Fatalf("Parse(%q): canonical form %q is not stable: %q", src, s, got)
		}
		for _, d := range file.Decls {
			fd := d.(*ast.FuncDecl)
			if p.Match(fd) != q.Match(fd) {
				t.Fatalf("Parse(%q): %q and its canonical form %q disagree on %s", src, src, s, fd.Name.Name)
			}
			p.MatchQualified(fd, quals)
		}
		p.NameRegexps()
	})
}

// parseFileNoT is parseFile for callers without a *testing.T. It panics on
// a syntax error in the test's own source.
func parseFileNoT(src string) *ast.File {
	f, err := goparser.ParseFile(token.NewFileSet(), "x.go", "package p\n"+src, goparser.SkipObjectResolution)
	if err != nil {
		panic(err)
	}
	return f
}
