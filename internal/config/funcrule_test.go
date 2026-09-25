package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// ruleEntry decodes src, the mapping of one rule, and returns the entry
// for key. Fields start with "rules[0](x)".
func ruleEntry(t *testing.T, c *compiler, src, key string) entry {
	t.Helper()
	es, _ := c.mapping(parseYAML(t, src), field("rules[0](x)"), "func", "stmt", "expr")
	e, ok := es.get(key)
	if !ok {
		t.Fatalf("no %s in %q", key, src)
	}
	return e
}

// funcDecls parses src, a Go file without the package clause, and returns
// its function declarations by name.
func funcDecls(t *testing.T, src string) map[string]*ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package p\n\n"+src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	decls := map[string]*ast.FuncDecl{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			decls[fd.Name.Name] = fd
		}
	}
	return decls
}

func TestFuncRuleMatch(t *testing.T) {
	c := &compiler{}
	ps, ok := c.funcRule(ruleEntry(t, c, `func: ["func (_) String() string", "func New*"]`, "func"))
	if !ok || len(ps) != 2 || len(c.errs) > 0 {
		t.Fatalf("funcRule = %v, %v, errors %q", ps, ok, format(c.errs))
	}
	var m rule.FuncMatcher = ps
	decls := funcDecls(t, `
func (u User) String() string { return "" }
func NewUser() *User { return nil }
func Parse() {}
`)
	for name, want := range map[string]bool{"String": true, "NewUser": true, "Parse": false} {
		if got := m.MatchFunc(decls[name], nil); got != want {
			t.Errorf("MatchFunc(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestFuncRuleRelaxed(t *testing.T) {
	c := &compiler{}
	ps, _ := c.funcRule(ruleEntry(t, c, `func: "func Run(context.Context) error"`, "func"))
	fd := funcDecls(t, `
import ctx2 "context"

func Run(c ctx2.Context) error { return nil }
`)["Run"]
	var m rule.RelaxedFuncMatcher = ps
	if ps.MatchFunc(fd, nil) {
		t.Error("MatchFunc matched an aliased qualifier")
	}
	if !m.MatchFuncRelaxed(fd, nil, map[string]string{"ctx2": "context"}) {
		t.Error("MatchFuncRelaxed did not match with the alias rewritten")
	}
	if m.MatchFuncRelaxed(fd, nil, nil) {
		t.Error("MatchFuncRelaxed matched without qualifiers")
	}
}

func TestFuncRuleErrors(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		want  []string
		warns []string
	}{
		{
			"pattern error",
			`func: "func (*Repository[_]) Find*(, ...) (_, error)"`,
			[]string{"1:7: rules[0](x).func: 1:29: expected type, found ','\n" +
				"    func (*Repository[_]) Find*(, ...) (_, error)\n" +
				"                                ^"},
			nil,
		},
		{
			"pattern error in a list",
			"func:\n  - func A()\n  - 'func B(]'",
			[]string{"3:5: rules[0](x).func[1]: 1:8: expected type, found ']'\n" +
				"    func B(]\n" +
				"           ^"},
			nil,
		},
		{
			"pattern error on a later line",
			"func: |\n  func C(\n    a int,\n    b ,\n  )",
			[]string{"1:7: rules[0](x).func: 3:3: missing parameter type\n" +
				"      b ,\n" +
				"      ^"},
			nil,
		},
		{"empty list", "func: []", []string{"1:7: rules[0](x).func: at least one pattern is required"}, nil},
		{"wrong type", "func: {a: b}", []string{
			"1:7: rules[0](x).func: expected a string or a list of strings, found a mapping",
		}, nil},
		{"item of the wrong type", "func: [func A(), 3]", []string{
			"1:18: rules[0](x).func[1]: expected a string, found integer 3",
		}, nil},
		{
			"unanchored name regexps",
			`func: ["func (/Handler/) /^Get/()", 'func (*/^Mock/) /a\/b/()', "func /Set$/()"]`,
			nil,
			[]string{
				"1:8: rules[0](x).func[0]: name regexp /Handler/ is not anchored with ^ or $, so it matches every name that contains a match",
				`1:37: rules[0](x).func[1]: name regexp /a\/b/ is not anchored with ^ or $, so it matches every name that contains a match`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiler{}
			_, ok := c.funcRule(ruleEntry(t, c, tt.src, "func"))
			if ok != (len(tt.want) == 0) {
				t.Errorf("funcRule ok = %v", ok)
			}
			checkDiags(t, "errors", c.errs, tt.want)
			checkDiags(t, "warnings", c.warns, tt.warns)
		})
	}
}
