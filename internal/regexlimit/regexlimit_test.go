package regexlimit

import (
	"regexp"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	nested := func(n int) string { return strings.Repeat("(", n) + "a" + strings.Repeat(")", n) }
	if err := Check(nested(MaxGroups)); err != nil {
		t.Errorf("Check(%d groups) = %v", MaxGroups, err)
	}
	err := Check(nested(MaxGroups + 1))
	if err == nil || err.Error() != `the expression has 201 "(" characters, more than the limit of 200` {
		t.Errorf("Check(%d groups) = %v", MaxGroups+1, err)
	}
	if err := Check(strings.Repeat(`\(`, MaxGroups+1)); err == nil {
		t.Error("Check counts escaped parentheses too")
	}
}

// TestDepthBound checks that the deepest shapes Check accepts stay below
// the nesting limit of regexp/syntax, which it enforces with a panic.
func TestDepthBound(t *testing.T) {
	for _, group := range []struct{ open, close string }{
		{"(", ")"},
		{"(", ")*"},
		{"(x|", ")+"},
		{"(x|y", "z)*"},
		{"(?:x|", ")*"},
	} {
		expr := strings.Repeat(group.open, MaxGroups) + "a" + strings.Repeat(group.close, MaxGroups)
		if err := Check(expr); err != nil {
			t.Fatalf("Check(%q...) = %v", expr[:20], err)
		}
		if _, err := regexp.Compile(expr); err != nil {
			t.Errorf("%s...%s: %v", group.open, group.close, err)
		}
	}
}
