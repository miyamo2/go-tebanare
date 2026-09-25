// Package regexlimit rejects regular expressions that would make
// regexp/syntax panic.
//
// regexp/syntax reports an expression that nests more than 1000 levels
// deep by panicking and recovering inside its parser. The TinyGo wasm
// build cannot recover, so such an expression in a config would stop the
// engine. Every group adds at most four levels (the group, an alternation,
// a concatenation, and a repetition), so an expression with at most
// MaxGroups "(" characters stays far below that depth.
package regexlimit

import (
	"fmt"
	"strings"
)

// MaxGroups is the largest number of "(" characters that Check accepts.
// Escaped parentheses and parentheses in character classes count too.
const MaxGroups = 200

// Check returns an error when expr holds more than MaxGroups "("
// characters.
func Check(expr string) error {
	if n := strings.Count(expr, "("); n > MaxGroups {
		return fmt.Errorf("the expression has %d \"(\" characters, more than the limit of %d", n, MaxGroups)
	}
	return nil
}
