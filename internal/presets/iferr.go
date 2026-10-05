package presets

import (
	"go/ast"
	"go/token"

	"github.com/miyamo2/go-tebanare/internal/configschema"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

const iferrSummary = "`if err != nil` blocks that return the error unchanged."

// Values of the iferr init setting (see the enum in the schema).
const (
	InitExclude  = configschema.Exclude
	InitFoldBody = configschema.FoldBody
)

func init() {
	register(&Preset{
		Name:    "iferr",
		Summary: iferrSummary,
		Criteria: []string{
			"The condition is `err != nil` and nothing else. Parentheses are ignored. `nil != err` does not match.",
			"The body is one return statement, and its last result is the identifier from the condition. Parentheses are ignored. A wrapped error, such as `fmt.Errorf(\"load: %w\", err)` or `errors.Wrap(err, \"load\")`, does not match.",
			"The statement has no init statement (`if err := f(); ...`) and no else branch.",
			"No comment is on the hidden lines, including a comment after the closing brace.",
			"The other results hold no function call (except the builtin `new`), no function literal, and no channel receive. A bare return does not match.",
		},
		Kind:        StmtKind,
		newSettings: func() any { return new(IferrSettings) },
		Compile:     compileIferr,
		Examples:    iferrExamples,
	})
}

var iferrExamples = []Example{
	{Code: "if err != nil {\n\treturn err\n}", Match: true},
	{Code: "if err != nil {\n\treturn nil, err\n}", Match: true},
	{Code: "if err != nil {\n\treturn *new(T), err\n}", Match: true},
	{Code: "if err != nil {\n\treturn fmt.Errorf(\"load config: %w\", err)\n}", Note: "It wraps the error."},
	{Code: "if err != nil {\n\tlog.Printf(\"load: %v\", err)\n\treturn err\n}", Note: "Another statement runs before `return`."},
	{Code: "if err := load(); err != nil {\n\treturn err\n}", Note: "It has an init statement."},
	{Code: "if err != nil {\n\treturn\n}", Note: "It uses a bare return."},
	{Code: "if err != nil { // the caller logs it\n\treturn err\n}", Note: "It has a comment."},
	{Settings: "names: [err, \"*Err\"]", Code: "if parseErr != nil {\n\treturn nil, parseErr\n}", Match: true,
		Note: "`\"*Err\"` in `names` adds `parseErr`."},
	{Settings: "names: [err, \"*Err\"]", Code: "if fooErr != nil {\n\treturn nil, barErr\n}",
		Note: "The condition and the return use different variables."},
	{Settings: "allow_comments: true", Code: "if err != nil { // the caller logs it\n\treturn err\n}", Match: true,
		Note: "`allow_comments: true` hides statements with comments."},
	{Settings: "init: fold-body", Code: "if err := load(); err != nil {\n\treturn err\n}", Match: true,
		Note: "The header line stays visible. The lines from `return` to the closing brace are hidden."},
	{Settings: "init: fold-body", Code: "if err := load(); err != nil { return err }",
		Note: "`fold-body` needs `return` on a line after the opening brace."},
	{Settings: "allow_bare_return: true", Code: "if err != nil {\n\treturn\n}", Match: true},
	{Settings: "allow_calls_in_results: true", Code: "if err != nil {\n\treturn time.Now(), err\n}", Match: true},
}

func compileIferr(settings any) (*rule.Rule, error) {
	s, err := checkSettings("iferr", settings, func(s *IferrSettings) []problem {
		return validatePaths(s.Paths, s.ExcludePaths)
	})
	if err != nil {
		return nil, err
	}
	m := &iferrMatcher{
		names:         s.Names,
		allowComments: boolValue(s.AllowComments),
		foldBody:      s.Init != nil && *s.Init == InitFoldBody,
		bareReturn:    boolValue(s.AllowBareReturn),
		calls:         boolValue(s.AllowCallsInResults),
	}
	return stmtRule("iferr", iferrSummary, s.Paths, s.ExcludePaths, m), nil
}

type iferrMatcher struct {
	names         []string
	allowComments bool
	foldBody      bool
	bareReturn    bool
	calls         bool
}

// Accepts implements rule.NodeMatcher.
func (m *iferrMatcher) Accepts(n ast.Node) bool {
	_, ok := n.(*ast.IfStmt)
	return ok
}

// MatchNode implements rule.NodeMatcher. With init: fold-body, an if
// statement with an init statement gets a Span from its return statement
// to its end.
func (m *iferrMatcher) MatchNode(n ast.Node, f *rule.File) (rule.Span, bool) {
	s, ok := n.(*ast.IfStmt)
	if !ok || s.Else != nil || s.Body == nil || len(s.Body.List) != 1 {
		return rule.Span{}, false
	}
	name, ok := m.errName(s.Cond)
	if !ok {
		return rule.Span{}, false
	}
	ret, ok := s.Body.List[0].(*ast.ReturnStmt)
	if !ok || !m.returnsErr(ret, name) {
		return rule.Span{}, false
	}
	var span rule.Span
	from := s.Pos()
	if s.Init != nil {
		if !m.foldBody || f.Line(ret.Pos()) <= f.Line(s.Body.Lbrace) {
			return rule.Span{}, false
		}
		span = rule.Span{From: ret.Pos(), To: s.End()}
		from = ret.Pos()
	}
	if !m.allowComments && f.HasCommentInLines(f.Line(from), f.Line(s.End()), nil) {
		return rule.Span{}, false
	}
	return span, true
}

// errName returns the variable name when cond is "name != nil" and name
// matches one of the globs.
func (m *iferrMatcher) errName(cond ast.Expr) (string, bool) {
	b, ok := rule.StripParens(cond).(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return "", false
	}
	x, ok := rule.StripParens(b.X).(*ast.Ident)
	if !ok {
		return "", false
	}
	if y, ok := rule.StripParens(b.Y).(*ast.Ident); !ok || y.Name != "nil" {
		return "", false
	}
	for _, g := range m.names {
		if matchGlob(g, x.Name) {
			return x.Name, true
		}
	}
	return "", false
}

// returnsErr reports whether ret returns the identifier name last, with
// other results that pass the safety checks.
func (m *iferrMatcher) returnsErr(ret *ast.ReturnStmt, name string) bool {
	if len(ret.Results) == 0 {
		return m.bareReturn
	}
	last, ok := rule.StripParens(ret.Results[len(ret.Results)-1]).(*ast.Ident)
	if !ok || last.Name != name {
		return false
	}
	for _, e := range ret.Results[:len(ret.Results)-1] {
		if !m.safeResult(e) {
			return false
		}
	}
	return true
}

// safeResult reports whether e has no function literal, no channel
// receive, and, unless calls are allowed, no call other than new(...).
func (m *iferrMatcher) safeResult(e ast.Expr) bool {
	safe := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			safe = false
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				safe = false
			}
		case *ast.CallExpr:
			if id, ok := rule.StripParens(n.Fun).(*ast.Ident); !m.calls && (!ok || id.Name != "new") {
				safe = false
			}
		}
		return safe
	})
	return safe
}
