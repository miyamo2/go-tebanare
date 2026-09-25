package bridge

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

const config = "version: 1\npresets: [getter]\n"

const getter = "package p\n\ntype U struct{ name string }\n\nfunc (u *U) Name() string { return u.name }\n"

func decode[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("invalid JSON %s: %v", b, err)
	}
	return v
}

func compile(t *testing.T, b *Bridge, yaml string) uint32 {
	t.Helper()
	out := decode[compileJSON](t, b.Compile([]byte(yaml)))
	if out.Error != "" || out.Handle == 0 {
		t.Fatalf("Compile(%q) = %+v", yaml, out)
	}
	return out.Handle
}

func TestInfo(t *testing.T) {
	got := decode[infoJSON](t, New().Info())
	if got.APIVersion != tebanare.APIVersion || got.EngineVersion != tebanare.EngineVersion ||
		!slices.Equal(got.ConfigFileNames, tebanare.ConfigFileNames) {
		t.Errorf("Info() = %+v", got)
	}
}

func TestYAMLProgress(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want uint32
	}{
		{"valid", config, YAMLDone},
		{"two documents", "version: 1\n---\npresets: [getter]\n", YAMLDone},
		{"invalid but valid YAML", "version: 2\n", YAMLDone},
		// The parser reads to the end, then reports the missing "]".
		{"unclosed flow sequence", "version: 1\npresets: [", 21},
		// Line 5 (offsets 46 to 55) has the bad indentation. The parser
		// reads the whole line before it stops.
		{"bad indentation", "version: 1\npresets:\n  - getter: a\n    noop: b\n   bad: 1\n", 56},
		// The tab at offset 11 starts line 2. The parser reads one more
		// byte before it stops.
		{"tab", "version: 1\n\tpresets: []\n", 13},
	}
	if got := *New().YAMLProgress(); got != YAMLDone {
		t.Errorf("progress before Compile = %d, want YAMLDone", got)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New()
			b.Compile([]byte(tt.yaml))
			if got := *b.YAMLProgress(); got != tt.want {
				t.Errorf("progress after Compile(%q) = %d, want %d", tt.yaml, got, tt.want)
			}
		})
	}
}

func TestCompile(t *testing.T) {
	b := New()
	h1 := compile(t, b, config)
	h2 := compile(t, b, config)
	if h1 != 1 || h2 != 2 {
		t.Errorf("handles = %d, %d; want 1, 2", h1, h2)
	}
	raw := string(b.Compile([]byte(config)))
	for _, want := range []string{`"diagnostics":[]`, `"rules":[{"id":"getter"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("Compile JSON %s does not contain %s", raw, want)
		}
	}
}

func TestCompileError(t *testing.T) {
	out := decode[compileJSON](t, New().Compile([]byte("version: 2\n")))
	if out.Handle != 0 || out.Error == "" || len(out.Diagnostics) == 0 || out.Rules == nil {
		t.Fatalf("Compile(version 2) = %+v", out)
	}
	if d := out.Diagnostics[0]; d.Severity != "error" || d.Line != 1 {
		t.Errorf("first diagnostic = %+v", d)
	}
}

func TestAnalyzeChange(t *testing.T) {
	b := New()
	h := compile(t, b, config)
	tests := []struct {
		name     string
		meta     string
		old, new string
		wantOld  int
		wantNew  int
		skipped  string
	}{
		{"added", `{"newPath":"p.go","hasNew":true}`, "", getter, 0, 1, ""},
		{"deleted", `{"oldPath":"p.go","hasOld":true}`, getter, "", 1, 0, ""},
		{"both", `{"oldPath":"p.go","newPath":"p.go","hasOld":true,"hasNew":true}`, getter, getter, 1, 1, ""},
		{"empty new file", `{"newPath":"p.go","hasNew":true}`, "", "", 0, 0, "parse-error"},
		{"not a target", `{"newPath":"p.txt","hasNew":true}`, "", getter, 0, 0, "not-target"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := b.AnalyzeChange(h, []byte(tt.meta), []byte(tt.old), []byte(tt.new))
			got := decode[tebanare.ChangeResult](t, raw)
			if len(got.Old) != tt.wantOld || len(got.New) != tt.wantNew || string(got.Skipped) != tt.skipped {
				t.Errorf("AnalyzeChange = %s", raw)
			}
			if strings.Contains(string(raw), "null") {
				t.Errorf("AnalyzeChange JSON has null: %s", raw)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	b := New()
	h := compile(t, b, config)
	tests := []struct {
		name string
		out  []byte
		want string
	}{
		{"unknown handle", b.AnalyzeChange(99, []byte(`{}`), nil, nil), "unknown handle 99"},
		{"bad meta", b.AnalyzeChange(h, []byte(`{`), nil, nil), "bad meta"},
	}
	for _, tt := range tests {
		got := decode[errJSON](t, tt.out)
		if !strings.Contains(got.Error, tt.want) {
			t.Errorf("%s: error %q does not contain %q", tt.name, got.Error, tt.want)
		}
	}
}

func TestRelease(t *testing.T) {
	b := New()
	h := compile(t, b, config)
	b.Release(h)
	b.Release(12345)
	got := decode[errJSON](t, b.AnalyzeChange(h, []byte(`{}`), nil, nil))
	if !strings.Contains(got.Error, "unknown handle") {
		t.Errorf("AnalyzeChange after Release = %+v", got)
	}
}

func TestPresets(t *testing.T) {
	got := decode[presetsJSON](t, New().Presets())
	if len(got.Presets) != 3 || got.Presets[0].Name != "getter" {
		t.Errorf("Presets = %+v", got)
	}
}
