package sigpattern

import (
	"bytes"
	"errors"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Pattern {
	t.Helper()
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return p
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"", "1:1: expected 'func', found 'EOF'"},
		{"fun F", "1:1: expected 'func', found fun"},
		{"func", "1:5: expected name, found 'EOF'"},
		{"func (*Repository[_]) Find*(, ...) (_, error)", "1:29: expected type, found ','"},
		{"func ()", "1:7: expected receiver type, found ')'"},
		{"func (*) F", "1:8: expected receiver type, found ')'"},
		{"func (T F", "1:10: expected ')', found 'EOF'"},
		{"func (a b c) F", "1:11: expected ')', found c"},
		{"func (_[T]) F", "1:8: expected ')', found '['"},
		// Go reads "h*Handler" as a pointer receiver named h; a glob reads
		// it as one value receiver type name.
		{"func (h*Handler) M()", `1:8: ambiguous receiver: write "h *Handler" for a pointer receiver or "_ h*Handler" for a value receiver glob`},
		{"func (h**[K]) M()", `1:8: ambiguous receiver: write "h **" for a pointer receiver or "_ h**" for a value receiver glob`},
		{"func (*Cache[]) F", "1:14: expected type parameter name, found ']'"},
		{"func (*Cache[K, *V]) F", "1:17: expected type parameter name, found '*'"},
		{"func (*Pair[K, K]) F", "1:16: type parameter K redeclared"},
		{"func (T) F[U any]()", "1:11: methods cannot have type parameters"},
		{"func F[]()", "1:8: empty type parameter list"},
		{"func F[T, T any]()", "1:11: type parameter T redeclared"},
		{"func F[_ any]()", "1:10: expected ']', found any"},
		{"func F[... any]()", "1:12: expected ']', found any"},
		{"func F[*T]()", "1:8: expected type parameter, found '*'"},
		{"func F[T any", "1:13: expected ']', found 'EOF'"},
		{"func F int", "1:8: expected '(', found int"},
		{"func a.b*", "1:7: expected '(', found '.'"},
		{"func F() string int", "1:17: expected 'EOF', found int"},
		{"func F() ...int", "1:13: expected 'EOF', found int"},
		{"func /abc", "1:6: regexp not terminated"},
		{"func /a(/", "1:6: error parsing regexp: missing closing ): `a(`"},
		{"func (*/*H/) M", "1:8: error parsing regexp: missing argument to repetition operator: `*`"},
		{"func /" + strings.Repeat("(", 201) + "/", `1:6: the expression nests too deeply (estimated depth 1210, more than the limit of 600)`},
		// go/parser reports more than ten errors here only with AllErrors.
		{"func F([f(a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn)]int)", "1:12: missing ',' before newline in argument list"},
		{"func Get$", "1:9: illegal character U+0024 '$'"},
		{"func Get*0x", "1:12: hexadecimal literal has no digits"},
		{"func F?() /* x", "1:11: comment not terminated"},
		{"func F?\xef\xbb\xbf", "1:8: illegal byte order mark"},
		{"func F(x [1]", "1:13: expected type, found 'EOF'"},
		{"func F(x [1", "1:12: expected ']', found 'EOF'"},
		{"func F(x [...]int)", "1:11: invalid use of [...] array (outside a composite literal)"},
		{"func F(\n\tx int,\n\ty ?)", "3:4: illegal character U+003F '?'"},
	}
	for _, tt := range tests {
		p, err := Parse(tt.src)
		if p != nil || err == nil || err.Error() != tt.want {
			t.Errorf("Parse(%q) = %v, %v; want error %q", tt.src, p, err, tt.want)
			continue
		}
		if e := (*Error)(nil); !errors.As(err, &e) {
			t.Errorf("Parse(%q) error is %T, want *Error", tt.src, err)
		}
	}
}

func TestParseReceiver(t *testing.T) {
	tests := []struct {
		src              string
		anyType, pointer bool
		name             string // glob or regexp text
		regexp           bool
		args             int // -1: omitted
	}{
		{src: "func (_) M", anyType: true, args: -1},
		{src: "func (*_) M", anyType: true, pointer: true, args: -1},
		{src: "func (x _) M", anyType: true, args: -1},
		{src: "func (_ *T) M", pointer: true, name: "T", args: -1},
		{src: "func (T) M", name: "T", args: -1},
		{src: "func (h *Handler) M", pointer: true, name: "Handler", args: -1},
		{src: "func (**Handler) M", pointer: true, name: "*Handler", args: -1},
		{src: "func (**) M", pointer: true, name: "*", args: -1},
		{src: "func (*Mock*) M", pointer: true, name: "Mock*", args: -1},
		{src: "func (Mock*) M", name: "Mock*", args: -1},
		{src: "func (_ h*Handler) M", name: "h*Handler", args: -1},
		{src: "func (h* Handler) M", pointer: true, name: "Handler", args: -1},
		{src: "func (h*/Handler$/) M", pointer: true, name: "Handler$", regexp: true, args: -1},
		{src: "func (Mock*[K]) M", name: "Mock*", args: 1},
		{src: "func (/Handler$/) M", name: "Handler$", regexp: true, args: -1},
		{src: "func (*/Handler$/) M", pointer: true, name: "Handler$", regexp: true, args: -1},
		{src: "func (h */a\\/b/) M", pointer: true, name: "a/b", regexp: true, args: -1},
		{src: "func (*Cache[K, _, ...]) M", pointer: true, name: "Cache", args: 3},
		{src: "func (c Cache[K,]) M", name: "Cache", args: 1},
	}
	for _, tt := range tests {
		r := mustParse(t, tt.src).recv
		args := -1
		if r.args != nil {
			args = len(r.args.List)
		}
		if r.anyType != tt.anyType || r.pointer != tt.pointer || r.name.text != tt.name ||
			r.name.regexp != tt.regexp || args != tt.args {
			t.Errorf("%q: got %+v (args %d)", tt.src, *r, args)
		}
	}
	if p := mustParse(t, "func F"); p.recv != nil {
		t.Error("func F has a receiver")
	}
}

func TestParseTypeParamGroups(t *testing.T) {
	tests := []struct {
		src  string
		want string // name:constraint for each element; "?" is any constraint
	}{
		{"func F[T, U any]", "T:any U:any"},
		{"func F[T, U]", "T:? U:?"},
		{"func F[T any, U]", "T:any U:?"},
		{"func F[T, _, U any]", "T:? $_:? U:any"},
		{"func F[T, ..., U comparable]", "T:? $...:? U:comparable"},
		{"func F[S ~[]E, E any]", "S:~[]$tp:E E:any"},
		{"func F[T interface{ M() T }]", "T:interface { M() $tp:T }"},
		{"func F[...]", "$...:?"},
	}
	for _, tt := range tests {
		var got []string
		for _, f := range mustParse(t, tt.src).tparams.List {
			c := "?"
			if f.Type != nil {
				var b bytes.Buffer
				if err := printer.Fprint(&b, token.NewFileSet(), f.Type); err != nil {
					t.Fatal(err)
				}
				c = strings.Join(strings.Fields(b.String()), " ")
			}
			got = append(got, f.Names[0].Name+":"+c)
		}
		if g := strings.Join(got, " "); g != tt.want {
			t.Errorf("%q: got %q, want %q", tt.src, g, tt.want)
		}
	}
}

func TestParseOmissions(t *testing.T) {
	p := mustParse(t, "func (*Mock) *")
	if p.tparams != nil || p.sig != nil || p.recv.args != nil {
		t.Error("func (*Mock) * does not leave everything open")
	}
	p = mustParse(t, "func (_) Close()")
	if p.sig == nil || p.sig.Results != nil || len(p.sig.Params.List) != 0 {
		t.Error("func (_) Close() does not require no parameters and no results")
	}
	p = mustParse(t, "func (_) Close() ...")
	if r := p.sig.Results; r == nil || len(r.List) != 1 {
		t.Error("func (_) Close() ... does not accept any results")
	}
}

func TestNameRegexps(t *testing.T) {
	p := mustParse(t, `func (*/Mock$/) /^(Get|Set)\/x/`)
	got := p.NameRegexps()
	if len(got) != 2 || got[0] != "Mock$" || got[1] != "^(Get|Set)/x" {
		t.Errorf("NameRegexps() = %q", got)
	}
	if got := mustParse(t, "func (*Mock) Get*").NameRegexps(); len(got) != 0 {
		t.Errorf("NameRegexps() = %q, want none", got)
	}
}
