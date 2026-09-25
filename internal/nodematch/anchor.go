package nodematch

import (
	"regexp/syntax"

	"github.com/miyamo2/go-tebanare/internal/regexlimit"
)

// IsAnchored reports whether the regular expression expr is anchored at
// its start or at its end. The start is anchored when the expression
// begins with ^ or \A, and the end is anchored when it ends with $ or \z.
// One anchored end is enough, so "^a" and "a$" are both anchored. A group
// counts as its contents, so "(^a)b" is anchored. An alternation anchors
// an end only when every branch anchors that same end: "^a|^b" is
// anchored, while "^a|b" and "^a|b$" are not. It returns false when expr
// does not parse.
//
// Config validation uses it to warn about unanchored regex entries, which
// can hide more code than intended because matching searches the whole
// normalized text.
func IsAnchored(expr string) bool {
	if regexlimit.Check(expr) != nil {
		return false
	}
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return false
	}
	return anchoredStart(re) || anchoredEnd(re)
}

// anchoredStart reports whether every match of re starts at ^ or \A.
// The parser limits nesting depth, which bounds the recursion.
func anchoredStart(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpBeginText, syntax.OpBeginLine:
		return true
	case syntax.OpCapture:
		return len(re.Sub) == 1 && anchoredStart(re.Sub[0])
	case syntax.OpConcat:
		return len(re.Sub) > 0 && anchoredStart(re.Sub[0])
	case syntax.OpAlternate:
		return every(re.Sub, anchoredStart)
	}
	return false
}

// anchoredEnd reports whether every match of re ends at $ or \z.
func anchoredEnd(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpEndText, syntax.OpEndLine:
		return true
	case syntax.OpCapture:
		return len(re.Sub) == 1 && anchoredEnd(re.Sub[0])
	case syntax.OpConcat:
		return len(re.Sub) > 0 && anchoredEnd(re.Sub[len(re.Sub)-1])
	case syntax.OpAlternate:
		return every(re.Sub, anchoredEnd)
	}
	return false
}

func every(subs []*syntax.Regexp, anchored func(*syntax.Regexp) bool) bool {
	for _, sub := range subs {
		if !anchored(sub) {
			return false
		}
	}
	return len(subs) > 0
}
