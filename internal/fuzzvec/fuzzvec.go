// Package fuzzvec stores the inputs of fuzz tests as JSON test vectors.
// The tests of packages/engine replay them through the TinyGo wasm build,
// which cannot recover from a panic (plan 8): an input that panics only
// there would stop the engine.
//
// Tests in internal/analyzer, internal/config, and internal/sigpattern
// keep testvectors/fuzz/<name>.json in step with the seeds and the
// testdata/fuzz corpus of their fuzz tests. internal/tools/fuzzcorpus
// writes the corpus that `go test -fuzz` collects in GOCACHE in the same
// format.
package fuzzvec

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Input is one fuzz input: Text when it is valid UTF-8, Base64 otherwise.
type Input struct {
	Text   *string `json:"text,omitempty"`
	Base64 string  `json:"base64,omitempty"`
	// Syntax marks a config that the native build rejects with a YAML
	// syntax error. The wasm build stops with a trap on such a config.
	Syntax bool `json:"syntax,omitempty"`
}

// NewInput returns the Input that holds b.
func NewInput(b []byte) Input {
	if utf8.Valid(b) {
		s := string(b)
		return Input{Text: &s}
	}
	return Input{Base64: base64.StdEncoding.EncodeToString(b)}
}

// File is the content of one vector file.
type File struct {
	// Source is Go source that the replay analyzes with every config that
	// compiles. Empty means the replay's own sample.
	Source string  `json:"source,omitempty"`
	Inputs []Input `json:"inputs"`
}

// Marshal returns the indented JSON of f with a final newline.
func (f File) Marshal() ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// corpusHeader is the first line of a file in a Go fuzz corpus.
const corpusHeader = "go test fuzz v1"

// ReadCorpus returns the values in the Go fuzz corpus directory dir, such
// as testdata/fuzz/FuzzAnalyze, sorted by file name. Every file must hold
// one []byte or string value. A missing directory holds no values.
func ReadCorpus(dir string) ([][]byte, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out [][]byte
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		v, err := parseCorpusFile(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// parseCorpusFile returns the value of a corpus file with one []byte or
// string value, the format that `go test -fuzz` writes.
func parseCorpusFile(b []byte) ([]byte, error) {
	header, rest, _ := strings.Cut(string(b), "\n")
	if strings.TrimSpace(header) != corpusHeader {
		return nil, fmt.Errorf("the first line is not %q", corpusHeader)
	}
	rest = strings.TrimSpace(rest)
	for _, typ := range []string{"[]byte", "string"} {
		lit, ok := strings.CutPrefix(rest, typ+"(")
		if !ok {
			continue
		}
		lit, ok = strings.CutSuffix(lit, ")")
		if !ok {
			break
		}
		v, err := strconv.Unquote(lit)
		if err != nil {
			return nil, fmt.Errorf("value %s: %w", rest, err)
		}
		return []byte(v), nil
	}
	return nil, fmt.Errorf("want one []byte or string value, found %q", rest)
}

// PatternConfig returns a config with one func rule whose pattern is p.
// It writes p as a YAML double-quoted string and escapes every character
// that YAML does not allow in it, so the config always parses as YAML.
// Invalid UTF-8 in p becomes U+FFFD.
func PatternConfig(p string) string {
	var b strings.Builder
	b.WriteString("version: 1\nrules:\n  - id: p\n    func: \"")
	for _, r := range strings.ToValidUTF8(p, "�") {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case yamlPrintable(r):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "\\U%08X", r)
		}
	}
	b.WriteString("\"\n")
	return b.String()
}

// yamlPrintable reports whether r may appear unescaped in a YAML
// double-quoted string. yaml.v3 also rejects the byte order mark there.
func yamlPrintable(r rune) bool {
	switch {
	case r >= 0x20 && r <= 0x7E:
		return true
	case r == 0xFEFF:
		return false
	case r >= 0xA0 && r <= 0xD7FF, r >= 0xE000 && r <= 0xFFFD, r >= 0x10000 && r <= 0x10FFFF:
		return true
	}
	return false
}

// Check compares testvectors/fuzz/<name>.json at the module root with f.
// With update set, it writes the file instead.
func Check(t testing.TB, name string, f File, update bool) {
	t.Helper()
	got, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "testvectors", "fuzz", name+".json")
	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (rerun this test with -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s does not hold the current fuzz seeds and corpus; rerun this test with -update", path)
	}
}

// moduleRoot returns the nearest directory above the working directory
// that holds go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}
