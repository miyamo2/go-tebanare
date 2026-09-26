package tebanare_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/presetdoc"
)

// presetsDoc is the generated presets reference page.
var presetsDoc = filepath.Join("docs", "presets.md")

// TestPresetsDocUpToDate checks that docs/presets.md is the output of
// presetdoc.Markdown. With -update it rewrites the file.
func TestPresetsDocUpToDate(t *testing.T) {
	want := presetdoc.Markdown()
	if *update {
		if err := os.WriteFile(presetsDoc, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(presetsDoc)
	if err != nil {
		t.Fatalf("%v (run go test -run TestPresetsDocUpToDate -update to create it)", err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date (run go test -run TestPresetsDocUpToDate -update to regenerate it)\n%s",
			presetsDoc, firstDiff(string(got), want))
	}
}

// firstDiff describes the first line where got and want differ.
func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < max(len(g), len(w)); i++ {
		if i >= len(g) || i >= len(w) || g[i] != w[i] {
			return fmt.Sprintf("line %d:\n got: %s\nwant: %s", i+1, lineAt(g, i), lineAt(w, i))
		}
	}
	return "the contents differ"
}

// lineAt returns line i of lines, quoted, or "end of file" when lines has
// no line i.
func lineAt(lines []string, i int) string {
	if i >= len(lines) {
		return "end of file"
	}
	return fmt.Sprintf("%q", lines[i])
}
