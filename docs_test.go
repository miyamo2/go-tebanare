package tebanare_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

// TestConfigurationDocErrors checks the error formats that the
// "Validation" section of docs/configuration.md describes, including its
// example error.
func TestConfigurationDocErrors(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("docs", "configuration.md"))
	if err != nil {
		t.Fatal(err)
	}
	yamls, texts := fencedBlocks(string(doc), "yaml"), fencedBlocks(string(doc), "text")
	if len(yamls) == 0 || len(texts) == 0 {
		t.Fatal("configuration.md has no yaml block or no text block")
	}
	if _, _, err := tebanare.Compile([]byte(yamls[0])); err != nil {
		t.Errorf("the example configuration is invalid: %v", err)
	}
	broken := strings.Replace(yamls[0], "- getter", "- geter", 1)
	if got, want := onlyConfigError(t, broken).Format(".gotebanare.yml"), strings.TrimSpace(texts[0]); got != want {
		t.Errorf("example error:\n got: %s\nwant: %s", got, want)
	}
	if d := onlyConfigError(t, "presets: [getter]\n"); d.Field != "" || d.Line == 0 || d.Column == 0 {
		t.Errorf("error about the whole file = %+v, want a line and a column and no field", d)
	}
	if d := onlyConfigError(t, "version: 1\npresets: [getter\n"); d.Field != "" || d.Line == 0 || d.Column != 0 {
		t.Errorf("YAML syntax error = %+v, want a line only", d)
	}
}

// TestConfigurationDocDirectives checks the directive examples of
// docs/configuration.md: a directive keeps the doc comment visible, and a
// comment with a space after the colon is no directive.
func TestConfigurationDocDirectives(t *testing.T) {
	rs := mustCompile(t, "version: 1\npresets: [noop]\n")
	for _, tt := range []struct {
		comment string
		start   int // first hidden line
	}{
		{"//go:linkname f runtime.f", 4},
		{"//nolint:errcheck", 4},
		{"//todo: fix", 2},
		{"//go: x", 2},
	} {
		src := "package p\n// F does nothing.\n" + tt.comment + "\nfunc (T) F() {}\n"
		res := rs.AnalyzeChange(tebanare.FileChange{NewPath: "p.go", New: []byte(src)})
		if len(res.New) != 1 || res.New[0].Start != tt.start || res.New[0].End != 4 {
			t.Errorf("%s: hidden %+v, want lines %d-4", tt.comment, res.New, tt.start)
		}
	}
}

// onlyConfigError compiles src and returns its error, which must be the
// only one.
func onlyConfigError(t *testing.T, src string) tebanare.Diagnostic {
	t.Helper()
	_, _, err := tebanare.Compile([]byte(src))
	var ce *tebanare.ConfigError
	if !errors.As(err, &ce) || len(ce.Diagnostics) != 1 {
		t.Fatalf("Compile(%q) error = %v, want exactly one error", src, err)
	}
	return ce.Diagnostics[0]
}

// fencedBlocks returns the contents of the fenced code blocks of doc whose
// info string is lang, in order.
func fencedBlocks(doc, lang string) []string {
	var blocks []string
	var b strings.Builder
	in := false
	for line := range strings.Lines(doc) {
		switch fence := strings.TrimSpace(line); {
		case !in && fence == "```"+lang:
			in = true
			b.Reset()
		case in && fence == "```":
			in = false
			blocks = append(blocks, b.String())
		case in:
			b.WriteString(line)
		}
	}
	return blocks
}
