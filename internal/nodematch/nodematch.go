// Package nodematch matches statements and expressions against the
// regular expressions of stmt and expr rules.
//
// A node matches when its kind is in the rule's kind set, at least one
// `regex` entry matches its normalized text (see package canon), and no
// `not_regex` entry does. Matching uses search semantics
// (regexp.Regexp.MatchString), so an unanchored expression can match
// anywhere in the text.
package nodematch

import (
	"errors"
	"fmt"
	"go/ast"
	"regexp"
	"slices"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/regexlimit"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// Matcher is the compiled kind filter and regular expressions of one stmt
// or expr rule.
type Matcher struct {
	kinds    map[string]bool
	regex    []*regexp.Regexp
	notRegex []*regexp.Regexp
}

// Error describes one invalid entry of a stmt or expr rule.
type Error struct {
	// Key is "kind", "regex", or "not_regex".
	Key string
	// Index is the position of the entry in its list, or -1 when the
	// error concerns the list as a whole.
	Index int
	Msg   string
}

// Error returns "<key>[<index>]: <msg>", or "<key>: <msg>" when Index is
// negative.
func (e *Error) Error() string {
	if e.Index < 0 {
		return e.Key + ": " + e.Msg
	}
	return fmt.Sprintf("%s[%d]: %s", e.Key, e.Index, e.Msg)
}

// New compiles a matcher for a rule whose target is result.TargetStmt or
// result.TargetExpr. An empty kinds means rule.DefaultStmtKinds or
// rule.DefaultExprKinds. regex must have at least one entry.
//
// When an entry is invalid, New reports every invalid entry: the error
// wraps one *Error per entry (see errors.Join), so errors.As finds the
// first one and Unwrap() []error returns all of them. The message of an
// invalid regular expression names its index, as in
// "regex[1]: error parsing regexp: missing closing ): `(a`".
func New(target result.Target, kinds []string, regex, notRegex []string) (*Matcher, error) {
	var valid, defaults []string
	switch target {
	case result.TargetStmt:
		valid, defaults = rule.StmtKinds, rule.DefaultStmtKinds
	case result.TargetExpr:
		valid, defaults = rule.ExprKinds, rule.DefaultExprKinds
	default:
		return nil, fmt.Errorf("nodematch: target is %q, want %q or %q",
			target, result.TargetStmt, result.TargetExpr)
	}

	var errs []error
	m := &Matcher{kinds: map[string]bool{}}
	if len(kinds) == 0 {
		kinds = defaults
	}
	for i, k := range kinds {
		if !slices.Contains(valid, k) {
			errs = append(errs, &Error{Key: "kind", Index: i, Msg: fmt.Sprintf(
				"unknown %s kind %q (valid kinds: %s)", target, k, strings.Join(valid, ", "))})
			continue
		}
		m.kinds[k] = true
	}

	if len(regex) == 0 {
		errs = append(errs, &Error{Key: "regex", Index: -1, Msg: "at least one regular expression is required"})
	}
	var more []error
	m.regex, more = compile("regex", regex)
	errs = append(errs, more...)
	m.notRegex, more = compile("not_regex", notRegex)
	errs = append(errs, more...)

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return m, nil
}

func compile(key string, exprs []string) ([]*regexp.Regexp, []error) {
	var out []*regexp.Regexp
	var errs []error
	for i, expr := range exprs {
		if err := regexlimit.Check(expr); err != nil {
			errs = append(errs, &Error{Key: key, Index: i, Msg: err.Error()})
			continue
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			errs = append(errs, &Error{Key: key, Index: i, Msg: err.Error()})
			continue
		}
		out = append(out, re)
	}
	return out, errs
}

// Accepts reports whether the kind of n (rule.KindOf) is in the rule's
// kind set.
func (m *Matcher) Accepts(n ast.Node) bool {
	return m.kinds[rule.KindOf(n)]
}

// MatchText reports whether at least one regex entry matches text and no
// not_regex entry does.
func (m *Matcher) MatchText(text string) bool {
	return anyMatch(m.regex, text) && !anyMatch(m.notRegex, text)
}

func anyMatch(res []*regexp.Regexp, text string) bool {
	for _, re := range res {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}
