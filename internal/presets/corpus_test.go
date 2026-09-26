package presets

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

var update = flag.Bool("update", false, "rewrite testdata/corpus_snapshot.json")

const snapshotFile = "testdata/corpus_snapshot.json"

// corpusSnapshot counts the matches of each preset, with default settings,
// in the non-test Go files of one Go release's GOROOT/src.
type corpusSnapshot struct {
	GoVersion string `json:"goVersion"`
	Files     int    `json:"files"`
	Getter    int    `json:"getter"`
	Noop      int    `json:"noop"`
	Iferr     int    `json:"iferr"`
}

// TestCorpusSnapshot guards against unnoticed changes to what the presets
// match. A changed count fails the test. Rewrite the snapshot with -update
// only after checking that the change narrows the matches or fixes a bug.
//
// The counts hold for one Go release. Locally, another release skips the
// test; in CI it fails, so that bumping CI's Go version without rewriting
// the snapshot cannot turn the test into a silent no-op.
func TestCorpusSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("walks GOROOT/src")
	}
	var want corpusSnapshot
	data, err := os.ReadFile(snapshotFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !*update && runtime.Version() != want.GoVersion {
		skipOutsideCI(t, "snapshot is for %s, running %s; run with -update under %s after checking the new counts",
			want.GoVersion, runtime.Version(), runtime.Version())
	}
	goroot := findGOROOT(t)

	got := corpusSnapshot{GoVersion: runtime.Version()}
	counts := map[string]*int{"getter": &got.Getter, "noop": &got.Noop, "iferr": &got.Iferr}
	rules := map[string]*rule.Rule{}
	for name := range counts {
		p, ok := Lookup(name)
		if !ok {
			t.Fatalf("preset %s is not registered", name)
		}
		rules[name] = compileYAML(t, p, "")
	}
	src := filepath.Join(goroot, "src")
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f := parseSource(t, path, b)
		got.Files++
		for name, r := range rules {
			*counts[name] += countMatches(r, f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", got)

	if *update {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(got); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(snapshotFile, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if got != want {
		t.Errorf("corpus counts changed:\n got %+v\nwant %+v", got, want)
	}
}

// skipOutsideCI skips the test, or fails it when the CI environment
// variable is set, as it is on GitHub Actions.
func skipOutsideCI(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

// findGOROOT returns the GOROOT of the go command, and skips the test (fails
// it in CI) when that GOROOT belongs to another Go release than the test
// binary.
func findGOROOT(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		skipOutsideCI(t, "go env GOROOT: %v", err)
	}
	goroot := strings.TrimSpace(string(out))
	version, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		skipOutsideCI(t, "read GOROOT/VERSION: %v", err)
	}
	if v, _, _ := strings.Cut(string(version), "\n"); v != runtime.Version() {
		skipOutsideCI(t, "GOROOT %s holds %s, running %s", goroot, v, runtime.Version())
	}
	return goroot
}

// countMatches counts the matches of r in f: top-level functions for func
// rules, and accepted statements anywhere in the file for stmt rules.
func countMatches(r *rule.Rule, f *rule.File) int {
	n := 0
	if r.Func != nil {
		for _, d := range f.AST.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && r.Func.MatchFunc(fd, f) {
				n++
			}
		}
		return n
	}
	ast.Inspect(f.AST, func(node ast.Node) bool {
		if node != nil && r.Node.Accepts(node) {
			if _, ok := r.Node.MatchNode(node, f); ok {
				n++
			}
		}
		return true
	})
	return n
}
