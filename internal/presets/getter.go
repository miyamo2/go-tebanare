package presets

import (
	"go/ast"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

const getterSummary = "Methods that only return a field of the receiver."

func (s *GetterSettings) validate() []problem {
	return validatePaths(s.Paths, s.ExcludePaths)
}

func init() {
	register(&Preset{
		Name:    "getter",
		Summary: getterSummary,
		Criteria: []string{
			"The function is a method with a named receiver. Value, pointer, and generic receiver types all count.",
			"It takes no parameters.",
			"It returns exactly one value of any type. A named result is allowed.",
			"The body is one return statement that returns a field of the receiver, such as `u.name`, or a chain of fields, such as `u.cfg.timeout`. Parentheses are ignored.",
			"The first field after the receiver (`x` in `u.x`) is not the name of a method that the same file declares on the same type, since `u.x` would then be a method value.",
			"No comment other than the doc comment is on the lines of the declaration.",
		},
		Kind:     FuncKind,
		Compile:  compileGetter,
		Examples: getterExamples,
	})
}

var getterExamples = []Example{
	{Code: "func (u *User) Name() string { return u.name }", Match: true},
	{Code: "func (u User) ID() (id int64) { return (u.id) }", Match: true,
		Note: "A named result and parentheses are allowed."},
	{Code: "func (s *Stack[T]) Len() int { return s.n }", Match: true,
		Note: "Generic receiver types are allowed."},
	{Code: "// Timeout returns the request timeout.\nfunc (u *User) Timeout() time.Duration { return u.cfg.timeout }", Match: true,
		Note: "A chain of fields is allowed, and the doc comment does not affect the result."},
	{Code: "func (u *User) Title() string { return strings.TrimSpace(u.title) }",
		Note: "It does more than return a field."},
	{Code: "func (u *User) NameOr(def string) string { return u.name }", Note: "It takes a parameter."},
	{Code: "func (u *User) Pair() (string, int) { return u.name, u.age }", Note: "It returns two values."},
	{Code: "func (u *User) First() string { return u.items[0] }", Note: "It indexes a field."},
	{Code: "func (u *User) Limit() int { return u.cfg.Limit() }", Note: "It calls a method."},
	{Code: "func (*User) Kind() string { return kind }", Note: "`kind` is not a field of the receiver."},
	{Code: "func (u *User) Age() int { return u.age /* TODO */ }", Note: "It has a comment."},
	{Settings: "max_depth: 1", Code: "func (u *User) Name() string { return u.name }", Match: true,
		Note: "One field is within `max_depth`."},
	{Settings: "max_depth: 1", Code: "func (u *User) Timeout() time.Duration { return u.cfg.timeout }",
		Note: "The chain has two fields, more than `max_depth`."},
}

func compileGetter(settings any) (*rule.Rule, error) {
	s, err := checkSettings[GetterSettings]("getter", settings)
	if err != nil {
		return nil, err
	}
	m := &getterMatcher{}
	if s.MaxDepth != nil {
		m.maxDepth = *s.MaxDepth
	}
	return funcRule("getter", getterSummary, s.Paths, s.ExcludePaths, s.IncludeDoc, m), nil
}

type getterMatcher struct {
	maxDepth int // 0 means unlimited
}

// MatchFunc implements rule.FuncMatcher.
func (m *getterMatcher) MatchFunc(fd *ast.FuncDecl, f *rule.File) bool {
	base, _, ok := rule.RecvBase(fd)
	if !ok || fd.Type.Params.NumFields() != 0 || fd.Type.Results.NumFields() != 1 {
		return false
	}
	names := fd.Recv.List[0].Names
	if len(names) != 1 || names[0].Name == "_" {
		return false
	}
	if fd.Body == nil || len(fd.Body.List) != 1 {
		return false
	}
	ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	depth, first, ok := fieldChain(ret.Results[0], names[0].Name)
	if !ok || (m.maxDepth > 0 && depth > m.maxDepth) {
		return false
	}
	if f.MethodsOf(base)[first] {
		return false
	}
	return !f.HasCommentInLines(f.Line(fd.Pos()), f.Line(fd.End()), fd.Doc)
}

// fieldChain reports whether e is a chain of selectors on the identifier
// recv, such as recv.a.b, ignoring parentheses at every level. It returns
// the number of selectors and the one applied to recv (a).
func fieldChain(e ast.Expr, recv string) (depth int, first string, ok bool) {
	e = rule.StripParens(e)
	for {
		sel, isSel := e.(*ast.SelectorExpr)
		if !isSel {
			break
		}
		depth++
		first = sel.Sel.Name
		e = rule.StripParens(sel.X)
	}
	id, isIdent := e.(*ast.Ident)
	return depth, first, depth > 0 && isIdent && id.Name == recv
}
