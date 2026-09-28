package sigpattern

import (
	"bytes"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// parseTypeText parses src as one type with the given type parameters
// declared. It returns the type as printed by go/printer with whitespace
// collapsed, so sentinels show as "$_", "$...", and "$tp:T".
func parseTypeText(src string, union bool, declared ...string) (string, error) {
	p := newParser(src)
	for _, d := range declared {
		p.declared[d] = true
	}
	x := p.parseType(union)
	p.expect(token.EOF)
	if p.err != nil {
		return "", p.err
	}
	var b bytes.Buffer
	if err := printer.Fprint(&b, token.NewFileSet(), x); err != nil {
		return "", err
	}
	return strings.Join(strings.Fields(b.String()), " "), nil
}

func TestParseType(t *testing.T) {
	tests := []struct {
		src, want string
		union     bool
	}{
		{src: "int", want: "int"},
		{src: "_", want: "$_"},
		{src: "T", want: "$tp:T"},
		{src: "context.Context", want: "context.Context"},
		{src: "pkg.List[int, _]", want: "pkg.List[int, $_]"},
		{src: "List[...]", want: "List[$...]"},
		{src: "List[int,]", want: "List[int]"},
		{src: "Map[K, List[V]]", want: "Map[K, List[V]]"},
		{src: "*[]*T", want: "*[]*$tp:T"},
		{src: "[4]int", want: "[4]int"},
		{src: "[N/2]T", want: "[N / 2]$tp:T"},
		{src: "[len(\"]\")]int", want: "[len(\"]\")]int"},
		{src: "[_]int", want: "[$_]int"},
		{src: "[ _ ]int", want: "[$_]int"},
		{src: "map[string][]_", want: "map[string][]$_"},
		{src: "chan int", want: "chan int"},
		{src: "chan<- int", want: "chan<- int"},
		{src: "<-chan int", want: "<-chan int"},
		{src: "chan <-chan int", want: "chan<- chan int"},
		{src: "chan (<-chan int)", want: "chan (<-chan int)"},
		{src: "<-chan <-chan int", want: "<-chan <-chan int"},
		{src: "(int)", want: "(int)"},
		{src: "~int | ~string | float64", want: "~int | ~string | float64", union: true},
		{src: "(~int | T)", want: "(~int | $tp:T)", union: true},
		{src: "[]int |\n string", want: "[]int | string", union: true},
	}
	for _, tt := range tests {
		got, err := parseTypeText(tt.src, tt.union, "T")
		if err != nil || got != tt.want {
			t.Errorf("%q: got %q, %v; want %q", tt.src, got, err, tt.want)
		}
	}
}

func TestParseTypeErrors(t *testing.T) {
	tests := []struct {
		src   string
		union bool
		want  string
	}{
		{src: "", want: "1:1: expected type, found 'EOF'"},
		{src: "~int", want: "1:1: expected type, found '~'"},
		{src: "int | string", want: "1:5: expected 'EOF', found '|'"},
		{src: "*~int", union: true, want: "1:2: expected type, found '~'"},
		{src: "[...]int", want: "1:2: invalid use of [...] array (outside a composite literal)"},
		{src: "[4", want: "1:3: expected ']', found 'EOF'"},
		{src: "[4)]int", want: "1:3: expected ']', found ')'"},
		{src: "[a b]int", want: "1:4: expected 'EOF', found b"},
		{src: "[_]", want: "1:4: expected type, found 'EOF'"},
		{src: "List[]", want: "1:6: expected type, found ']'"},
		{src: "List[int", want: "1:9: expected ']', found 'EOF'"},
		{src: "map[int", want: "1:8: expected ']', found 'EOF'"},
		{src: "<-int", want: "1:3: expected 'chan', found int"},
		{src: "pkg.*", want: "1:5: expected 'IDENT', found '*'"},
		{src: "T[int]", want: "1:2: expected 'EOF', found '['"},
		{src: "Get?", want: "1:4: illegal character U+003F '?'"},
		{src: "[0x]int", want: "1:4: hexadecimal literal has no digits"},
		{src: strings.Repeat("*", maxDepth) + "int", want: "1:101: pattern is nested too deeply"},
		{src: "int" + strings.Repeat("|int", maxDepth), union: true, want: "1:401: pattern is nested too deeply"},
		{src: "[" + strings.Repeat("1+", maxLengthTokens) + "1]int", want: "1:2: array length is too long"},
	}
	for _, tt := range tests {
		_, err := parseTypeText(tt.src, tt.union, "T")
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q: got error %v, want %q", tt.src, err, tt.want)
		}
	}
}
