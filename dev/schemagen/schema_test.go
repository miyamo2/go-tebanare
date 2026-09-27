package schemagen

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	tebanare "github.com/miyamo2/go-tebanare"
)

var update = flag.Bool("update", false, "rewrite schema/gotebanare.schema.json")

// schemaFile is the published JSON Schema of the configuration.
var schemaFile = filepath.Join("..", "..", "schema", "gotebanare.schema.json")

// TestSchemaUpToDate checks that schema/gotebanare.schema.json is the
// output of JSONSchema. With -update it rewrites the file.
func TestSchemaUpToDate(t *testing.T) {
	want := JSONSchema()
	if *update {
		if err := os.WriteFile(schemaFile, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("%v (run make schema to create it)", err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date (run make schema to regenerate it)", schemaFile)
	}
}

func TestSettingConstraintsExist(t *testing.T) {
	for key := range settingConstraints {
		preset, name, _ := strings.Cut(key, ".")
		found := slices.ContainsFunc(tebanare.Presets(), func(p tebanare.PresetInfo) bool {
			return p.Name == preset && slices.ContainsFunc(p.Settings, func(s tebanare.SettingInfo) bool {
				return s.Name == name
			})
		})
		if !found {
			t.Errorf("settingConstraints[%q]: no such preset setting", key)
		}
	}
}

func TestDefaultValue(t *testing.T) {
	for _, tt := range []struct {
		s    tebanare.SettingInfo
		want any
	}{
		{tebanare.SettingInfo{Type: "bool", Default: "true"}, true},
		{tebanare.SettingInfo{Type: "int", Default: "unlimited"}, nil},
		{tebanare.SettingInfo{Type: "int", Default: "3"}, 3},
		{tebanare.SettingInfo{Type: "string", Default: "a", Enum: []string{"a", "b"}}, "a"},
		{tebanare.SettingInfo{Type: "string", Default: "c", Enum: []string{"a", "b"}}, nil},
		{tebanare.SettingInfo{Type: "[]string", Default: "none"}, nil},
		{tebanare.SettingInfo{Type: "[]string", Default: `[err, "*Err"]`}, []any{"err", "*Err"}},
		{tebanare.SettingInfo{Type: "[]string", Default: "[1]"}, nil},
	} {
		got := defaultValue(tt.s)
		if gj, wj := mustJSON(t, got), mustJSON(t, tt.want); gj != wj {
			t.Errorf("defaultValue(%+v) = %s, want %s", tt.s, gj, wj)
		}
	}
}

// TestSchemaAgreesWithCompile checks that the schema accepts every
// configuration that tebanare.Compile accepts, and rejects the invalid
// configurations it can describe. Some errors, such as a preset listed
// twice or an invalid glob, are beyond a JSON Schema.
func TestSchemaAgreesWithCompile(t *testing.T) {
	sch := compileSchema(t)
	for _, tt := range []struct {
		name string
		src  string
		// valid is the verdict of tebanare.Compile.
		valid bool
		// schemaValid is the verdict of the schema.
		schemaValid bool
	}{
		{"minimal", "version: 1\n", true, true},
		{"docs example", "version: 1\nfiles:\n  include: [\"**/*.go\"]\n  exclude: [\"vendor/**\"]\npresets:\n  - getter\n  - noop\n  - iferr:\n      names: [err, \"*Err\"]\n", true, true},
		{"null values", "version: 1\nfiles:\n  include:\n  exclude:\npresets:\n  - iferr:\n  - getter:\n      max_depth:\n", true, true},
		{"null sections", "version: 1\nfiles:\npresets:\n", true, true},
		{"empty settings", "version: 1\npresets:\n  - noop: {}\n", true, true},
		{"every setting", "version: 1\npresets:\n" +
			"  - getter: {paths: [\"a/**\"], exclude_paths: [\"b/**\"], include_doc: false, max_depth: 2}\n" +
			"  - noop: {allow_comments: false, include_functions: true}\n" +
			"  - iferr: {names: [err, \"?rr\", \"エラー\"], allow_comments: true, init: fold-body, allow_bare_return: true, allow_calls_in_results: true}\n", true, true},
		{"missing version", "presets: [getter]\n", false, false},
		{"version 2", "version: 2\n", false, false},
		{"version string", "version: \"1\"\n", false, false},
		{"unknown top-level key", "version: 1\npreset: [getter]\n", false, false},
		{"unknown files key", "version: 1\nfiles: {includes: [\"**\"]}\n", false, false},
		{"files include string", "version: 1\nfiles: {include: \"**/*.go\"}\n", false, false},
		{"unknown preset", "version: 1\npresets: [geter]\n", false, false},
		{"unknown preset key", "version: 1\npresets:\n  - geter: {}\n", false, false},
		{"two presets in one item", "version: 1\npresets:\n  - {getter: {}, noop: {}}\n", false, false},
		{"null preset item", "version: 1\npresets:\n  -\n", false, false},
		{"unknown setting", "version: 1\npresets:\n  - getter: {max_dept: 1}\n", false, false},
		{"setting of another preset", "version: 1\npresets:\n  - noop: {names: [err]}\n", false, false},
		{"settings list", "version: 1\npresets:\n  - noop: [true]\n", false, false},
		{"bool as string", "version: 1\npresets:\n  - noop: {include_functions: \"yes\"}\n", false, false},
		{"int as string", "version: 1\npresets:\n  - getter: {max_depth: \"1\"}\n", false, false},
		{"int out of range", "version: 1\npresets:\n  - getter: {max_depth: 2147483648}\n", false, false},
		{"max_depth 0", "version: 1\npresets:\n  - getter: {max_depth: 0}\n", false, false},
		{"unknown enum value", "version: 1\npresets:\n  - iferr: {init: fold}\n", false, false},
		{"empty names", "version: 1\npresets:\n  - iferr: {names: []}\n", false, false},
		{"invalid name", "version: 1\npresets:\n  - iferr: {names: [\"e.rr\"]}\n", false, false},
		{"empty name", "version: 1\npresets:\n  - iferr: {names: [\"\"]}\n", false, false},
		{"non-string name", "version: 1\npresets:\n  - iferr: {names: [1]}\n", false, false},
		// Beyond the schema.
		{"duplicate preset", "version: 1\npresets: [getter, getter]\n", false, true},
		{"invalid glob", "version: 1\nfiles: {exclude: [\"[\"]}\n", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := compiles(tt.src); got != tt.valid {
				t.Fatalf("tebanare.Compile valid = %v, want %v", got, tt.valid)
			}
			if got := validates(t, sch, tt.src); got != tt.schemaValid {
				t.Errorf("schema valid = %v, want %v", got, tt.schemaValid)
			}
		})
	}
}

// TestSchemaAcceptsFixtures checks the schema against the configuration
// files of the repository and the settings of the preset examples. Each
// one that tebanare.Compile accepts must pass the schema.
func TestSchemaAcceptsFixtures(t *testing.T) {
	sch := compileSchema(t)
	root := filepath.Join("..", "..")
	var srcs []string
	err := filepath.WalkDir(filepath.Join(root, "testdata"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || (filepath.Ext(path) != ".yml" && filepath.Ext(path) != ".yaml") {
			return err
		}
		b, err := os.ReadFile(path)
		srcs = append(srcs, string(b))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range tebanare.Presets() {
		for _, ex := range p.Examples {
			if ex.Settings != "" {
				srcs = append(srcs, "version: 1\npresets:\n  - "+p.Name+":\n"+indentLines(ex.Settings, "      "))
			}
		}
	}
	checked := 0
	for _, src := range srcs {
		if !compiles(src) {
			continue
		}
		checked++
		if !validates(t, sch, src) {
			t.Errorf("schema rejects a valid configuration:\n%s", src)
		}
	}
	if checked < 10 {
		t.Errorf("checked %d configurations, want at least 10", checked)
	}
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(JSONSchema()))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(SchemaID, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(SchemaID)
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func compiles(src string) bool {
	_, _, err := tebanare.Compile([]byte(src))
	return err == nil
}

// validates reports whether the YAML document src passes sch.
func validates(t *testing.T, sch *jsonschema.Schema, src string) bool {
	t.Helper()
	var v any
	if err := yaml.Unmarshal([]byte(src), &v); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(mustJSON(t, v)))
	if err != nil {
		t.Fatal(err)
	}
	return sch.Validate(inst) == nil
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// indentLines prefixes each non-empty line of s with prefix.
func indentLines(s, prefix string) string {
	var b strings.Builder
	for line := range strings.Lines(s) {
		if strings.TrimSpace(line) != "" {
			b.WriteString(prefix)
		}
		b.WriteString(line)
	}
	if !strings.HasSuffix(s, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}
