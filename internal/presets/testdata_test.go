package presets

import (
	"errors"
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

// TestTestdata runs testdata/<preset>/<variant>: with the settings in
// settings.yml (defaults when absent), every candidate in match.go matches
// and none in nomatch.go does.
func TestTestdata(t *testing.T) {
	for _, p := range All() {
		dirs, err := os.ReadDir(filepath.Join("testdata", p.Name))
		if err != nil {
			t.Errorf("%s: %v", p.Name, err)
			continue
		}
		variants := map[string]bool{}
		for _, d := range dirs {
			if d.IsDir() {
				variants[d.Name()] = true
				t.Run(p.Name+"/"+d.Name(), func(t *testing.T) {
					runVariant(t, p, filepath.Join("testdata", p.Name, d.Name()))
				})
			}
		}
		common := map[string]bool{"paths": true, "exclude_paths": true, "include_doc": true}
		names := map[string]bool{"default": true}
		for _, s := range Settings(p) {
			names[s.Name] = true
			if !common[s.Name] && !variants[s.Name] {
				t.Errorf("%s: no testdata variant for setting %s", p.Name, s.Name)
			}
		}
		for v := range variants {
			if !names[v] {
				t.Errorf("%s: testdata variant %s is neither default nor a setting", p.Name, v)
			}
		}
	}
}

func runVariant(t *testing.T, p *Preset, dir string) {
	settings, err := os.ReadFile(filepath.Join(dir, "settings.yml"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	r := compileYAML(t, p, string(settings))
	for _, file := range []string{"match.go", "nomatch.go"} {
		src, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		f := parseSource(t, file, src)
		nodes := candidates(t, p, f)
		if len(nodes) == 0 {
			t.Errorf("%s: nothing to test", file)
		}
		want := file == "match.go"
		for _, n := range nodes {
			if got := matches(r, n, f); got != want {
				t.Errorf("%s:%d: match = %v, want %v\n%s", file, f.Line(n.Pos()), got, want, firstLine(f, n))
			}
		}
	}
}

func firstLine(f *rule.File, n ast.Node) string {
	line, _, _ := strings.Cut(string(f.Src[f.Offset(n.Pos()):]), "\n")
	return line
}
