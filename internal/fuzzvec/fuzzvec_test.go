package fuzzvec

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadCorpus(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"b": "go test fuzz v1\n[]byte(\"package p\\n\\xff\")\n",
		"a": "go test fuzz v1\nstring(\"func F()\")\n",
		"c": "go test fuzz v1\n[]byte(`raw`)",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadCorpus(dir)
	want := [][]byte{[]byte("func F()"), []byte("package p\n\xff"), []byte("raw")}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ReadCorpus = %q, %v; want %q", got, err, want)
	}

	if got, err := ReadCorpus(filepath.Join(dir, "none")); got != nil || err != nil {
		t.Errorf("missing directory: %q, %v", got, err)
	}

	for _, bad := range []string{
		"[]byte(\"x\")\n",
		"go test fuzz v1\nint(1)\n",
		"go test fuzz v1\n[]byte(\"x\")\nstring(\"y\")\n",
		"go test fuzz v1\nstring(\"x\n",
	} {
		sub := t.TempDir()
		if err := os.WriteFile(filepath.Join(sub, "f"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCorpus(sub); err == nil {
			t.Errorf("ReadCorpus accepted %q", bad)
		}
	}
}

func TestNewInput(t *testing.T) {
	if in := NewInput([]byte("ok")); in.Text == nil || *in.Text != "ok" || in.Base64 != "" {
		t.Errorf("NewInput(ok) = %+v", in)
	}
	if in := NewInput([]byte{0xff}); in.Text != nil || in.Base64 != "/w==" {
		t.Errorf("NewInput(0xff) = %+v", in)
	}
	b, err := File{Inputs: []Input{NewInput(nil), NewInput([]byte("<"))}}.Marshal()
	if want := "{\n  \"inputs\": [\n    {\n      \"text\": \"\"\n    },\n    {\n      \"text\": \"<\"\n    }\n  ]\n}\n"; err != nil || string(b) != want {
		t.Errorf("Marshal = %q, %v; want %q", b, err, want)
	}
}

func TestPatternConfig(t *testing.T) {
	got := PatternConfig("func F(\"a\\b\")\t\x00\ufeff\xff é")
	want := `version: 1
rules:
  - id: p
    func: "func F(\"a\\b\")\U00000009\U00000000\U0000FEFF` + "� é\"\n"
	if got != want {
		t.Errorf("PatternConfig = %q, want %q", got, want)
	}
	if !strings.HasPrefix(PatternConfig(""), "version: 1\n") {
		t.Error("PatternConfig(\"\") has no version")
	}
}
