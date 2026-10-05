package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// decodePresets decodes the `presets` list of src.
func decodePresets(t *testing.T, src string) (*compiler, []*rule.Rule) {
	t.Helper()
	var f File
	if err := yaml.Unmarshal([]byte(src), &f); err != nil || f.Presets == nil {
		t.Fatalf("yaml.Unmarshal(%q) = %v, presets %v", src, err, f.Presets)
	}
	c := &compiler{}
	rules := c.presets(f.Presets)
	sortByPosition(c.errs)
	return c, rules
}

// firstIf parses src, a function body, and returns its first if statement.
func firstIf(t *testing.T, body string) (*ast.IfStmt, *rule.File) {
	t.Helper()
	src := []byte("package p\n\nfunc f() error {\n" + body + "\nreturn nil\n}\n")
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found *ast.IfStmt
	ast.Inspect(af, func(n ast.Node) bool {
		if s, ok := n.(*ast.IfStmt); ok && found == nil {
			found = s
		}
		return found == nil
	})
	if found == nil {
		t.Fatalf("no if statement in %q", body)
	}
	return found, &rule.File{Path: "x.go", Fset: fset, AST: af, Src: src}
}

func TestPresets(t *testing.T) {
	c, rules := decodePresets(t, `
presets:
  - getter:
      include_doc: false
  - noop: {}
  - iferr:
      names: [err, "*Err"]
`)
	if len(c.errs) > 0 {
		t.Fatalf("errors %q", format(c.errs))
	}
	var ids []string
	for _, r := range rules {
		ids = append(ids, r.ID)
		if r.Preset != r.ID {
			t.Errorf("rule %s has Preset %q", r.ID, r.Preset)
		}
	}
	if want := []string{"getter", "noop", "iferr"}; !slices.Equal(ids, want) {
		t.Fatalf("ids = %q, want %q", ids, want)
	}
	if rules[0].IncludeDoc || !rules[1].IncludeDoc || rules[2].Target != result.TargetStmt {
		t.Errorf("rules = %+v, %+v, %+v", rules[0], rules[1], rules[2])
	}
	s, f := firstIf(t, "if parseErr != nil { return parseErr }")
	if _, ok := rules[2].Node.MatchNode(s, f); !ok {
		t.Error("iferr with names [err, *Err] does not match parseErr")
	}
}

// TestPresetsPlanSettings decodes the preset settings example of plan 4.5.
func TestPresetsPlanSettings(t *testing.T) {
	c, rules := decodePresets(t, `
presets:
  - getter                    # enabled with the default settings
  - noop:
      include_functions: true # also hide empty functions that are not methods
  - iferr:
      names: [err, "*Err"]    # also match parseErr and similar names
      paths: ["internal/**"]  # limit the target files of this preset only
`)
	if len(c.errs) > 0 || len(c.warns) > 0 {
		t.Fatalf("errors %q, warnings %q", format(c.errs), format(c.warns))
	}
	if len(rules) != 3 {
		t.Fatalf("rules = %v", rules)
	}
	getter, noop, iferr := rules[0], rules[1], rules[2]
	if !getter.IncludeDoc || getter.Paths != nil || noop.Paths != nil {
		t.Errorf("getter = %+v, noop = %+v", getter, noop)
	}

	src := []byte("package p\n\nfunc noop() {}\n")
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	fd, _ := af.Decls[0].(*ast.FuncDecl)
	file := &rule.File{Path: "x.go", Fset: fset, AST: af, Src: src}
	if !noop.Func.MatchFunc(fd, file) {
		t.Error("noop with include_functions: true does not match func noop() {}")
	}
	if _, defaults := decodePresets(t, "presets: [noop]"); defaults[0].Func.MatchFunc(fd, file) {
		t.Error("noop with the default settings matches func noop() {}")
	}

	if !slices.Equal(iferr.Paths, []string{"internal/**"}) || !iferr.AppliesTo("internal/x/x.go") || iferr.AppliesTo("cmd/x/x.go") {
		t.Errorf("iferr Paths = %q", iferr.Paths)
	}
	s, f := firstIf(t, "if parseErr != nil { return parseErr }")
	if _, ok := iferr.Node.MatchNode(s, f); !ok {
		t.Error("iferr with names [err, *Err] does not match parseErr")
	}
}

func TestPresetsDefaults(t *testing.T) {
	c, rules := decodePresets(t, "presets: [getter, 'noop', iferr: , noop2: ~]")
	checkDiags(t, "errors", c.errs, []string{
		`1:36: presets[3](noop2): unknown preset "noop2" (available presets: getter, iferr, noop)`,
	})
	if len(rules) != 3 || !rules[0].IncludeDoc || !rules[1].IncludeDoc {
		t.Fatalf("rules = %v", rules)
	}
	s, f := firstIf(t, "if parseErr != nil { return parseErr }")
	if _, ok := rules[2].Node.MatchNode(s, f); ok {
		t.Error("iferr with default names matches parseErr")
	}
}

func TestPresetsErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"entry of the wrong type", "presets: [1, [getter]]", []string{
			"1:11: presets[0]: expected a preset name or a mapping from a preset name to its settings, found integer 1",
			"1:14: presets[1]: expected a preset name or a mapping from a preset name to its settings, found a list",
		}},
		{"mapping with two keys", "presets:\n  - getter:\n    noop:", []string{
			"2:5: presets[0]: expected a mapping with one key, the preset name, found 2 keys",
		}},
		{"empty mapping", "presets: [{}]", []string{
			"1:11: presets[0]: expected a mapping with one key, the preset name, found 0 keys",
		}},
		{"key that is not a string", "presets: [{1: {}}]", []string{
			"1:12: presets[0]: expected a preset name as the key, found integer 1",
		}},
		{"unknown preset", "presets: [getter, gettr]", []string{
			`1:19: presets[1](gettr): unknown preset "gettr" (available presets: getter, iferr, noop)`,
		}},
		{"duplicate preset", "presets:\n  - getter\n  - noop\n  - getter: {max_depth: 1}", []string{
			`4:5: presets[2](getter): preset "getter" is already enabled on line 2`,
		}},
		{"unknown setting", "presets:\n  - iferr:\n      nmes: [err]", []string{
			`3:7: presets[0](iferr).nmes: unknown setting "nmes" (available settings: ` +
				`paths, exclude_paths, names, allow_comments, init, allow_bare_return, allow_calls_in_results)`,
		}},
		{"settings of the wrong type", "presets:\n  - getter: {max_depth: x, include_doc: 1}", []string{
			`2:25: presets[0](getter).max_depth: expected an integer, found string "x"`,
			"2:41: presets[0](getter).include_doc: expected a boolean, found integer 1",
		}},
		{"invalid settings", "presets:\n  - getter: {max_depth: 0}\n  - iferr: {init: fold, names: ['a b'], paths: ['[']}", []string{
			"2:25: presets[0](getter).max_depth: must be at least 1, found 0",
			`3:19: presets[1](iferr).init: unknown value "fold" (valid values: exclude, fold-body)`,
			`3:33: presets[1](iferr).names[0]: invalid name "a b": use letters, digits, "_", "*", and "?"`,
			`3:49: presets[1](iferr).paths[0]: invalid glob "["`,
		}},
		{"settings that are not a mapping", "presets:\n  - noop: [x]", []string{
			"2:11: presets[0](noop): settings must be a mapping, found a list",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := decodePresets(t, tt.src)
			checkDiags(t, "errors", c.errs, tt.want)
			for _, d := range c.errs {
				if d.Severity != result.SeverityError || d.Code != result.CodeConfigInvalid {
					t.Errorf("%s: severity %s, code %s", d.Format(""), d.Severity, d.Code)
				}
			}
		})
	}
}

func TestPresetsDiagnosticRuleID(t *testing.T) {
	c, _ := decodePresets(t, "presets:\n  - getter: {max_depth: 0}\n  - 1\n  - nope")
	if len(c.errs) != 3 || c.errs[0].RuleID != "getter" || c.errs[1].RuleID != "" || c.errs[2].RuleID != "" {
		t.Errorf("errors = %+v", c.errs)
	}
}
