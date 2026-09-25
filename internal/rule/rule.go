// Package rule defines the compiled form of a configuration. The config
// package builds a Set, and the analyzer runs it against Go source files.
package rule

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// DefaultInclude is the file filter used when `files.include` is omitted.
var DefaultInclude = []string{"**/*.go"}

// Set is a compiled configuration.
type Set struct {
	// Include and Exclude are doublestar patterns matched against the
	// slash-separated path relative to the repository root.
	Include []string
	Exclude []string
	// Rules holds the rules built from presets, in `presets` order.
	Rules []*Rule
}

// IsTarget reports whether the file at path is analyzed at all. Only files
// ending in ".go" can be targets.
func (s *Set) IsTarget(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	include := s.Include
	if len(include) == 0 {
		include = DefaultInclude
	}
	return matchAny(include, path) && !matchAny(s.Exclude, path)
}

// Rule is one compiled rule.
type Rule struct {
	ID          string
	Description string
	// Preset is the preset name when the rule was built from `presets`.
	Preset string
	Target result.Target

	// Paths and ExcludePaths narrow the files the rule applies to, on top
	// of Set.Include and Set.Exclude.
	Paths        []string
	ExcludePaths []string

	// IncludeDoc hides the doc comment together with the function. It is
	// used by func rules only.
	IncludeDoc bool

	// Func is set when Target is result.TargetFunc.
	Func FuncMatcher
	// Node is set when Target is result.TargetStmt.
	Node NodeMatcher
}

// AppliesTo reports whether the rule's own path filters accept path.
func (r *Rule) AppliesTo(path string) bool {
	if len(r.Paths) > 0 && !matchAny(r.Paths, path) {
		return false
	}
	return !matchAny(r.ExcludePaths, path)
}

// FuncMatcher decides whether a function declaration matches a func rule.
type FuncMatcher interface {
	MatchFunc(fd *ast.FuncDecl, f *File) bool
}

// NodeMatcher decides whether a statement matches a stmt rule.
type NodeMatcher interface {
	// Accepts is a cheap filter on the node kind. MatchNode is called only
	// for nodes that Accepts approves.
	Accepts(n ast.Node) bool
	// MatchNode reports whether n matches. A non-zero Span overrides the
	// lines to hide; a zero Span means the lines of n.
	MatchNode(n ast.Node, f *File) (Span, bool)
}

// FuncMatcherFunc adapts a function to FuncMatcher.
type FuncMatcherFunc func(fd *ast.FuncDecl, f *File) bool

// MatchFunc implements FuncMatcher.
func (fn FuncMatcherFunc) MatchFunc(fd *ast.FuncDecl, f *File) bool { return fn(fd, f) }

// Span is a source range to hide, from the start of From to the end of To.
// Both positions refer to the File's FileSet.
type Span struct {
	From token.Pos
	To   token.Pos
}

// IsZero reports whether the span is unset.
func (s Span) IsZero() bool { return s.From == token.NoPos && s.To == token.NoPos }

func matchAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if ok, err := doublestar.Match(p, path); err == nil && ok {
			return true
		}
	}
	return false
}
