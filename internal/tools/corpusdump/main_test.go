package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

const getterGo = "package x\n\nfunc (u *U) Name() string { return u.name }\n"

// writeTree writes files, keyed by slash-separated paths, under a new
// temporary directory and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runDump(args ...string) (status int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	status = run(args, &out, &errOut)
	return status, out.String(), errOut.String()
}

func TestDump(t *testing.T) {
	cfg := filepath.Join(writeTree(t, map[string]string{"c.yml": "version: 1\npresets: [getter]\n"}), "c.yml")
	root := writeTree(t, map[string]string{
		"a.go":              getterGo,
		"a/b.go":            "package a\n",
		"b/c.go":            "package b\n",
		"b/c_test.go":       getterGo,
		"b/testdata/t.go":   getterGo,
		"vendor/v/v.go":     getterGo,
		".git/g.go":         getterGo,
		"_old/o.go":         getterGo,
		"notes.txt":         "hello",
		"z/bad.go":          "package",
		"z/deep/dir/tag.go": "package tag // <b> & </b>\n",
	})

	status, stdout, stderr := runDump("-config", cfg, "-root", root)
	if status != 0 || stderr != "" {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	var got struct {
		Files map[string]tebanare.ChangeResult `json:"files"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("%v:\n%s", err, stdout)
	}
	// One line per file, sorted by path: a.go sorts before a/b.go.
	wantKeys := []string{"a.go", "a/b.go", "b/c.go", "z/bad.go", "z/deep/dir/tag.go"}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != len(wantKeys)+2 || lines[0] != `{"files":{` || lines[len(lines)-1] != "}}" {
		t.Fatalf("unexpected layout:\n%s", stdout)
	}
	for i, key := range wantKeys {
		if !strings.HasPrefix(lines[i+1], `"`+key+`":{`) {
			t.Errorf("line %d = %q, want the result of %s", i+2, lines[i+1], key)
		}
	}
	if len(got.Files) != len(wantKeys) {
		t.Errorf("files = %d, want %d", len(got.Files), len(wantKeys))
	}
	if r := got.Files["a.go"].New; len(r) != 1 || r[0].Start != 3 || r[0].End != 3 || r[0].Hits[0].RuleID != "getter" {
		t.Errorf("a.go: new = %+v, want the getter on line 3", r)
	}
	if got.Files["z/bad.go"].Skipped != "parse-error" {
		t.Errorf("z/bad.go: %+v, want a parse error", got.Files["z/bad.go"])
	}
	if !strings.Contains(stdout, `"a/b.go":{"old":[],"new":[],"diagnostics":[],"skipped":""}`) {
		t.Errorf("empty results must hold arrays:\n%s", stdout)
	}

	status, limited, _ := runDump("-config", cfg, "-root", root, "-limit", "2")
	want := "{\"files\":{\n" + lines[1] + "\n" + strings.TrimSuffix(lines[2], ",") + "\n}}\n"
	if status != 0 || limited != want {
		t.Errorf("-limit 2: status %d, output:\n%s\nwant:\n%s", status, limited, want)
	}

	status, td, _ := runDump("-config", cfg, "-root", filepath.Join(root, "b", "testdata"))
	if status != 0 || !strings.HasPrefix(td, "{\"files\":{\n\"t.go\":{") {
		t.Errorf("root named testdata: status %d, output %q; want its files", status, td)
	}
}

func TestDumpSymlinkRoot(t *testing.T) {
	dir := writeTree(t, map[string]string{"c.yml": "version: 1\npresets: [getter]\n", "real/p/a.go": getterGo})
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "real"), link); err != nil {
		t.Skipf("cannot create a symbolic link: %v", err)
	}
	cfg := filepath.Join(dir, "c.yml")
	_, want, _ := runDump("-config", cfg, "-root", filepath.Join(dir, "real"))
	status, got, stderr := runDump("-config", cfg, "-root", link)
	if status != 0 || got != want || !strings.Contains(got, `"p/a.go":{"old":[],"new":[{"start":3`) {
		t.Errorf("status %d, stderr %q, output:\n%s\nwant the getter in p/a.go:\n%s", status, stderr, got, want)
	}
}

// TestDumpBadRoot checks that a root without Go files to analyze is an
// error, so that a parity test never compares an empty corpus.
func TestDumpBadRoot(t *testing.T) {
	cfg := filepath.Join(writeTree(t, map[string]string{"c.yml": "version: 1\n"}), "c.yml")
	root := writeTree(t, map[string]string{"a.go": getterGo, "b/b_test.go": getterGo, "v/vendor/v.go": getterGo})
	for _, tt := range []struct{ root, stderr string }{
		{filepath.Join(root, "none"), "corpusdump: stat " + filepath.Join(root, "none") + ": no such file or directory\n"},
		{filepath.Join(root, "a.go"), "corpusdump: " + filepath.Join(root, "a.go") + " is not a directory\n"},
		{filepath.Join(root, "b"), "corpusdump: no Go files under " + filepath.Join(root, "b") + "\n"},
		{filepath.Join(root, "v"), "corpusdump: no Go files under " + filepath.Join(root, "v") + "\n"},
		{t.TempDir(), "corpusdump: no Go files under "},
	} {
		status, stdout, stderr := runDump("-config", cfg, "-root", tt.root)
		if status != 1 || stdout != "" || !strings.HasPrefix(stderr, tt.stderr) {
			t.Errorf("-root %s: status %d, stdout %q, stderr %q; want status 1 and %q", tt.root, status, stdout, stderr, tt.stderr)
		}
	}
}

func TestDumpErrors(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"bad.yml": "version: 2\n",
		"ok.yml":  "version: 1\npresets: [getter]\n",
		"x.go":    getterGo,
	})
	bad, ok := filepath.Join(dir, "bad.yml"), filepath.Join(dir, "ok.yml")
	for _, tt := range []struct {
		args   []string
		status int
		stderr []string
	}{
		{[]string{"-config", bad, "-root", dir}, 1,
			[]string{bad + ":1:10: version: unsupported version 2", "corpusdump: " + bad + " is invalid"}},
		{[]string{"-config", filepath.Join(dir, "none.yml"), "-root", dir}, 1, []string{"corpusdump: open "}},
		{[]string{"-root", dir}, 2, []string{"usage: corpusdump"}},
		{[]string{"-config", ok}, 2, []string{"usage: corpusdump"}},
		{[]string{"-config", ok, "-root", dir, "extra"}, 2, []string{"usage: corpusdump"}},
		{[]string{"-config", ok, "-root", dir, "-limit", "-1"}, 2, []string{"usage: corpusdump"}},
		{[]string{"-nope"}, 2, []string{"flag provided but not defined: -nope"}},
		{[]string{"-h"}, 0, []string{"Usage of corpusdump"}},
	} {
		status, _, stderr := runDump(tt.args...)
		if status != tt.status {
			t.Errorf("%q: status %d, want %d; stderr:\n%s", tt.args, status, tt.status, stderr)
		}
		for _, s := range tt.stderr {
			if !strings.Contains(stderr, s) {
				t.Errorf("%q: stderr does not contain %q:\n%s", tt.args, s, stderr)
			}
		}
	}
}

func TestGoFiles(t *testing.T) {
	root := writeTree(t, map[string]string{"b.go": "", "a/x.go": "", "a.go": "", "a/x_test.go": "", "a/.x/y.go": ""})
	got, err := goFiles(root)
	if want := []string{"a.go", "a/x.go", "b.go"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("goFiles = %q, %v; want %q", got, err, want)
	}
}

func TestDumpPairs(t *testing.T) {
	cfg := filepath.Join(writeTree(t, map[string]string{"c.yml": "version: 1\npresets: [getter]\n"}), "c.yml")
	root := writeTree(t, map[string]string{
		"a.go": getterGo,
		// The same method, no longer a getter: the pairing reports it.
		"b.go": "package x\n\nfunc (u *U) Name() string { log(); return u.name }\n",
		"c.go": getterGo,
	})
	status, stdout, stderr := runDump("-config", cfg, "-root", root, "-pairs")
	if status != 0 || stderr != "" {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	var got struct {
		Files    map[string]tebanare.ChangeResult `json:"files"`
		Previous map[string]string                `json:"previous"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("%v:\n%s", err, stdout)
	}
	if want := map[string]string{"b.go": "a.go", "c.go": "b.go"}; !reflect.DeepEqual(got.Previous, want) {
		t.Errorf("previous = %v, want %v", got.Previous, want)
	}
	if a := got.Files["a.go"]; len(a.Old) != 0 || len(a.New) != 1 {
		t.Errorf("a.go, the first file, is not analyzed as added: %+v", a)
	}
	for _, name := range []string{"b.go", "c.go"} {
		r := got.Files[name]
		if len(r.Old) != 0 || len(r.New) != 0 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "match-changed" {
			t.Errorf("%s: %+v, want only a match-changed diagnostic", name, r)
		}
	}
	if !strings.HasSuffix(stdout, "\n\"previous\":{\"b.go\":\"a.go\",\"c.go\":\"b.go\"}}\n") {
		t.Errorf("unexpected layout:\n%s", stdout)
	}
}
