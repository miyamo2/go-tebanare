package nodematch

import (
	"go/ast"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

var _ rule.NodeMatcher = (*Matcher)(nil)

// MatchNode reports whether the normalized text of n, taken from f.Text,
// matches. It reports false when the text is unavailable, for example
// because n is larger than the size limit. The returned Span is always
// zero: the analyzer derives the lines from n and the rule.
func (m *Matcher) MatchNode(n ast.Node, f *rule.File) (rule.Span, bool) {
	if f == nil {
		return rule.Span{}, false
	}
	text, ok := f.Text(n)
	if !ok {
		return rule.Span{}, false
	}
	return rule.Span{}, m.MatchText(text)
}
