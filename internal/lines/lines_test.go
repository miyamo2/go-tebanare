package lines

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func r(start, end int) Range { return Range{Start: start, End: end} }

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   []Range
		want []Range
	}{
		{"nil", nil, nil},
		{"only invalid", []Range{r(0, 3), r(5, 4), r(-2, -1)}, nil},
		{"single", []Range{r(3, 5)}, []Range{r(3, 5)}},
		{"single line", []Range{r(7, 7)}, []Range{r(7, 7)}},
		{"sorts", []Range{r(10, 12), r(1, 2), r(5, 6)}, []Range{r(1, 2), r(5, 6), r(10, 12)}},
		{"merges overlap", []Range{r(1, 5), r(3, 8)}, []Range{r(1, 8)}},
		{"merges adjacent", []Range{r(1, 2), r(3, 4)}, []Range{r(1, 4)}},
		{"keeps gap of one line", []Range{r(1, 2), r(4, 5)}, []Range{r(1, 2), r(4, 5)}},
		{"contained", []Range{r(1, 10), r(3, 4)}, []Range{r(1, 10)}},
		{"same start", []Range{r(2, 9), r(2, 3)}, []Range{r(2, 9)}},
		{"duplicates", []Range{r(4, 6), r(4, 6)}, []Range{r(4, 6)}},
		{"chain", []Range{r(5, 6), r(1, 2), r(3, 4), r(7, 7)}, []Range{r(1, 7)}},
		{"drops invalid among valid", []Range{r(0, 2), r(3, 4), r(9, 8)}, []Range{r(3, 4)}},
		{"max int", []Range{r(math.MaxInt, math.MaxInt), r(1, math.MaxInt-1)}, []Range{r(1, math.MaxInt)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := slices.Clone(tt.in)
			got := Normalize(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Normalize(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if !slices.Equal(tt.in, in) {
				t.Errorf("Normalize modified its input: %v, was %v", tt.in, in)
			}
		})
	}
}

func TestMergeAcrossBlank(t *testing.T) {
	// Lines 3, 4, 8, and 12 are blank. Every other line holds code.
	src := []byte("a\nb\n\n \t\nc\nd\ne\n\nf\ng\nh\n\ni\n")
	blank := BlankLines(src)
	tests := []struct {
		name  string
		in    []Range
		blank func(int) bool
		want  []Range
	}{
		{"blank gap", []Range{r(1, 2), r(5, 5)}, blank, []Range{r(1, 5)}},
		{"one blank line", []Range{r(7, 7), r(9, 9)}, blank, []Range{r(7, 9)}},
		{"gap with code", []Range{r(5, 5), r(7, 7)}, blank, []Range{r(5, 5), r(7, 7)}},
		{"gap mixes blank and code", []Range{r(1, 2), r(7, 7)}, blank, []Range{r(1, 2), r(7, 7)}},
		{"chain across blanks", []Range{r(1, 2), r(5, 5), r(11, 11), r(13, 13), r(9, 9)}, blank, []Range{r(1, 5), r(9, 9), r(11, 13)}},
		{"overlap and blank", []Range{r(1, 1), r(2, 2), r(5, 7), r(6, 7), r(9, 9)}, blank, []Range{r(1, 9)}},
		{"gap past end of file", []Range{r(13, 13), r(20, 21)}, blank, []Range{r(13, 13), r(20, 21)}},
		{"nil blank", []Range{r(1, 2), r(5, 5)}, nil, []Range{r(1, 2), r(5, 5)}},
		{"single", []Range{r(3, 3)}, blank, []Range{r(3, 3)}},
		{"empty", nil, blank, nil},
		{"invalid dropped", []Range{r(0, 1), r(5, 5)}, blank, []Range{r(5, 5)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeAcrossBlank(tt.in, tt.blank)
			if !slices.Equal(got, tt.want) {
				t.Errorf("MergeAcrossBlank(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	rs := []Range{r(2, 4), r(7, 7), r(10, 20)}
	for line := -1; line <= 22; line++ {
		want := (line >= 2 && line <= 4) || line == 7 || (line >= 10 && line <= 20)
		if got := Contains(rs, line); got != want {
			t.Errorf("Contains(%v, %d) = %v, want %v", rs, line, got, want)
		}
	}
	if Contains(nil, 1) {
		t.Error("Contains(nil, 1) = true")
	}
}

// rs builds ranges from start, end pairs.
func rs(v ...int) []Range {
	var out []Range
	for i := 0; i+1 < len(v); i += 2 {
		out = append(out, r(v[i], v[i+1]))
	}
	return out
}

func TestSetOperations(t *testing.T) {
	const m = math.MaxInt
	tests := []struct {
		name                     string
		a, b, union, inter, diff []Range
	}{
		{"empty", nil, nil, nil, nil, nil},
		{"b empty", rs(1, 3), nil, rs(1, 3), nil, rs(1, 3)},
		{"a empty", nil, rs(1, 3), rs(1, 3), nil, nil},
		{"disjoint", rs(1, 2), rs(5, 6), rs(1, 2, 5, 6), nil, rs(1, 2)},
		{"adjacent", rs(1, 2), rs(3, 4), rs(1, 4), nil, rs(1, 2)},
		{"overlap", rs(1, 5), rs(4, 8), rs(1, 8), rs(4, 5), rs(1, 3)},
		{"b inside a", rs(1, 10), rs(3, 4, 7, 7), rs(1, 10), rs(3, 4, 7, 7), rs(1, 2, 5, 6, 8, 10)},
		{"a inside b", rs(3, 4, 7, 7), rs(1, 10), rs(1, 10), rs(3, 4, 7, 7), nil},
		{"b spans several a", rs(1, 3, 5, 7, 9, 11), rs(2, 10), rs(1, 11), rs(2, 3, 5, 7, 9, 10), rs(1, 1, 11, 11)},
		{"unnormalized input", rs(5, 6, 1, 3, 2, 4, 0, 9), rs(4, 4, 3, 3), rs(1, 6), rs(3, 4), rs(1, 2, 5, 6)},
		{"b reaches max int", rs(1, m), rs(5, m), rs(1, m), rs(5, m), rs(1, 4)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Union(tt.a, tt.b); !slices.Equal(got, tt.union) {
				t.Errorf("Union = %v, want %v", got, tt.union)
			}
			if got := Intersect(tt.a, tt.b); !slices.Equal(got, tt.inter) {
				t.Errorf("Intersect = %v, want %v", got, tt.inter)
			}
			if got := Subtract(tt.a, tt.b); !slices.Equal(got, tt.diff) {
				t.Errorf("Subtract = %v, want %v", got, tt.diff)
			}
		})
	}
}

// TestSetOperationsRandom compares the set operations with a bitmap.
func TestSetOperationsRandom(t *testing.T) {
	const size = 40
	rng := rand.New(rand.NewPCG(1, 2))
	gen := func() []Range {
		rs := make([]Range, rng.IntN(5))
		for i := range rs {
			s := rng.IntN(size) + 1
			rs[i] = r(s, s+rng.IntN(6))
		}
		return rs
	}
	bits := func(rs []Range) [size + 8]bool {
		var b [size + 8]bool
		for _, x := range rs {
			for l := x.Start; l <= x.End; l++ {
				b[l] = true
			}
		}
		return b
	}
	for i := 0; i < 2000; i++ {
		a, b := gen(), gen()
		ba, bb := bits(a), bits(b)
		var wantU, wantI, wantS [size + 8]bool
		count := 0
		for l := range ba {
			wantU[l] = ba[l] || bb[l]
			wantI[l] = ba[l] && bb[l]
			wantS[l] = ba[l] && !bb[l]
			if ba[l] {
				count++
			}
		}
		for _, c := range []struct {
			op   string
			got  []Range
			want [size + 8]bool
		}{
			{"Union", Union(a, b), wantU},
			{"Intersect", Intersect(a, b), wantI},
			{"Subtract", Subtract(a, b), wantS},
		} {
			if bits(c.got) != c.want || !slices.Equal(Normalize(c.got), c.got) {
				t.Fatalf("%s(%v, %v) = %v", c.op, a, b, c.got)
			}
		}
		if got := Count(a); got != count {
			t.Fatalf("Count(%v) = %d, want %d", a, got, count)
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		in   []Range
		want int
	}{
		{nil, 0},
		{[]Range{r(1, 1)}, 1},
		{[]Range{r(1, 3), r(5, 6)}, 5},
		{[]Range{r(1, 5), r(3, 8)}, 8},
		{[]Range{r(0, 3), r(4, 2)}, 0},
	}
	for _, tt := range tests {
		if got := Count(tt.in); got != tt.want {
			t.Errorf("Count(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestBlankLines(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		blank []int // blank lines
		lines int   // number of lines in src
	}{
		{"empty", "", nil, 0},
		{"one newline", "\n", []int{1}, 1},
		{"no trailing newline", "a\n\nb", []int{2}, 3},
		{"trailing line of spaces", "a\n  ", []int{2}, 2},
		{"trailing newline", "a\n\n", []int{2}, 2},
		{"whitespace kinds", "x\n \t\r\f\v\ny\n", []int{2}, 3},
		{"crlf", "a\r\n\r\n  \r\nb\r\n", []int{2, 3}, 4},
		{"crlf without final newline", "a\r\n\r\nb", []int{2}, 3},
		{"code after spaces", "  x\n\t}\n", nil, 2},
		{"non-ASCII space is not blank", "\u00a0\n\u3000\n", nil, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blank := BlankLines([]byte(tt.src))
			for line := -1; line <= tt.lines+2; line++ {
				want := slices.Contains(tt.blank, line)
				if got := blank(line); got != want {
					t.Errorf("line %d: blank = %v, want %v", line, got, want)
				}
			}
		})
	}
}
