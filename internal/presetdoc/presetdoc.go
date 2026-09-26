// Package presetdoc renders the descriptions of the built-in presets as
// the Markdown page docs/presets.md. The page comes from tebanare.Presets,
// so it follows the preset declarations.
package presetdoc

import (
	"strings"

	tebanare "github.com/miyamo2/go-tebanare"
)

// hides describes what a preset of the given kind hides.
func hides(kind string) string {
	switch kind {
	case "func":
		return "function declarations"
	case "stmt":
		return "statements"
	}
	return kind
}

// settingType returns the type of s, followed by the allowed values when
// s has an enum.
func settingType(s tebanare.SettingInfo) string {
	if len(s.Enum) == 0 {
		return s.Type
	}
	return s.Type + " (" + strings.Join(s.Enum, ", ") + ")"
}

// verdict returns "Matches." or "Does not match.", followed by the note of
// ex.
func verdict(ex tebanare.PresetExample) string {
	v := "Does not match."
	if ex.Match {
		v = "Matches."
	}
	if ex.Note != "" {
		v += " " + ex.Note
	}
	return v
}

// exampleGroup holds the examples that share one settings text.
type exampleGroup struct {
	settings string
	examples []tebanare.PresetExample
}

// groupExamples groups exs by their settings, in the order in which each
// settings text first appears.
func groupExamples(exs []tebanare.PresetExample) []exampleGroup {
	var groups []exampleGroup
	index := map[string]int{}
	for _, ex := range exs {
		key := strings.TrimSpace(ex.Settings)
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, exampleGroup{settings: key})
		}
		groups[i].examples = append(groups[i].examples, ex)
	}
	return groups
}

// indent prefixes every non-empty line of s with prefix and ends the
// result with a newline.
func indent(s, prefix string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if line != "" {
			b.WriteString(prefix)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
