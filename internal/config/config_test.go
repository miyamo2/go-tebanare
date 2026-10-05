package config

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

func readFile(tb testing.TB, name string) []byte {
	tb.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		tb.Fatal(err)
	}
	return src
}

// compileError compiles src and returns the diagnostics of the *Error.
func compileError(t *testing.T, src string) []result.Diagnostic {
	t.Helper()
	set, _, err := Compile([]byte(src))
	var ce *Error
	if !errors.As(err, &ce) || set != nil {
		t.Fatalf("Compile(%q) = %v, %v; want an *Error", src, set, err)
	}
	return ce.Diagnostics
}

type ruleSummary struct {
	ID, Description, Preset string
	Target                  result.Target
	IncludeDoc              bool
	Paths                   string
}

func summarize(rules []*rule.Rule) []ruleSummary {
	out := make([]ruleSummary, len(rules))
	for i, r := range rules {
		out[i] = ruleSummary{r.ID, r.Description, r.Preset, r.Target, r.IncludeDoc, strings.Join(r.Paths, ",")}
	}
	return out
}

func TestCompileMinimal(t *testing.T) {
	set, warns, err := Compile(readFile(t, "testdata/minimal.yml"))
	if err != nil || len(warns) > 0 {
		t.Fatalf("Compile = %v, warnings %q", err, format(warns))
	}
	want := []ruleSummary{
		{"getter", "", "getter", result.TargetFunc, true, ""},
		{"noop", "", "noop", result.TargetFunc, true, ""},
		{"iferr", "", "iferr", result.TargetStmt, false, ""},
	}
	got := summarize(set.Rules)
	for i := range got {
		got[i].Description = ""
	}
	if !slices.Equal(got, want) || set.Include != nil || set.Exclude != nil {
		t.Errorf("set = %+v, rules %+v", set, got)
	}
	if !set.IsTarget("internal/x.go") {
		t.Error("the default include does not apply")
	}
}

func TestCompileFiles(t *testing.T) {
	src := "version: 1\nfiles:\n  include: [\"**/*.go\"]\n  exclude: [\"vendor/**\", \"third_party/**\"]\npresets: [iferr]\n"
	set, warns, err := Compile([]byte(src))
	if err != nil || len(warns) > 0 {
		t.Fatalf("Compile = %v, warnings %q", err, format(warns))
	}
	if !slices.Equal(set.Include, []string{"**/*.go"}) || !slices.Equal(set.Exclude, []string{"vendor/**", "third_party/**"}) {
		t.Errorf("Include = %q, Exclude = %q", set.Include, set.Exclude)
	}
	if !set.IsTarget("internal/x.go") || set.IsTarget("vendor/x/x.go") {
		t.Error("files.exclude does not apply")
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"not a mapping", "- version: 1", []string{"1:1: expected a mapping, found a list"}},
		{"missing version", "\npresets: []", []string{`2:1: missing required key "version"`}},
		{"unknown version", "version: 2", []string{"1:10: version: unsupported version 2 (supported versions: 1)"}},
		{"version as a string", "version: '1'", []string{`1:10: version: expected an integer, found string "1"`}},
		{"unknown key", "version: 1\nrule: []", []string{
			`2:1: rule: unknown key "rule" (allowed keys: version, files, presets)`,
		}},
		// The schema errors stop the compiler, so the glob is not checked.
		{"files", "version: 1\nfiles:\n  include: ['[']\n  exclude: '**'\n  excludes: []", []string{
			`4:12: files.exclude: expected a list of strings, found string "**"`,
			`5:3: files.excludes: unknown key "excludes" (allowed keys: include, exclude)`,
		}},
		{"files not a mapping", "version: 1\nfiles: ['**']", []string{"2:8: files: expected a mapping, found a list"}},
		// The alias check runs first and stops the compiler, so the
		// version and the unknown key are not reported.
		{"aliases past the limit", "version: 3\na: &a " + strings.Repeat("x", maxAliasCopy) + "\npresets: [*a]", []string{
			"3:11: " + tooMuch,
		}},
		{"alias inside its value", "version: 1\npresets: &a [*a]", []string{"2:14: alias *a refers to a value that contains it"}},
		// files is checked before presets; the errors come back sorted.
		{"errors in several sections, sorted", "version: 3\npresets: [x]\nfiles: {include: 1}", []string{
			"1:10: version: unsupported version 3 (supported versions: 1)",
			`2:11: presets[0](x): unknown preset "x" (available presets: getter, iferr, noop)`,
			"3:18: files.include: expected a list of strings, found integer 1",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := compileError(t, tt.src)
			checkDiags(t, "errors", ds, tt.want)
			for _, d := range ds {
				if d.Severity != result.SeverityError || d.Code != result.CodeConfigInvalid {
					t.Errorf("%s: severity %s, code %s", d.Format(""), d.Severity, d.Code)
				}
			}
		})
	}
}

func TestErrorString(t *testing.T) {
	err := &Error{Diagnostics: []result.Diagnostic{
		{Line: 1, Column: 10, Field: "version", Message: "unsupported version 2"},
		{Line: 3, Column: 5, Field: "presets[0]", Message: "a\n  b"},
	}}
	if got, want := err.Error(), "1:10: version: unsupported version 2\n3:5: presets[0]: a\n  b"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if got := (&Error{}).Error(); got != "invalid configuration" {
		t.Errorf("empty Error() = %q", got)
	}
}
