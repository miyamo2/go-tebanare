// Package regexlimit rejects regular expressions that would make
// regexp/syntax panic.
//
// regexp/syntax enforces three limits by panicking and recovering inside
// its parser (checkLimits in parse.go). The TinyGo wasm build cannot
// recover, so an expression in a config that reaches one of them would
// stop the engine. Check accepts only expressions that stay well below
// all three, judging from the text alone in one linear pass:
//
//   - Nesting depth (ErrNestingDepth): the parse tree may be at most 1000
//     levels high. A group adds up to four levels (the repetition applied
//     to it, the capture, an alternation, and a concatenation), and
//     factoring the common prefixes of the alternatives of an alternation
//     adds up to two more levels per "|": "(x|y" ... "x|y)*" reaches six
//     levels per group. Check estimates the height as
//     4 + 3*(top-level "|") + the heaviest chain of nested groups, each
//     group weighing 6 + 3*(its own "|"), and rejects an estimate above
//     MaxHeight. The weights are 1.5 times the observed ones and MaxHeight
//     is 60% of the parser's limit.
//
//   - Rune count (ErrLarge, maxRunes = 2^25 runes in character classes):
//     a character class holds, per byte of its text, at most 500 runes
//     from Unicode tables (\pC takes 3 bytes for 1424 runes in Go 1.25),
//     plus at most 2F+2 runes for case folding and negation, where F = 2878
//     runes have a case fold. A class takes at least two bytes, so every
//     byte adds at most 500 + (2F+2)/2 < 3400 runes, and MaxLength bytes
//     add at most about 14M runes, 41% of the limit. Literals add one rune
//     per byte. TestRuneBound checks these figures against the running Go
//     version.
//
//   - Program size (ErrLarge, maxSize = 3,355,443 instructions): the size
//     regexp/syntax computes for a tree is the sum over its nodes of a
//     local cost, each multiplied by the counts of the repetitions
//     enclosing the node. Local costs add at most 3 per byte of the text,
//     plus max-min for x{min,max}, which is at most the product P of the
//     counts of all repetitions in the expression. The size is therefore
//     at most P * (3*len + 1 + len/3) <= 4 * P * len. Check rejects
//     P * len above MaxRepeatCost, which keeps the size below 1.6M, half
//     the limit. P counts every "{n}", "{n,}", and "{n,m}" in the text,
//     including ones in classes and escapes, taking max(n, m, 1).
package regexlimit

import (
	"fmt"
	"strings"
)

const (
	// MaxLength is the longest expression, in bytes, that Check accepts.
	MaxLength = 4096

	// MaxHeight is the largest estimated parse tree height that Check
	// accepts. See the package documentation for the estimate.
	MaxHeight = 600

	// MaxRepeatCost is the largest product of the expression length and
	// the counts of all its "{n,m}" repetitions that Check accepts.
	MaxRepeatCost = 400_000
)

// Weights of the height estimate.
const (
	heightBase  = 4
	heightGroup = 6
	heightAlt   = 3
)

// Check returns an error when expr might make regexp/syntax reach one of
// its internal limits, which it enforces by panicking. An expression that
// Check accepts may still be invalid; Check only guarantees that parsing
// it returns rather than panics.
func Check(expr string) error {
	if len(expr) > MaxLength {
		return fmt.Errorf("the expression is %d bytes long, more than the limit of %d", len(expr), MaxLength)
	}
	if h := height(expr); h > MaxHeight {
		return fmt.Errorf("the expression nests too deeply (estimated depth %d, more than the limit of %d)", h, MaxHeight)
	}
	if repeatCost(expr) > MaxRepeatCost {
		return fmt.Errorf("the expression repeats too much (its length times its repetition counts is more than the limit of %d)", MaxRepeatCost)
	}
	return nil
}

// height estimates the parse tree height of expr. It skips escapes,
// \Q...\E, and character classes the way regexp/syntax does; where the
// two disagree the expression is invalid from that point on, so
// regexp/syntax stops there.
func height(s string) int {
	type frame struct{ alts, child int }
	stack := []frame{{}}
	closeTop := func() {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		h := heightGroup + heightAlt*f.alts + f.child
		p := &stack[len(stack)-1]
		p.child = max(p.child, h)
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) && s[i+1] == 'Q' {
				if j := strings.Index(s[i+2:], `\E`); j >= 0 {
					i += 2 + j + 1
				} else {
					i = len(s)
				}
				continue
			}
			i++ // skip the escaped byte
		case '[':
			i = classEnd(s, i)
		case '(':
			stack = append(stack, frame{})
		case ')':
			// An unmatched ")" is a syntax error: parsing stops here.
			if len(stack) > 1 {
				closeTop()
			}
		case '|':
			stack[len(stack)-1].alts++
		}
	}
	for len(stack) > 1 {
		closeTop()
	}
	return heightBase + heightAlt*stack[0].alts + stack[0].child
}

// classEnd returns the index of the "]" that ends the character class
// starting at s[i], or len(s) when the class is not terminated.
func classEnd(s string, i int) int {
	j := i + 1
	if j < len(s) && s[j] == '^' {
		j++
	}
	if j < len(s) && s[j] == ']' {
		j++ // a leading "]" is a literal
	}
	for ; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '[' && j+1 < len(s) && s[j+1] == ':':
			if k := strings.Index(s[j+2:], ":]"); k >= 0 {
				j += 2 + k + 1
			}
		case s[j] == ']':
			return j
		}
	}
	return len(s)
}

// repeatCost returns len(s) times the product of the counts of every
// "{n}", "{n,}", and "{n,m}" in s, saturating above MaxRepeatCost.
func repeatCost(s string) int {
	cost := max(len(s), 1)
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		n, j := number(s, i+1)
		if j == i+1 {
			continue
		}
		if j < len(s) && s[j] == ',' {
			var m int
			m, j = number(s, j+1)
			n = max(n, m)
		}
		if j >= len(s) || s[j] != '}' {
			continue
		}
		cost *= max(n, 1)
		if cost > MaxRepeatCost {
			return MaxRepeatCost + 1
		}
		i = j
	}
	return cost
}

// number parses the decimal digits at s[i:], saturating above
// MaxRepeatCost, and returns the value and the index after the digits.
func number(s string, i int) (int, int) {
	n := 0
	for ; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
		n = min(n*10+int(s[i]-'0'), MaxRepeatCost+1)
	}
	return n, i
}
