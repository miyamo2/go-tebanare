package tebanare_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

var update = flag.Bool("update", false, "rewrite testdata/analyze/*/want.json")

// goldenRoot holds one directory per case. The TypeScript engine package
// runs the same cases in its contract test.
var goldenRoot = filepath.Join("testdata", "analyze")

// goldenFiles lists the files a case directory may hold. config.yml and
// want.json are required.
var goldenFiles = map[string]bool{
	"config.yml": true,
	"old.go":     true,
	"new.go":     true,
	"meta.json":  true,
	"want.json":  true,
}

// goldenMeta holds the paths of the two sides. A path left empty is
// "x.go" when the side's file exists.
type goldenMeta struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
}

func TestGolden(t *testing.T) {
	entries, err := os.ReadDir(goldenRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("no cases in %s", goldenRoot)
	}
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("%s: want only case directories", filepath.Join(goldenRoot, e.Name()))
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			runGolden(t, filepath.Join(goldenRoot, e.Name()))
		})
	}
}

func runGolden(t *testing.T, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !goldenFiles[e.Name()] {
			t.Errorf("unexpected file %s", filepath.Join(dir, e.Name()))
		}
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	ch := tebanare.FileChange{
		Old: readOptional(t, filepath.Join(dir, "old.go")),
		New: readOptional(t, filepath.Join(dir, "new.go")),
	}
	var meta goldenMeta
	if b := readOptional(t, filepath.Join(dir, "meta.json")); b != nil {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&meta); err != nil {
			t.Fatalf("meta.json: %v", err)
		}
	}
	ch.OldPath, ch.NewPath = meta.OldPath, meta.NewPath
	if ch.Old != nil && ch.OldPath == "" {
		ch.OldPath = "x.go"
	}
	if ch.New != nil && ch.NewPath == "" {
		ch.NewPath = "x.go"
	}

	rs, warns, err := tebanare.Compile(cfg)
	if err != nil {
		t.Fatalf("config.yml: %v", err)
	}
	for _, w := range warns {
		t.Logf("config.yml: %s", w.Format(""))
	}
	got := encodeGolden(t, rs.AnalyzeChange(ch))

	wantPath := filepath.Join(dir, "want.json")
	if *update {
		if err := os.WriteFile(wantPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("%v (run go test -run TestGolden -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("result differs from %s (run go test -run TestGolden -update to accept it)\ngot:\n%s\nwant:\n%s", wantPath, got, want)
	}
}

// readOptional returns the content of path, or nil when it does not exist.
// An existing empty file gives an empty, non-nil slice.
func readOptional(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	if b == nil {
		b = []byte{}
	}
	return b
}

// encodeGolden returns res as JSON indented by two spaces with a trailing
// newline. HTML characters such as < stay unescaped for readability.
func encodeGolden(t *testing.T, res tebanare.ChangeResult) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
