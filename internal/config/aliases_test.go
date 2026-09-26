package config

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// tooMuch is the error for aliases past maxAliasCopy.
const tooMuch = "aliases expand the config by more than 64 KiB"

func newAliasSizer() *aliasSizer {
	return &aliasSizer{sizes: map[*yaml.Node]int{}, open: map[*yaml.Node]bool{}}
}

func TestAliasCopied(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"no aliases", "a: &a [x, y]", 0},
		{"alias to a scalar", "a: &a xy\nb: *a", 1 + 2},
		{"alias to a list", "a: &a [x, yz]\nb: *a", 1 + (1 + 1) + (1 + 2)},
		{"alias to a mapping", "a: &a {k: v}\nb: *a", 1 + (1 + 1) + (1 + 1)},
		{"alias to a null", "a: &a\nb: *a", 1},
		{"each use counts", "a: &a xy\nb: [*a, *a, *a]", 3 * 3},
		// b copies a twice as written, and c copies b with the two
		// copies of a inside it.
		{"nested aliases", "a: &a [x]\nb: &b [*a, *a]\nc: *b", 3 + 3 + (1 + 3 + 3)},
		{"alias as a key", "a: &a k\nb: {*a : v}", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAliasSizer()
			if stop := a.visit(resolve(parseYAML(t, tt.src))); stop != nil || a.cycle != nil {
				t.Fatalf("visit stopped at %d:%d", stop.Line, stop.Column)
			}
			if a.copied != tt.want {
				t.Errorf("copied = %d, want %d", a.copied, tt.want)
			}
		})
	}
}

// laughs returns a document of n levels whose last level expands to
// 10^n scalars.
func laughs(n int) string {
	var b strings.Builder
	b.WriteString("l0: &l0 [x, x, x, x, x, x, x, x, x, x]\n")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, "l%d: &l%d [%s]\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*l%d, ", i-1), 10), ", "))
	}
	return b.String()
}

func TestCheckAliases(t *testing.T) {
	// ok holds a scalar whose alias copies exactly maxAliasCopy.
	ok := strings.Repeat("x", maxAliasCopy-1)
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"at the limit", "a: &a " + ok + "\nb: *a", nil},
		{"past the limit", "a: &a " + ok + "x\nb: *a", []string{"2:4: " + tooMuch}},
		{"past the limit in several steps", "a: &a " + ok[:maxAliasCopy/2] + "\nb: [*a, *a]", []string{"2:9: " + tooMuch}},
		// 10^9 scalars: the check measures each anchor once.
		{"exponential", laughs(9), []string{"5:15: " + tooMuch}},
		{"list that contains its alias", "a: &a [*a]", []string{"1:8: alias *a refers to a value that contains it"}},
		{"cycle through a nested anchor", "a: &a {b: &b [x, *a]}\nc: *b", []string{"1:18: alias *a refers to a value that contains it"}},
		{"cycle through two anchors", "a: &a [&b [*a]]\nc: *b", []string{"1:12: alias *a refers to a value that contains it"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiler{}
			if got := c.checkAliases(resolve(parseYAML(t, tt.src))); got != (len(tt.want) == 0) {
				t.Errorf("checkAliases = %v", got)
			}
			checkDiags(t, "errors", c.errs, tt.want)
		})
	}
}

// nestedCycle returns n nested anchored lists, the innermost of which
// holds an alias to each of them, from the innermost out.
func nestedCycle(n int) (src string, column int) {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "&a%d [", i)
	}
	column = b.Len() + 1
	for i := n - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "*a%d, ", i)
	}
	return strings.TrimSuffix(b.String(), ", ") + strings.Repeat("]", n), column
}

func TestCheckAliasesQuadratic(t *testing.T) {
	list := func(item string, n int) string { return strings.TrimSuffix(strings.Repeat(item+", ", n), ", ") }
	cycle, column := nestedCycle(2400)
	tests := []struct {
		name  string
		src   string
		want  string
		limit time.Duration
	}{
		// Each *r copies 300 invalid globs; the 70th copy passes the limit.
		{"config with errors in the copies", "version: 1\np: &p 'a['\nf: &f [" + list("*p", 300) +
			"]\nr: &r {iferr: {names: *f}}\npresets: [" + list("*r", 300) + "]\n", "5:287: " + tooMuch, 10 * time.Second},
		// 38 KB: measuring on after the cycle took seconds and a stack that
		// grew with the square of the depth.
		{"nested anchors in a cycle", cycle, fmt.Sprintf("1:%d: alias *a2399 refers to a value that contains it", column), time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiler{}
			root, ok := c.parse([]byte(tt.src))
			if !ok {
				t.Fatalf("parse: %q", format(c.errs))
			}
			start := time.Now()
			if c.checkAliases(root) {
				t.Error("checkAliases = true")
			}
			if d := time.Since(start); d > tt.limit {
				t.Errorf("checkAliases took %v", d)
			}
			checkDiags(t, "errors", c.errs, []string{tt.want})
		})
	}
}
