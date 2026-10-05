package presets

import (
	"go/ast"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

const noopSummary = "Methods that take no parameters, return nothing, and have an empty body."

func init() {
	register(&Preset{
		Name:    "noop",
		Summary: noopSummary,
		Criteria: []string{
			"The function is a method. The receiver may be unnamed. Value, pointer, and generic receiver types all count.",
			"It takes no parameters.",
			"It has no results.",
			"It has a body, and the body holds no statements (`{}`). Comments in the body are allowed and hidden with it. A declaration without a body, such as a method implemented in assembly, does not match.",
		},
		Kind:        FuncKind,
		newSettings: func() any { return new(NoopSettings) },
		Compile:     compileNoop,
		Examples:    noopExamples,
	})
}

var noopExamples = []Example{
	{Code: "func (*BadExpr) exprNode() {}", Match: true, Note: "A marker method from `go/ast`."},
	{Code: "func (s Set[T]) sealed() {}", Match: true},
	{Code: "func (t *noopTracer) Flush() {}", Match: true},
	{Code: "func (s *Server) Shutdown() { /* TODO: implement */ }", Match: true,
		Note: "A comment in the body does not matter by default."},
	{Code: "func (t *Tracer) Flush() { t.buf.Reset() }", Note: "The body has a statement."},
	{Code: "func (nopLogger) Printf(format string, args ...any) {}", Note: "It takes parameters."},
	{Code: "func (nopCloser) Close() error { return nil }", Note: "It returns a value."},
	{Code: "func noop() {}", Note: "It is not a method."},
	{Code: "func (t *Timer) stop()", Note: "It has no body (implemented in assembly, for example)."},
	{Settings: "allow_comments: false", Code: "func (s *Server) Shutdown() { /* TODO: implement */ }",
		Note: "`allow_comments: false` keeps methods with comments visible."},
	{Settings: "include_functions: true", Code: "func noop() {}", Match: true,
		Note: "`include_functions: true` adds functions that are not methods."},
}

func compileNoop(settings any) (*rule.Rule, error) {
	s, err := checkSettings("noop", settings, func(s *NoopSettings) []problem {
		return validatePaths(s.Paths, s.ExcludePaths)
	})
	if err != nil {
		return nil, err
	}
	m := &noopMatcher{allowComments: boolValue(s.AllowComments), includeFunctions: boolValue(s.IncludeFunctions)}
	return funcRule("noop", noopSummary, s.Paths, s.ExcludePaths, boolValue(s.IncludeDoc), m), nil
}

type noopMatcher struct {
	allowComments    bool
	includeFunctions bool
}

// MatchFunc implements rule.FuncMatcher.
func (m *noopMatcher) MatchFunc(fd *ast.FuncDecl, f *rule.File) bool {
	if fd.Body == nil || len(fd.Body.List) != 0 {
		return false
	}
	if fd.Type.Params.NumFields() != 0 || fd.Type.Results.NumFields() != 0 {
		return false
	}
	if _, _, method := rule.RecvBase(fd); !method && (!m.includeFunctions || fd.Recv != nil) {
		return false
	}
	return m.allowComments || !f.HasCommentInLines(f.Line(fd.Pos()), f.Line(fd.End()), fd.Doc)
}
