package presetdoc

import (
	"strings"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

func TestMarkdownPreset(t *testing.T) {
	var b strings.Builder
	markdownPreset(&b, testPreset)
	want := "" +
		"## demo\n" +
		"\n" +
		"Demo statements. Hides: statements.\n" +
		"\n" +
		"### Criteria\n" +
		"\n" +
		"With the default settings, code matches when all of these hold:\n" +
		"\n" +
		"- It is a demo.\n" +
		"- It has no comment.\n" +
		"\n" +
		"### Settings\n" +
		"\n" +
		"| Setting | Type | Default | Description |\n" +
		"|---|---|---|---|\n" +
		"| `paths` | []string | none | Globs of files. |\n" +
		"| `mode` | string (a, b) | a | The mode \\| or not. |\n" +
		"\n" +
		"### Examples\n" +
		"\n" +
		"With the default settings:\n" +
		"\n" +
		"Matches.\n" +
		"\n" +
		"```go\nif x {\n\treturn\n}\n```\n" +
		"\n" +
		"Does not match. It uses y.\n" +
		"\n" +
		"```go\nif y {}\n```\n" +
		"\n" +
		"With these settings:\n" +
		"\n" +
		"```yaml\nmode: b\npaths: [\"a/**\"]\n```\n" +
		"\n" +
		"Matches. Mode b.\n" +
		"\n" +
		"```go\nif x {}\n```\n"
	if got := b.String(); got != want {
		t.Errorf("markdownPreset:\n%s\nwant:\n%s", got, want)
	}
}

func TestMarkdownIntro(t *testing.T) {
	other := tebanare.PresetInfo{Name: "plain", Kind: "func", Summary: "Plain."}
	got := markdown([]tebanare.PresetInfo{other, testPreset})
	for _, want := range []string{
		"# Presets\n\n<!-- Generated from internal/presets by \"go test -run TestPresetsDocUpToDate -update .\". Do not edit. -->\n",
		"```yaml\nversion: 1\npresets:\n  - plain\n  - demo\n```\n",
		// The first example settings of any preset show the name: settings form.
		"```yaml\npresets:\n  - demo:\n      mode: b\n      paths: [\"a/**\"]\n```\n",
		"| [`plain`](#plain) | function declarations | Plain. |\n" +
			"| [`demo`](#demo) | statements | Demo statements. |\n\n## plain\n",
		"\n## demo\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown does not contain %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "### Examples") {
		t.Errorf("markdown has no examples:\n%s", got)
	}

	// Without example settings the introduction leaves out the second
	// config.
	got = markdown([]tebanare.PresetInfo{other})
	if strings.Contains(got, "To change the settings") {
		t.Errorf("markdown shows settings without examples:\n%s", got)
	}
	if strings.Contains(got, "### Examples") {
		t.Errorf("markdown has an empty Examples section:\n%s", got)
	}
}

func TestMarkdownBuiltin(t *testing.T) {
	md := Markdown()
	if !strings.HasSuffix(md, "\n") || strings.HasSuffix(md, "\n\n") {
		t.Errorf("Markdown() must end with exactly one newline")
	}
	for _, p := range tebanare.Presets() {
		if !strings.Contains(md, "\n## "+p.Name+"\n") {
			t.Errorf("Markdown() has no section for %s", p.Name)
		}
	}
	if n := strings.Count(md, "```"); n%2 != 0 {
		t.Errorf("Markdown() has %d fences, want an even number", n)
	}
	// Every row of a table has as many cell separators as its header.
	cols := 0
	for i, line := range strings.Split(md, "\n") {
		if strings.TrimRight(line, " ") != line {
			t.Errorf("line %d ends with a space: %q", i+1, line)
		}
		if !strings.HasPrefix(line, "|") {
			cols = 0
			continue
		}
		n := strings.Count(line, "|") - strings.Count(line, `\|`)
		if cols == 0 {
			cols = n
		} else if n != cols {
			t.Errorf("line %d has %d separators, want %d: %q", i+1, n, cols, line)
		}
	}
}

func TestCell(t *testing.T) {
	if got, want := cell("a | b\n  c"), `a \| b c`; got != want {
		t.Errorf("cell = %q, want %q", got, want)
	}
}

func TestFence(t *testing.T) {
	for _, tt := range []struct{ code, want string }{
		{"x := 1\n", "```go\nx := 1\n```\n"},
		{"s := `a```b`", "````go\ns := `a```b`\n````\n"},
	} {
		if got := fence("go", tt.code); got != tt.want {
			t.Errorf("fence(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}
