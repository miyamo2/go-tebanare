package config

import (
	"errors"
	"go/ast"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/nodematch"
	"github.com/miyamo2/go-tebanare/internal/rule"
	"github.com/miyamo2/go-tebanare/internal/sigpattern"
)

// patterns is the matcher of a func rule. It matches a declaration when
// any of its patterns does.
type patterns []*sigpattern.Pattern

var _ rule.RelaxedFuncMatcher = patterns(nil)

// MatchFunc implements rule.FuncMatcher.
func (ps patterns) MatchFunc(fd *ast.FuncDecl, _ *rule.File) bool {
	for _, p := range ps {
		if p.Match(fd) {
			return true
		}
	}
	return false
}

// MatchFuncRelaxed implements rule.RelaxedFuncMatcher.
func (ps patterns) MatchFuncRelaxed(fd *ast.FuncDecl, _ *rule.File, qualifiers map[string]string) bool {
	for _, p := range ps {
		if p.MatchQualified(fd, qualifiers) {
			return true
		}
	}
	return false
}

// caretIndent prefixes the two lines that show where a pattern error is.
const caretIndent = "    "

// funcRule compiles the `func` value of a rule: one signature pattern or a
// list of them. It reports false after any error.
func (c *compiler) funcRule(e entry) (patterns, bool) {
	items, ok := c.stringList(e, true)
	if ok && len(items) == 0 {
		c.errorf(e.value, e.field, "at least one pattern is required")
		return nil, false
	}
	var ps patterns
	for _, it := range items {
		p, err := sigpattern.Parse(it.value)
		if err != nil {
			c.errorf(it.node, it.field, "%s", patternError(err))
			ok = false
			continue
		}
		for _, re := range p.NameRegexps() {
			if !nodematch.IsAnchored(re) {
				c.warnUnanchored(it.node, it.field, "name regexp /"+strings.ReplaceAll(re, "/", `\/`)+
					"/ is not anchored with ^ or $, so it matches every name that contains a match")
			}
		}
		ps = append(ps, p)
	}
	return ps, ok
}

// patternError renders a pattern syntax error as its position in the
// pattern and message, followed by the pattern line and a caret under the
// error column:
//
//	1:29: expected type, found ','
//	    func (*Repository[_]) Find*(, ...) (_, error)
//	                                ^
func patternError(err error) string {
	var pe *sigpattern.Error
	if errors.As(err, &pe) {
		return pe.Error() + "\n" + pe.Caret(caretIndent)
	}
	return err.Error()
}
