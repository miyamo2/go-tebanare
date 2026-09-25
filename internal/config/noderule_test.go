package config

import (
	"go/ast"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/nodematch"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// decodeNodeRule decodes the stmt or expr mapping in src into a rule.
func decodeNodeRule(t *testing.T, src string, target result.Target) (*compiler, *rule.Rule, bool) {
	t.Helper()
	c := &compiler{}
	r := &rule.Rule{Target: target}
	ok := c.nodeRule(ruleEntry(t, c, src, string(target)), r)
	return c, r, ok
}

func TestNodeRuleStmt(t *testing.T) {
	c, r, ok := decodeNodeRule(t, `
stmt:
  kind: [AssignStmt, DeferStmt]
  regex:
    - '^ctx, span := \w+\.Start\(ctx, .+\)$'
    - '^defer span\.End\(\)$'
  include_leading_comments: true
`, result.TargetStmt)
	if !ok || len(c.errs) > 0 || len(c.warns) > 0 {
		t.Fatalf("nodeRule = %v, errors %q, warnings %q", ok, format(c.errs), format(c.warns))
	}
	m, isMatcher := r.Node.(*nodematch.Matcher)
	if !isMatcher {
		t.Fatalf("Node = %T, want *nodematch.Matcher", r.Node)
	}
	if !r.IncludeLeadingComments || r.Hide != rule.HideSelf {
		t.Errorf("rule = %+v", r)
	}
	if !m.Accepts(&ast.DeferStmt{}) || m.Accepts(&ast.IfStmt{}) {
		t.Error("Accepts does not follow kind")
	}
	for text, want := range map[string]bool{
		"ctx, span := tracer.Start(ctx, \"find\")": true,
		"defer span.End()":                         true,
		"defer span.End(x)":                        false,
	} {
		if got := m.MatchText(text); got != want {
			t.Errorf("MatchText(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestNodeRuleExpr(t *testing.T) {
	c, r, ok := decodeNodeRule(t, `
expr:
  kind: CallExpr
  regex: '^(log|slog|logger)\.Debug\w*\('
  not_regex: '(?i)password|token|secret'
  hide: statement
`, result.TargetExpr)
	if !ok || len(c.errs) > 0 || len(c.warns) > 0 {
		t.Fatalf("nodeRule = %v, errors %q, warnings %q", ok, format(c.errs), format(c.warns))
	}
	if r.Hide != rule.HideStatement || r.IncludeLeadingComments {
		t.Errorf("rule = %+v", r)
	}
	m := r.Node.(*nodematch.Matcher)
	if !m.MatchText(`log.Debugf("x=%d", x)`) || m.MatchText(`log.Debug("token", t)`) {
		t.Error("MatchText does not follow regex and not_regex")
	}
	if !m.Accepts(&ast.CallExpr{}) || m.Accepts(&ast.BinaryExpr{}) {
		t.Error("Accepts does not follow kind")
	}

	_, r, _ = decodeNodeRule(t, "expr: {regex: '^a', hide: self}", result.TargetExpr)
	if r.Hide != rule.HideSelf {
		t.Errorf("hide: self gives %v", r.Hide)
	}
}

func TestNodeRuleErrors(t *testing.T) {
	stmt, expr := result.TargetStmt, result.TargetExpr
	tests := []struct {
		name   string
		src    string
		target result.Target
		want   []string
		warns  []string
	}{
		{"not a mapping", "stmt: '^a$'", stmt, []string{
			`1:7: rules[0](x).stmt: expected a mapping, found string "^a$"`,
		}, nil},
		{"missing regex", "stmt:\n  kind: IfStmt", stmt, []string{
			`2:3: rules[0](x).stmt: missing required key "regex"`,
		}, nil},
		{"empty regex list", "stmt: {regex: []}", stmt, []string{
			`1:15: rules[0](x).stmt.regex: at least one regular expression is required`,
		}, nil},
		{"only bad regex items", "stmt: {regex: [1]}", stmt, []string{
			`1:16: rules[0](x).stmt.regex[0]: expected a string, found integer 1`,
		}, nil},
		{"invalid regex", "stmt: {regex: '^(a'}", stmt, []string{
			"1:15: rules[0](x).stmt.regex: error parsing regexp: missing closing ): `^(a`",
		}, nil},
		{"invalid regex in a list", "stmt:\n  regex:\n    - '^a$'\n    - '^(b'", stmt, []string{
			"4:7: rules[0](x).stmt.regex[1]: error parsing regexp: missing closing ): `^(b`",
		}, nil},
		{"invalid not_regex", "expr: {regex: '^a', not_regex: [b, '(']}", expr, []string{
			"1:36: rules[0](x).expr.not_regex[1]: error parsing regexp: missing closing ): `(`",
		}, nil},
		{"unknown kind", "stmt: {regex: '^a', kind: CallExpr}", stmt, []string{
			`1:27: rules[0](x).stmt.kind: unknown stmt kind "CallExpr" (valid kinds: ` +
				`AssignStmt, BlockStmt, BranchStmt, CaseClause, CommClause, DeclStmt, DeferStmt, EmptyStmt, ExprStmt, ` +
				`ForStmt, GoStmt, IfStmt, IncDecStmt, LabeledStmt, RangeStmt, ReturnStmt, SelectStmt, SendStmt, ` +
				`SwitchStmt, TypeSwitchStmt)`,
		}, nil},
		{"unknown kind in a list", "expr: {regex: '^a', kind: [CallExpr, IfStmt]}", expr, []string{
			`1:38: rules[0](x).expr.kind[1]: unknown expr kind "IfStmt" (valid kinds: ` +
				`ArrayType, BasicLit, BinaryExpr, CallExpr, ChanType, CompositeLit, Ellipsis, FuncLit, FuncType, Ident, ` +
				`IndexExpr, IndexListExpr, InterfaceType, KeyValueExpr, MapType, ParenExpr, SelectorExpr, SliceExpr, ` +
				`StarExpr, StructType, TypeAssertExpr, UnaryExpr)`,
		}, nil},
		{"hide on a stmt rule", "stmt: {regex: '^a', hide: self}", stmt, []string{
			`1:21: rules[0](x).stmt.hide: unknown key "hide" (allowed keys: kind, regex, not_regex, include_leading_comments)`,
		}, nil},
		{"unknown hide value", "expr: {regex: '^a', hide: line}", expr, []string{
			`1:27: rules[0](x).expr.hide: unknown value "line" (valid values: self, statement)`,
		}, nil},
		{"include_leading_comments of the wrong type", "stmt: {regex: '^a', include_leading_comments: yes}", stmt, []string{
			`1:47: rules[0](x).stmt.include_leading_comments: expected a boolean, found string "yes"`,
		}, nil},
		{"typo in a key", "stmt: {regx: '^a'}", stmt, []string{
			`1:7: rules[0](x).stmt: missing required key "regex"`,
			`1:8: rules[0](x).stmt.regx: unknown key "regx" (allowed keys: kind, regex, not_regex, include_leading_comments)`,
		}, nil},
		{"unanchored regex", "expr: {regex: ['a', '^b', 'c$', '^d|e', '('], not_regex: 'f'}", expr, []string{
			"1:41: rules[0](x).expr.regex[4]: error parsing regexp: missing closing ): `(`",
		}, []string{
			`1:16: rules[0](x).expr.regex[0]: regex "a" is not anchored with ^ or $, so it can match any part of the normalized text`,
			`1:33: rules[0](x).expr.regex[3]: regex "^d|e" is not anchored with ^ or $, so it can match any part of the normalized text`,
		}},
		{"unanchored scalar regex", "stmt: {regex: 'a'}", stmt, nil, []string{
			`1:15: rules[0](x).stmt.regex: regex "a" is not anchored with ^ or $, so it can match any part of the normalized text`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, r, ok := decodeNodeRule(t, tt.src, tt.target)
			if ok != (len(tt.want) == 0) || (r.Node != nil) != ok {
				t.Errorf("nodeRule ok = %v, Node = %v", ok, r.Node)
			}
			sortByPosition(c.errs)
			checkDiags(t, "errors", c.errs, tt.want)
			checkDiags(t, "warnings", c.warns, tt.warns)
		})
	}
}
