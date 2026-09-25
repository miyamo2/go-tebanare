package presetdoc

import (
	"strings"

	tebanare "github.com/miyamo2/go-tebanare"
)

// Markdown returns the presets reference page, docs/presets.md. The output
// ends with one newline.
func Markdown() string {
	return markdown(tebanare.Presets())
}

func markdown(ps []tebanare.PresetInfo) string {
	var b strings.Builder
	b.WriteString("# Presets\n\n")
	b.WriteString("<!-- Generated from internal/presets by \"go test -run TestPresetsDocUpToDate -update .\". Do not edit. -->\n\n")
	b.WriteString("Presets are built-in rules. A configuration enables a preset by listing its name under `presets`:\n\n")
	b.WriteString("```yaml\nversion: 1\npresets:\n")
	for _, p := range ps {
		b.WriteString("  - " + p.Name + "\n")
	}
	b.WriteString("```\n\n")
	if name, settings, ok := firstSettings(ps); ok {
		b.WriteString("To change the settings of a preset, write its name as a key and the settings as the value:\n\n")
		b.WriteString("```yaml\npresets:\n  - " + name + ":\n" + indent(settings, "      ") + "```\n\n")
	}
	b.WriteString("Settings that the configuration leaves out keep their defaults. ")
	b.WriteString("The criteria describe each preset with the default settings, ")
	b.WriteString("and the examples under \"With these settings\" show what a setting changes.\n\n")

	b.WriteString("| Preset | Hides | Summary |\n|---|---|---|\n")
	for _, p := range ps {
		b.WriteString("| [`" + p.Name + "`](#" + p.Name + ") | " + hides(p.Kind) + " | " + cell(p.Summary) + " |\n")
	}

	for _, p := range ps {
		b.WriteString("\n")
		markdownPreset(&b, p)
	}
	return b.String()
}

func markdownPreset(b *strings.Builder, p tebanare.PresetInfo) {
	b.WriteString("## " + p.Name + "\n\n")
	b.WriteString(p.Summary + " Hides: " + hides(p.Kind) + ".\n\n")

	b.WriteString("### Criteria\n\nWith the default settings, code matches when all of these hold:\n\n")
	for _, c := range p.Criteria {
		b.WriteString("- " + c + "\n")
	}

	if len(p.Settings) > 0 {
		b.WriteString("\n### Settings\n\n| Setting | Type | Default | Description |\n|---|---|---|---|\n")
		for _, s := range p.Settings {
			b.WriteString("| `" + s.Name + "` | " + cell(settingType(s)) + " | " + cell(s.Default) + " | " + cell(s.Description) + " |\n")
		}
	}

	if len(p.Examples) > 0 {
		b.WriteString("\n### Examples\n")
	}
	for _, g := range groupExamples(p.Examples) {
		if g.settings == "" {
			b.WriteString("\nWith the default settings:\n")
		} else {
			b.WriteString("\nWith these settings:\n\n")
			b.WriteString(fence("yaml", g.settings))
		}
		for _, ex := range g.examples {
			b.WriteString("\n" + verdict(ex) + "\n\n")
			b.WriteString(fence("go", ex.Code))
		}
	}
}

// firstSettings returns the first example settings of any preset, for the
// introduction.
func firstSettings(ps []tebanare.PresetInfo) (name, settings string, ok bool) {
	for _, p := range ps {
		for _, ex := range p.Examples {
			if s := strings.TrimSpace(ex.Settings); s != "" {
				return p.Name, s, true
			}
		}
	}
	return "", "", false
}

// cell escapes s for a table cell: pipes are escaped and line breaks
// become spaces.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.Join(strings.Fields(s), " ")
}

// fence returns code in a fenced code block. The fence is longer than any
// run of backticks in code.
func fence(lang, code string) string {
	f := "```"
	for strings.Contains(code, f) {
		f += "`"
	}
	return f + lang + "\n" + strings.TrimRight(code, "\n") + "\n" + f + "\n"
}
