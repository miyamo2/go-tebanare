package config

import (
	"slices"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/nodematch"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// decodeRules decodes the `rules` list of src with the given presets
// enabled.
func decodeRules(t *testing.T, src string, enabled enabledPresets) (*compiler, []*rule.Rule) {
	t.Helper()
	c := &compiler{}
	es, _ := c.mapping(parseYAML(t, src), "", "rules")
	e, ok := es.get("rules")
	if !ok {
		t.Fatalf("no rules in %q", src)
	}
	rules := c.rules(e, enabled)
	sortByPosition(c.errs)
	return c, rules
}

func TestRules(t *testing.T) {
	c, rules := decodeRules(t, `
rules:
  - id: stringer
    description: Stringer methods
    func: "func (_) String() string"
  - id: nodoc
    func: "func New*"
    include_doc: false
    paths: ["internal/**"]
    exclude_paths: ["**/*_gen.go"]
  - id: off
    enabled: false
    func: "func /Get/()"
  - id: tracing
    enabled: true
    stmt: {regex: '^defer span\.End\(\)$'}
  - id: log
    expr: {regex: '^log\.', hide: statement, include_leading_comments: true}
  - id: noop
    func: "func (_) Close()"
`, enabledPresets{"getter": 3})
	if len(c.errs) > 0 || len(c.warns) > 0 {
		t.Fatalf("errors %q, warnings %q", format(c.errs), format(c.warns))
	}
	var ids []string
	for _, r := range rules {
		ids = append(ids, r.ID)
	}
	if want := []string{"stringer", "nodoc", "tracing", "log", "noop"}; !slices.Equal(ids, want) {
		t.Fatalf("ids = %q, want %q", ids, want)
	}
	check := func(r *rule.Rule, target result.Target, desc string, includeDoc bool, paths, exclude []string) {
		t.Helper()
		if r.Target != target || r.Description != desc || r.IncludeDoc != includeDoc || r.Preset != "" ||
			!slices.Equal(r.Paths, paths) || !slices.Equal(r.ExcludePaths, exclude) {
			t.Errorf("rule %s = %+v", r.ID, r)
		}
		if (r.Func != nil) != (target == result.TargetFunc) || (r.Node != nil) == (target == result.TargetFunc) {
			t.Errorf("rule %s: Func = %v, Node = %v", r.ID, r.Func, r.Node)
		}
	}
	check(rules[0], result.TargetFunc, "Stringer methods", true, nil, nil)
	check(rules[1], result.TargetFunc, "", false, []string{"internal/**"}, []string{"**/*_gen.go"})
	check(rules[2], result.TargetStmt, "", false, nil, nil)
	check(rules[3], result.TargetExpr, "", false, nil, nil)
	check(rules[4], result.TargetFunc, "", true, nil, nil)
	if rules[3].Hide != rule.HideStatement || !rules[3].IncludeLeadingComments {
		t.Errorf("rule log = %+v", rules[3])
	}
}

func TestRulesAliases(t *testing.T) {
	c, rules := decodeRules(t, `
rules:
  - {id: a, paths: &p ["internal/**"], stmt: &s {regex: '^x'}}
  - {id: b, paths: *p, stmt: *s}
  - {id: c, func: [&f "func A()", *f]}
`, nil)
	if len(c.errs) > 0 || len(c.warns) > 0 || len(rules) != 3 {
		t.Fatalf("errors %q, warnings %q, rules %v", format(c.errs), format(c.warns), rules)
	}
	m, _ := rules[1].Node.(*nodematch.Matcher)
	ps, _ := rules[2].Func.(patterns)
	if !slices.Equal(rules[1].Paths, []string{"internal/**"}) || m == nil || !m.MatchText("x") || len(ps) != 2 {
		t.Errorf("rules = %+v", rules)
	}
}

func TestRulesErrors(t *testing.T) {
	enabled := enabledPresets{"getter": 3, "iferr": 4}
	tests := []struct {
		name  string
		src   string
		want  []string
		warns []string
	}{
		{"not a list", "rules: {id: a}", []string{"1:8: rules: expected a list, found a mapping"}, nil},
		{"entry not a mapping", "rules: [x]", []string{`1:9: rules[0]: expected a mapping, found string "x"`}, nil},
		{"missing id", "rules:\n  - func: 'func A()'", []string{`2:5: rules[0]: missing required key "id"`}, nil},
		{"id of the wrong type", "rules:\n  - id: 3\n    func: 'func A()'", []string{
			"2:9: rules[0].id: expected a string, found integer 3",
		}, nil},
		{"empty id", "rules:\n  - id: ''\n    func: 'func A()'", []string{"2:9: rules[0].id: id must not be empty"}, nil},
		{"duplicate id", "rules:\n  - {id: a, func: 'func A()'}\n  - {id: b, func: 'func B()'}\n  - {id: a, enabled: false, func: 'func C()'}", []string{
			`4:10: rules[2](a).id: duplicate id "a" (first set on line 2)`,
		}, nil},
		{"id of an enabled preset", "rules:\n  - id: getter\n    func: 'func A()'", []string{
			`2:9: rules[0](getter).id: id "getter" is the name of the preset enabled on line 3`,
		}, nil},
		{"id of a preset that is not enabled", "rules:\n  - id: noop\n    func: 'func A()'", nil, nil},
		{"no target", "rules:\n  - id: a\n    description: x", []string{
			"2:5: rules[0](a): a rule needs one of func, stmt, expr",
		}, nil},
		{"two targets", "rules:\n  - id: a\n    func: 'func A()'\n    stmt: {regex: '^a'}\n    expr: {regex: '^b'}", []string{
			"4:5: rules[0](a).stmt: a rule has exactly one of func, stmt, expr, and this rule already has func",
			"5:5: rules[0](a).expr: a rule has exactly one of func, stmt, expr, and this rule already has func",
		}, nil},
		{"include_doc on a stmt rule", "rules:\n  - id: a\n    stmt: {regex: '^a'}\n    include_doc: true", []string{
			"4:5: rules[0](a).include_doc: include_doc applies to func rules only",
		}, nil},
		{"hide on the rule", "rules:\n  - id: a\n    expr: {regex: '^a'}\n    hide: self", []string{
			`4:5: rules[0](a).hide: unknown key "hide" (allowed keys: id, description, func, stmt, expr, paths, exclude_paths, include_doc, enabled)`,
		}, nil},
		{"values of the wrong type", "rules:\n  - id: a\n    func: 'func A()'\n    description: [x]\n    include_doc: 1\n    enabled: 'no'", []string{
			"4:18: rules[0](a).description: expected a string, found a list",
			"5:18: rules[0](a).include_doc: expected a boolean, found integer 1",
			`6:14: rules[0](a).enabled: expected a boolean, found string "no"`,
		}, nil},
		{"invalid globs", "rules:\n  - id: a\n    func: 'func A()'\n    paths: ['a/[']\n    exclude_paths: 'b/**'", []string{
			`4:13: rules[0](a).paths[0]: invalid glob "a/["`,
			`5:20: rules[0](a).exclude_paths: expected a list of strings, found string "b/**"`,
		}, nil},
		{"disabled rule with an error", "rules:\n  - id: a\n    enabled: false\n    func: 'func A('", []string{
			"4:11: rules[0](a).func: 1:8: expected type, found 'EOF'\n    func A(\n           ^",
		}, nil},
		{"warnings of a disabled rule are dropped", "rules:\n  - id: a\n    enabled: false\n    stmt: {regex: x}", nil, nil},
		{"warnings of a disabled rule with an error are dropped", "rules: [{id: a, enabled: false, func: ['func /x/()', 'func B(,)']}]", []string{
			"1:54: rules[0](a).func[1]: 1:8: expected type, found ','\n    func B(,)\n           ^",
		}, nil},
		{"warnings of an enabled rule", "rules:\n  - id: a\n    stmt: {regex: x}", nil, []string{
			`3:19: rules[0](a).stmt.regex: regex "x" is not anchored with ^ or $, so it can match any part of the normalized text`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := decodeRules(t, tt.src, enabled)
			checkDiags(t, "errors", c.errs, tt.want)
			checkDiags(t, "warnings", c.warns, tt.warns)
		})
	}
}

func TestRulesDiagnosticRuleID(t *testing.T) {
	c, _ := decodeRules(t, "rules:\n  - id: a\n    stmt: {regex: x, kind: Foo}\n  - func: 'func A()'", nil)
	for _, d := range append(c.errs, c.warns...) {
		want := "a"
		if d.Line == 4 {
			want = ""
		}
		if d.RuleID != want {
			t.Errorf("%s: RuleID = %q, want %q", d.Format(""), d.RuleID, want)
		}
	}
	if len(c.errs) != 2 || len(c.warns) != 1 {
		t.Errorf("errors %q, warnings %q", format(c.errs), format(c.warns))
	}
}
