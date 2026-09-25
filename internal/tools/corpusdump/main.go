// Command corpusdump analyzes every non-test Go file under a directory as
// an added file and writes the results as JSON. The parity test of the
// TypeScript engine package compares them with the results of the wasm
// engine.
//
// Usage:
//
//	go run ./internal/tools/corpusdump -config <yml> -root <dir> [-limit N] [-pairs] > out.json
//
// With -pairs, corpusdump analyzes each file as a change whose old side is
// the file before it in path order, so that declarations with the same key
// pair across the two sides. Both sides use the path of the new file. The
// output then also holds "previous": {"<path>": "<path of the old side>"}
// for every file but the first.
//
// Root must be a directory or a symbolic link to one. corpusdump skips
// directories named testdata or vendor, directories whose names start with
// "." or "_", files named *_test.go, and symbolic links below root. Each
// file is analyzed with its slash-separated path relative to root. The
// output is {"files": {"<path>": ChangeResult}} with one file per line,
// sorted by path. A root without Go files to analyze is an error.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tebanare "github.com/miyamo2/go-tebanare"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs the command line args, without the program name, and returns
// the exit status: 0 on success, 1 on errors, 2 for invalid arguments.
func run(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("corpusdump", flag.ContinueOnError)
	f.SetOutput(stderr)
	configPath := f.String("config", "", "read the configuration from `file` (required)")
	root := f.String("root", "", "analyze the Go files under `dir` (required)")
	limit := f.Int("limit", 0, "analyze only the first `N` files in path order; 0 means all")
	pairs := f.Bool("pairs", false, "analyze each file as a change from the file before it in path order")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *configPath == "" || *root == "" || f.NArg() > 0 || *limit < 0 {
		_, _ = fmt.Fprintln(stderr, "usage: corpusdump -config <yml> -root <dir> [-limit N] [-pairs]")
		return 2
	}
	if err := dump(stdout, stderr, *configPath, *root, *limit, *pairs); err != nil {
		_, _ = fmt.Fprintf(stderr, "corpusdump: %v\n", err)
		return 1
	}
	return 0
}

func dump(stdout, stderr io.Writer, configPath, root string, limit int, pairs bool) error {
	src, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	rs, warnings, err := tebanare.Compile(src)
	var diags []tebanare.Diagnostic
	var ce *tebanare.ConfigError
	if errors.As(err, &ce) {
		diags = append(diags, ce.Diagnostics...)
	}
	for _, d := range append(diags, warnings...) {
		_, _ = fmt.Fprintln(stderr, d.Format(configPath))
	}
	if err != nil {
		return fmt.Errorf("%s is invalid", configPath)
	}

	fi, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}
	// filepath.WalkDir does not follow a symbolic link at the root.
	dir, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	files, err := goFiles(dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no Go files under %s", root)
	}
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	w := bufio.NewWriter(stdout)
	if _, err := w.WriteString(`{"files":{`); err != nil {
		return err
	}
	var prev []byte
	for i, rel := range files {
		src, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		key, err := encode(rel)
		if err != nil {
			return err
		}
		ch := tebanare.FileChange{NewPath: rel, New: src}
		if pairs && i > 0 {
			ch.OldPath, ch.Old = rel, prev
		}
		prev = src
		value, err := encode(analyze(rs, ch))
		if err != nil {
			return err
		}
		sep := ",\n"
		if i == 0 {
			sep = "\n"
		}
		if _, err := fmt.Fprintf(w, "%s%s:%s", sep, key, value); err != nil {
			return err
		}
	}
	if _, err := w.WriteString("\n}"); err != nil {
		return err
	}
	if pairs && len(files) > 1 {
		previous := map[string]string{}
		for i := 1; i < len(files); i++ {
			previous[files[i]] = files[i-1]
		}
		b, err := encode(previous)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, ",\n\"previous\":%s", b); err != nil {
			return err
		}
	}
	if _, err := w.WriteString("}\n"); err != nil {
		return err
	}
	return w.Flush()
}

// goFiles returns the slash-separated paths, relative to root, of the Go
// files to analyze, sorted.
func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "testdata" || name == "vendor" ||
				strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}

// analyze analyzes one change. A panic is a bug in the engine: analyze
// panics again with the path in the message.
func analyze(rs *tebanare.Ruleset, ch tebanare.FileChange) tebanare.ChangeResult {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("corpusdump: %s: %v", ch.NewPath, r))
		}
	}()
	return rs.AnalyzeChange(ch)
}

// encode returns the JSON encoding of v without HTML escaping.
func encode(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}
