package sigpattern

import "testing"

func TestParseBody(t *testing.T) {
	tests := []struct{ src, want string }{
		{"interface{}", "interface { }"},
		{"interface{ M() }", "interface { M() }"},
		{"interface{ M(int) error; io.Reader; }", "interface { M(int) error io.Reader }"},
		{"interface{ ~int | ~string }", "interface { ~int | ~string }"},
		{"interface{ int | T; comparable }", "interface { int | $tp:T comparable }"},
		{"interface{ M() T }", "interface { M() $tp:T }"},
		{"interface{ M() ... }", "interface { M() $... }"},
		{"interface {\n\tM()\n\tN() (int,\n\t\terror)\n}", "interface { M() N() (int, error) }"},
		{"struct{}", "struct { }"},
		{"struct{ a, b int; c string \"tag\" }", "struct { a, b int c string \"tag\" }"},
		{"struct{ io.Reader; *T; *pkg.U; List[int] }", "struct { io.Reader *$tp:T *pkg.U List[int] }"},
		{"struct{ a [4]int; b []int; _ int }", "struct { a [4]int b []int _ int }"},
		{"struct{ f func()\n g int `x`\n}", "struct { f func() g int `x` }"},
		{"struct{ _ }", "struct { $_ }"},
	}
	for _, tt := range tests {
		got, err := parseTypeText(tt.src, false, "T")
		if err != nil || got != tt.want {
			t.Errorf("%q: got %q, %v; want %q", tt.src, got, err, tt.want)
		}
	}
}

func TestParseBodyErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"interface{ M() N() }", "1:17: expected ';', found '('"},
		{"interface{ M()", "1:15: expected '}', found 'EOF'"},
		{"interface{ ; }", "1:12: expected type, found ';'"},
		{"struct{ a int b int }", "1:15: expected ';', found b"},
		{"struct{ ... }", "1:9: expected field name or embedded type, found '...'"},
		{"struct{ a, }", "1:12: expected 'IDENT', found '}'"},
		{"struct{ *[]int }", "1:10: expected 'IDENT', found '['"},
	}
	for _, tt := range tests {
		_, err := parseTypeText(tt.src, false)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q: got error %v, want %q", tt.src, err, tt.want)
		}
	}
}
