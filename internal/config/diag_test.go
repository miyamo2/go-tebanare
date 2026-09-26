package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// parseYAML returns the document node of the YAML in src.
func parseYAML(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal(%q): %v", src, err)
	}
	return &doc
}

// format renders diagnostics with Diagnostic.Format("").
func format(ds []result.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Format("")
	}
	return out
}

// checkDiags compares diagnostics with want, each formatted with
// Diagnostic.Format("").
func checkDiags(t *testing.T, what string, got []result.Diagnostic, want []string) {
	t.Helper()
	if g := format(got); !slices.Equal(g, want) {
		t.Errorf("%s:\n  %s\nwant:\n  %s", what, strings.Join(g, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestField(t *testing.T) {
	f := field("").key("presets").index(2).named("iferr").key("names").index(0)
	if got, want := string(f), "presets[2](iferr).names[0]"; got != want {
		t.Errorf("field = %q, want %q", got, want)
	}
	if got := field("presets").index(1).named(""); got != "presets[1]" {
		t.Errorf("named(\"\") = %q, want presets[1]", got)
	}
}

func TestSortByPosition(t *testing.T) {
	ds := []result.Diagnostic{
		{Line: 3, Column: 1, Message: "a"},
		{Line: 1, Column: 9, Message: "b"},
		{Message: "c"},
		{Line: 1, Column: 2, Message: "d"},
		{Line: 3, Column: 1, Message: "e"},
	}
	sortByPosition(ds)
	checkDiags(t, "sorted", ds, []string{"c", "1:2: d", "1:9: b", "3:1: a", "3:1: e"})
}

func TestDiagnostics(t *testing.T) {
	c := &compiler{ruleID: "getter"}
	n := &yaml.Node{Line: 2, Column: 5}
	c.errorf(n, "presets[0](getter).max_depth", "bad %s", "depth")
	want := []result.Diagnostic{
		{Severity: result.SeverityError, Code: result.CodeConfigInvalid, Message: "bad depth", Field: "presets[0](getter).max_depth", RuleID: "getter", Line: 2, Column: 5},
	}
	if got := c.errs; !slices.Equal(got, want) {
		t.Errorf("diagnostics = %+v, want %+v", got, want)
	}
}

func TestUnwrapAll(t *testing.T) {
	messages := func(errs []error) []string {
		out := make([]string, len(errs))
		for i, err := range errs {
			out[i] = err.Error()
		}
		return out
	}
	if got := messages(unwrapAll(errors.Join(errors.New("a"), errors.New("b")))); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("unwrapAll(Join) = %q", got)
	}
	if got := messages(unwrapAll(fmt.Errorf("x: %w", errors.New("a")))); !slices.Equal(got, []string{"x: a"}) {
		t.Errorf("unwrapAll(wrapped) = %q", got)
	}
}
