// Command fuzzcorpus writes the fuzz corpus that `go test -fuzz` collected
// in the build cache as JSON test vectors (see internal/fuzzvec), so the
// engine tests can replay the inputs through the TinyGo wasm build.
//
// Usage:
//
//	go run ./internal/tools/fuzzcorpus -cache "$(go env GOCACHE)/fuzz" -o <dir>
//
// It writes <dir>/analyze.json from FuzzAnalyze, <dir>/config.json from
// FuzzCompile, and <dir>/sigpattern.json from the FuzzParse of
// internal/sigpattern. A target without a cached corpus gives a file
// without inputs.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/miyamo2/go-tebanare/internal/config"
	"github.com/miyamo2/go-tebanare/internal/fuzzvec"
	"github.com/miyamo2/go-tebanare/internal/result"
)

const module = "github.com/miyamo2/go-tebanare"

// targets maps each output name to the package and fuzz test whose
// corpus it holds, and to the conversion of one corpus value.
var targets = []struct {
	name, pkg, fuzz string
	input           func([]byte) fuzzvec.Input
}{
	{"analyze", "internal/analyzer", "FuzzAnalyze", fuzzvec.NewInput},
	{"config", "internal/config", "FuzzCompile", configInput},
	{"sigpattern", "internal/sigpattern", "FuzzParse", func(b []byte) fuzzvec.Input {
		return fuzzvec.NewInput([]byte(fuzzvec.PatternConfig(string(b))))
	}},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run runs the command line args, without the program name, and returns
// the exit status: 0 on success, 1 on errors, 2 for invalid arguments.
func run(args []string, stderr io.Writer) int {
	f := flag.NewFlagSet("fuzzcorpus", flag.ContinueOnError)
	f.SetOutput(stderr)
	cache := f.String("cache", "", "read the corpus under `dir`, the fuzz directory of GOCACHE (required)")
	out := f.String("o", "", "write the vector files to `dir` (required)")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *cache == "" || *out == "" || f.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, "usage: fuzzcorpus -cache <dir> -o <dir>")
		return 2
	}
	if err := write(*cache, *out, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "fuzzcorpus: %v\n", err)
		return 1
	}
	return 0
}

func write(cache, out string, stderr io.Writer) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, t := range targets {
		dir := filepath.Join(cache, filepath.FromSlash(module+"/"+t.pkg), t.fuzz)
		values, err := fuzzvec.ReadCorpus(dir)
		if err != nil {
			return err
		}
		f := fuzzvec.File{Inputs: []fuzzvec.Input{}}
		for _, v := range values {
			f.Inputs = append(f.Inputs, t.input(v))
		}
		b, err := f.Marshal()
		if err != nil {
			return err
		}
		path := filepath.Join(out, t.name+".json")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stderr, "%s: %d inputs from %s\n", path, len(f.Inputs), dir)
	}
	return nil
}

// configInput returns the Input for a config and marks it when the native
// build rejects it with a YAML syntax error.
func configInput(src []byte) fuzzvec.Input {
	in := fuzzvec.NewInput(src)
	_, _, err := config.Compile(src)
	var ce *config.Error
	if errors.As(err, &ce) {
		for _, d := range ce.Diagnostics {
			in.Syntax = in.Syntax || d.Code == result.CodeConfigSyntax
		}
	}
	return in
}
