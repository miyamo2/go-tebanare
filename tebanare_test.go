package tebanare_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
	"github.com/miyamo2/go-tebanare/internal/presets"
)

const presetsConfig = `version: 1
presets:
  - noop
  - getter
`

const rulesConfig = `version: 1
presets:
  - noop
  - getter
rules:
  - id: stringer
    description: fmt.Stringer implementations
    func: "func (_) String() string"
  - id: off
    enabled: false
    func: "func (_) Off()"
  - id: debug-log
    expr:
      kind: CallExpr
      regex: '^log\.Debug\('
`

func mustCompile(t *testing.T, src string) *tebanare.Ruleset {
	t.Helper()
	rs, _, err := tebanare.Compile([]byte(src))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return rs
}

func TestCompile(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantErr   []string // "line:column field" of each error
		wantWarns []string // codes
	}{
		{name: "minimal", src: "version: 1\npresets: [getter]\n"},
		{
			name:    "unknown preset",
			src:     "version: 1\npresets: [nope]\n",
			wantErr: []string{"2:11 presets[0](nope)"},
		},
		{
			name:    "errors in two sections",
			src:     "version: 2\npresets: [nope]\n",
			wantErr: []string{"1:10 version", "2:11 presets[0](nope)"},
		},
		{
			name:      "unanchored regexp warns",
			src:       "version: 1\nrules:\n  - id: a\n    stmt:\n      regex: 'x'\n",
			wantWarns: []string{"unanchored-regexp"},
		},
		{
			name:      "errors and warnings together",
			src:       "version: 2\nrules:\n  - id: a\n    stmt:\n      regex: 'x'\n",
			wantErr:   []string{"1:10 version"},
			wantWarns: []string{"unanchored-regexp"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, warns, err := tebanare.Compile([]byte(tt.src))
			var codes []string
			for _, w := range warns {
				codes = append(codes, w.Code)
			}
			if !reflect.DeepEqual(codes, tt.wantWarns) {
				t.Errorf("warning codes = %q, want %q", codes, tt.wantWarns)
			}
			if tt.wantErr == nil {
				if err != nil || rs == nil {
					t.Fatalf("Compile = %v, %v; want a ruleset", rs, err)
				}
				return
			}
			if rs != nil {
				t.Errorf("Compile returned a ruleset with error %v", err)
			}
			var ce *tebanare.ConfigError
			if !errors.As(err, &ce) {
				t.Fatalf("error %v (%T) is not a *ConfigError", err, err)
			}
			var got []string
			for _, d := range ce.Diagnostics {
				if d.Severity != "error" || d.Code != "config-invalid" {
					t.Errorf("diagnostic %+v: want severity error and code config-invalid", d)
				}
				got = append(got, fmt.Sprintf("%d:%d %s", d.Line, d.Column, d.Field))
			}
			if !reflect.DeepEqual(got, tt.wantErr) {
				t.Errorf("errors =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.wantErr, "\n"))
			}
		})
	}
}

func TestRules(t *testing.T) {
	rs := mustCompile(t, rulesConfig)
	summary := func(name string) string {
		p, ok := presets.Lookup(name)
		if !ok {
			t.Fatalf("no preset %q", name)
		}
		return p.Summary
	}
	// The JSON object of each rule, decoded, so the comparison does not
	// depend on how encoding/json escapes the preset summaries. A missing
	// key checks omitempty.
	want := []map[string]string{
		{"id": "stringer", "description": "fmt.Stringer implementations", "target": "func"},
		{"id": "debug-log", "target": "expr"},
		{"id": "noop", "description": summary("noop"), "target": "func", "preset": "noop"},
		{"id": "getter", "description": summary("getter"), "target": "func", "preset": "getter"},
	}
	b, err := json.Marshal(rs.Rules())
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Rules() JSON = %s\nwant %q", b, want)
	}
}

func TestNilRuleset(t *testing.T) {
	var rs *tebanare.Ruleset
	if got := rs.Rules(); got == nil || len(got) != 0 {
		t.Errorf("Rules() = %#v, want an empty non-nil slice", got)
	}
	res := rs.AnalyzeChange(tebanare.FileChange{NewPath: "a.go", New: []byte("package a\n")})
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"old":[],"new":[],"diagnostics":[],"skipped":""}`; string(b) != want {
		t.Errorf("AnalyzeChange JSON = %s, want %s", b, want)
	}
	nodes, err := rs.Explain("a.go", []byte("package a\n\nvar x = 1 + 2\n"), 3)
	if err != nil || len(nodes) == 0 {
		t.Errorf("Explain = %v, %v; want nodes", nodes, err)
	}
}

func TestAnalyzeChange(t *testing.T) {
	rs := mustCompile(t, presetsConfig)
	src := []byte("package a\n\ntype T struct{ n int }\n\nfunc (t T) N() int {\n\treturn t.n\n}\n")
	tests := []struct {
		name string
		ch   tebanare.FileChange
		want string
	}{
		{
			name: "added",
			ch:   tebanare.FileChange{NewPath: "a.go", New: src},
			want: `{"old":[],"new":[{"start":5,"end":7,"hits":[{"ruleId":"getter","target":"func","node":"FuncDecl","label":"func (T) N","preset":"getter"}]}],"diagnostics":[],"skipped":""}`,
		},
		{
			name: "deleted",
			ch:   tebanare.FileChange{OldPath: "a.go", Old: src},
			want: `{"old":[{"start":5,"end":7,"hits":[{"ruleId":"getter","target":"func","node":"FuncDecl","label":"func (T) N","preset":"getter"}]}],"new":[],"diagnostics":[],"skipped":""}`,
		},
		{
			name: "empty file is present",
			ch:   tebanare.FileChange{OldPath: "a.go", Old: []byte{}, NewPath: "a.go", New: src},
			want: `{"old":[],"new":[],"diagnostics":[{"severity":"info","code":"skipped","message":"","side":"old","line":1,"column":1}],"skipped":"parse-error"}`,
		},
		{
			name: "not a go file",
			ch:   tebanare.FileChange{NewPath: "a.txt", New: src},
			want: `{"old":[],"new":[],"diagnostics":[],"skipped":"not-target"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := rs.AnalyzeChange(tt.ch)
			// The messages are not part of the contract.
			for i := range res.Diagnostics {
				res.Diagnostics[i].Message = ""
			}
			b, err := json.Marshal(res)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tt.want {
				t.Errorf("got  %s\nwant %s", b, tt.want)
			}
		})
	}
}

func TestExplain(t *testing.T) {
	rs := mustCompile(t, rulesConfig)
	src := []byte("package a\n\nimport \"log\"\n\nfunc f() {\n\tlog.Debug(\"x\")\n}\n")
	nodes, err := rs.Explain("a.go", src, 6)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(nodes)
	if err != nil {
		t.Fatal(err)
	}
	want := `[` +
		`{"kind":"ExprStmt","line":6,"endLine":6,"text":"log.Debug(\"x\")","inScope":true,"rules":[]},` +
		`{"kind":"CallExpr","line":6,"endLine":6,"text":"log.Debug(\"x\")","inScope":true,"rules":["debug-log"]},` +
		`{"kind":"SelectorExpr","line":6,"endLine":6,"text":"log.Debug","inScope":true,"rules":[]},` +
		`{"kind":"Ident","line":6,"endLine":6,"text":"log","inScope":true,"rules":[]},` +
		`{"kind":"Ident","line":6,"endLine":6,"text":"Debug","inScope":true,"rules":[]},` +
		`{"kind":"BasicLit","line":6,"endLine":6,"text":"\"x\"","inScope":true,"rules":[]}` +
		`]`
	if string(b) != want {
		t.Errorf("Explain JSON =\n%s\nwant\n%s", b, want)
	}

	if _, err := rs.Explain("vendor.txt", src, 6); err == nil || !strings.HasPrefix(err.Error(), "not-target") {
		t.Errorf("Explain on a non-Go file: err = %v, want not-target", err)
	}
	if _, err := rs.Explain("a.go", []byte("package"), 1); err == nil || !strings.HasPrefix(err.Error(), "parse-error") {
		t.Errorf("Explain on a broken file: err = %v, want parse-error", err)
	}
}

func TestVersions(t *testing.T) {
	if tebanare.APIVersion != 1 {
		t.Errorf("APIVersion = %d, want 1", tebanare.APIVersion)
	}
	if tebanare.EngineVersion == "" {
		t.Error("EngineVersion is empty")
	}
}
