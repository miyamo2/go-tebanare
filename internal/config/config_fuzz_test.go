package config

import (
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/fuzzvec"
	"github.com/miyamo2/go-tebanare/internal/result"
)

var update = flag.Bool("update", false, "rewrite testvectors/fuzz/config.json")

// fuzzCompileSeeds returns the seed inputs of FuzzCompile.
func fuzzCompileSeeds(tb testing.TB) []string {
	return []string{
		string(readFile(tb, "testdata/minimal.yml")),
		"version: 1\nfiles: {exclude: [vendor/**]}\npresets: [getter: {max_depth: 2}, noop: {include_functions: true}, iferr: {names: [a, '*Err'], init: fold-body}]",
		"a: &a [*a, *a]\nversion: *a\npresets: *a",
		"version: 1\npresets: [iferr: {names: &n [&x a, *x]}, noop: {paths: *n}]\n---\n",
		"version: 1\npresets: [",
		"version: 1\n\tpresets: []\n",
		"version: 1\npresets: [" + strings.Repeat("[", 1001) + strings.Repeat("]", 1001) + "]",
	}
}

// TestFuzzVectors keeps testvectors/fuzz/config.json in step with the
// seeds and the corpus of FuzzCompile. The engine tests replay it through
// the wasm build, where a YAML syntax error stops the module.
func TestFuzzVectors(t *testing.T) {
	corpus, err := fuzzvec.ReadCorpus(filepath.Join("testdata", "fuzz", "FuzzCompile"))
	if err != nil {
		t.Fatal(err)
	}
	var inputs [][]byte
	for _, s := range fuzzCompileSeeds(t) {
		inputs = append(inputs, []byte(s))
	}
	var f fuzzvec.File
	for _, b := range append(inputs, corpus...) {
		in := fuzzvec.NewInput(b)
		in.Syntax = hasSyntaxError(b)
		f.Inputs = append(f.Inputs, in)
	}
	fuzzvec.Check(t, "config", f, *update)
}

// hasSyntaxError reports whether Compile rejects src with a YAML syntax
// error.
func hasSyntaxError(src []byte) bool {
	_, _, err := Compile(src)
	var ce *Error
	if !errors.As(err, &ce) {
		return false
	}
	for _, d := range ce.Diagnostics {
		if d.Code == result.CodeConfigSyntax {
			return true
		}
	}
	return false
}

func FuzzCompile(f *testing.F) {
	for _, s := range fuzzCompileSeeds(f) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		set, _, err := Compile([]byte(src))
		if (set == nil) == (err == nil) {
			t.Errorf("Compile(%q) = %v, %v", src, set, err)
		}
	})
}
