package sigpattern

import "testing"

func TestParseFuncType(t *testing.T) {
	tests := []struct{ src, want string }{
		{"func()", "func()"},
		{"func(int) error", "func(int) error"},
		{"func(a, b int, c ...string) (x int, err error)", "func(a int, b int, c ...string) (x int, err error)"},
		{"func(...) ...", "func($...) $..."},
		{"func(...) (...)", "func($...) $..."},
		{"func(_, ...)", "func($_, $...)"},
		{"func(..._)", "func(...$_)"},
		{"func(...int, ...)", "func(...int, $...)"},
		{"func(a, b)", "func(a, b)"},
		{"func(_, error)", "func($_, error)"},
		{"func(_ error)", "func(_ error)"},
		{"func(T, U)", "func($tp:T, U)"},
		{"func(p.T, q.U[int])", "func(p.T, q.U[int])"},
		{"func() (int)", "func() int"},
		{"func() ()", "func()"},
		{"func() func() int", "func() func() int"},
		{"func(func() ..., int)", "func(func() $..., int)"},
		{"func(a, ..., b int)", "func(a int, $..., b int)"},
		{"func(int,\n string,\n)", "func(int, string)"},
		{"func()\n int", "func() int"},
		// An identifier followed by '[' is a generic type unless a type
		// follows the brackets.
		{"func(List[int])", "func(List[int])"},
		{"func(List[_])", "func(List[$_])"},
		{"func(x List[int])", "func(x List[int])"},
		{"func(a []int)", "func(a []int)"},
		{"func(a [4]int)", "func(a [4]int)"},
		{"func(a [N]T)", "func(a [N]$tp:T)"},
		{"func(a [_]int)", "func(a [$_]int)"},
		{"func(T [2]int)", "func(T [2]int)"},
	}
	for _, tt := range tests {
		got, err := parseTypeText(tt.src, false, "T")
		if err != nil || got != tt.want {
			t.Errorf("%q: got %q, %v; want %q", tt.src, got, err, tt.want)
		}
	}
}

func TestParseFuncTypeErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"func(", "1:6: expected type, found 'EOF'"},
		{"func(,)", "1:6: expected type, found ','"},
		{"func(a int, string)", "1:13: missing parameter type"},
		{"func(a, b int, c)", "1:16: missing parameter type"},
		{"func(a int, []string)", "1:13: missing parameter name"},
		{"func(a int, ...string)", "1:13: missing parameter name"},
		{"func(a, b ...int)", "1:11: can only use ... with final parameter"},
		{"func(...int, int)", "1:6: can only use ... with final parameter"},
		{"func() (...int)", "1:9: invalid use of ..."},
		{"func(a int b int)", "1:12: missing ',' in parameter list"},
		{"func(pkg.T int)", "1:12: missing ',' in parameter list"},
		{"func(x ...)", "1:11: expected type, found ')'"},
		{"func(a [4])", "1:9: expected type, found 4"},
		// When the input ends in or right after the brackets, one element
		// reads as an array length and a list as type arguments.
		{"func(a [4", "1:10: expected ']', found 'EOF'"},
		{"func(a [N/2]", "1:13: expected type, found 'EOF'"},
		{"func(List[K, V]", "1:16: expected ')', found 'EOF'"},
		{"func(List[...]", "1:15: expected ')', found 'EOF'"},
		{"func[T any]()", "1:5: expected '(', found '['"},
		{"func() (int, ?)", "1:14: illegal character U+003F '?'"},
	}
	for _, tt := range tests {
		_, err := parseTypeText(tt.src, false)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q: got error %v, want %q", tt.src, err, tt.want)
		}
	}
}
