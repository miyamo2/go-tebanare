package presetdoc

import (
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

// testPreset has an enum setting, settings that span two lines, and a
// multi-line example.
var testPreset = tebanare.PresetInfo{
	Name:     "demo",
	Summary:  "Demo statements.",
	Criteria: []string{"It is a demo.", "It has no comment."},
	Kind:     "stmt",
	Settings: []tebanare.SettingInfo{
		{Name: "paths", Type: "[]string", Default: "none", Description: "Globs of files."},
		{Name: "mode", Type: "string", Default: "a", Description: "The mode | or not.", Enum: []string{"a", "b"}},
	},
	Examples: []tebanare.PresetExample{
		{Code: "if x {\n\treturn\n}", Match: true},
		{Code: "if y {}", Note: "It uses y."},
		{Settings: "mode: b\npaths: [\"a/**\"]\n", Code: "if x {}", Match: true, Note: "Mode b."},
	},
}

func TestIndent(t *testing.T) {
	got := indent("a\n\n\tb\n", "  ")
	if want := "  a\n\n  \tb\n"; got != want {
		t.Errorf("indent = %q, want %q", got, want)
	}
}
