package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/fuzzvec"
)

func TestWrite(t *testing.T) {
	cache := t.TempDir()
	corpus := map[string]string{
		"internal/analyzer/FuzzAnalyze/a": "go test fuzz v1\n[]byte(\"package p\\n\")\n",
		"internal/config/FuzzCompile/a":   "go test fuzz v1\nstring(\"version: 1\\n\")\n",
		"internal/config/FuzzCompile/b":   "go test fuzz v1\nstring(\"version: 1\\npresets: [\")\n",
	}
	for name, content := range corpus {
		p := filepath.Join(cache, filepath.FromSlash(module+"/"+name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "vectors")
	var stderr bytes.Buffer
	if status := run([]string{"-cache", cache, "-o", out}, &stderr); status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stderr.String(), "config.json: 2 inputs") {
		t.Errorf("stderr = %q", stderr.String())
	}

	read := func(name string) fuzzvec.File {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(out, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var f fuzzvec.File
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	if f := read("analyze"); len(f.Inputs) != 1 || *f.Inputs[0].Text != "package p\n" {
		t.Errorf("analyze.json = %+v", f)
	}
	if f := read("config"); len(f.Inputs) != 2 || f.Inputs[0].Syntax || !f.Inputs[1].Syntax {
		t.Errorf("config.json = %+v, want the second input marked as a syntax error", f)
	}
	if f := read("sigpattern"); f.Inputs == nil || len(f.Inputs) != 0 {
		t.Errorf("sigpattern.json = %+v, want no inputs", f)
	}
}

func TestRunErrors(t *testing.T) {
	bad := t.TempDir()
	dir := filepath.Join(bad, filepath.FromSlash(module+"/internal/analyzer/FuzzAnalyze"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		args   []string
		status int
		stderr string
	}{
		{[]string{"-cache", bad}, 2, "usage: fuzzcorpus"},
		{[]string{"-o", bad}, 2, "usage: fuzzcorpus"},
		{[]string{"-cache", bad, "-o", t.TempDir(), "extra"}, 2, "usage: fuzzcorpus"},
		{[]string{"-cache", bad, "-o", t.TempDir()}, 1, "the first line is not"},
		{[]string{"-h"}, 0, "Usage of fuzzcorpus"},
	} {
		var stderr bytes.Buffer
		status := run(tt.args, &stderr)
		if status != tt.status || !strings.Contains(stderr.String(), tt.stderr) {
			t.Errorf("%q: status %d, stderr %q; want %d and %q", tt.args, status, stderr.String(), tt.status, tt.stderr)
		}
	}
}
