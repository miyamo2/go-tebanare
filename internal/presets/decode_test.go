package presets

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// decodeYAML parses src and decodes the document as settings of p.
func decodeYAML(t *testing.T, p *Preset, src string) (any, error) {
	t.Helper()
	return Decode(p, parseNode(t, src))
}

func ptr[T any](v T) *T { return &v }

func TestDecode(t *testing.T) {
	defaults := testPreset.NewSettings()
	tests := []struct {
		name string
		src  string
		want any
	}{
		{"empty document", "", defaults},
		{"null", "~", defaults},
		{"empty mapping", "{}", defaults},
		{"every setting", `
paths: ["a/**"]
exclude_paths: ["b/**"]
include_doc: false
level: 2
mode: slow
tags: [x, "y"]
quiet: true
`, &testSettings{
			FuncCommon: FuncCommon{Paths: []string{"a/**"}, ExcludePaths: []string{"b/**"}, IncludeDoc: ptr(false)},
			Level:      ptr(2), Mode: "slow", Tags: []string{"x", "y"}, Quiet: true,
		}},
		{"null values keep defaults", "level: 3\nmode: ~\ntags:\ninclude_doc: null", &testSettings{
			FuncCommon: newFuncCommon(), Level: ptr(3), Mode: "fast", Tags: []string{"a", "*b"},
		}},
		{"empty list", "tags: []", &testSettings{
			FuncCommon: newFuncCommon(), Mode: "fast", Tags: []string{},
		}},
		{"aliases", "tags: [&x p, *x]\nmode: !!str slow\nlevel: !!int 0x10", &testSettings{
			FuncCommon: newFuncCommon(), Level: ptr(16), Mode: "slow", Tags: []string{"p", "p"},
		}},
		{"standard tags", "!!map {tags: !!seq [x], level: 2147483647}", &testSettings{
			FuncCommon: newFuncCommon(), Level: ptr(2147483647), Mode: "fast", Tags: []string{"x"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeYAML(t, testPreset, tt.src)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decode =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}

	got, err := Decode(testPreset, nil)
	if err != nil || !reflect.DeepEqual(got, defaults) {
		t.Errorf("Decode(nil) = %+v, %v", got, err)
	}
	if got.(*testSettings).IncludeDoc == defaults.(*testSettings).IncludeDoc {
		t.Error("Decode shares the include_doc pointer between calls")
	}
}

func TestDecodeAliasedMapping(t *testing.T) {
	doc := parseNode(t, "base: &b {mode: slow}\nsettings: *b\n")
	got, err := Decode(testPreset, doc.Content[0].Content[3])
	if err != nil || got.(*testSettings).Mode != "slow" {
		t.Errorf("Decode(alias) = %+v, %v", got, err)
	}
}

func TestDecodeErrors(t *testing.T) {
	available := "(available settings: paths, exclude_paths, include_doc, level, mode, tags, quiet)"
	tests := []struct {
		src  string
		want []string
	}{
		{"[a, b]", []string{"1:1: settings must be a mapping, found a list"}},
		{"slow", []string{`1:1: settings must be a mapping, found string "slow"`}},
		{"\n\n  slow", []string{`3:3: settings must be a mapping, found string "slow"`}},
		{"levl: 1", []string{`1:1: levl: unknown setting "levl" ` + available}},
		{"1: x", []string{"1:1: setting names must be strings, found integer 1"}},
		{"? [a]\n: b", []string{"1:3: setting names must be strings, found a list"}},
		{"level: 1\nlevel: 2", []string{`2:1: level: setting "level" is already set on line 1`}},
		{"level: x", []string{`1:8: level: expected an integer, found string "x"`}},
		{"level: 1.5", []string{"1:8: level: expected an integer, found float 1.5"}},
		{"level: 99999999999999999999", []string{"1:8: level: expected an integer, found float 99999999999999999999"}},
		{"level: 9999999999999999999", []string{
			"1:8: level: expected an integer from -2147483648 to 2147483647, found integer 9999999999999999999"}},
		{"level: 2147483648", []string{
			"1:8: level: expected an integer from -2147483648 to 2147483647, found integer 2147483648"}},
		{"level: -0x80000001", []string{
			"1:8: level: expected an integer from -2147483648 to 2147483647, found integer -0x80000001"}},
		{"level: !!int abc", []string{`1:8: level: expected an integer, found !!int "abc"`}},
		{"level: 0", []string{"1:8: level: must be at least 1"}},
		{"quiet: yes", []string{`1:8: quiet: expected a boolean, found string "yes"`}},
		{"quiet: [true]", []string{"1:8: quiet: expected a boolean, found a list"}},
		{"mode: fastest", []string{`1:7: mode: unknown value "fastest" (valid values: fast, slow)`}},
		{"mode: 1", []string{"1:7: mode: expected a string, found integer 1"}},
		{"mode: !foo bar", []string{`1:7: mode: expected a string, found !foo "bar"`}},
		{"level: " + strings.Repeat("x", 50), []string{
			`1:8: level: expected an integer, found string "` + strings.Repeat("x", 40) + `..."`}},
		{"mode: '" + strings.Repeat("x", 50) + "'", []string{
			`1:7: mode: unknown value "` + strings.Repeat("x", 50) + `" (valid values: fast, slow)`}},
		{"tags: a", []string{`1:7: tags: expected a list of strings, found string "a"`}},
		{"tags: !!str [a]", []string{"1:7: tags: expected a list of strings, found a list tagged !!str"}},
		{"tags: !foo [a]", []string{"1:7: tags: expected a list of strings, found a list tagged !foo"}},
		{"!!str {mode: slow}", []string{"1:1: settings must be a mapping, found a mapping tagged !!str"}},
		{"tags: [a, 1, {b: c}]", []string{
			"1:11: tags[1]: expected a string, found integer 1",
			"1:14: tags[2]: expected a string, found a mapping",
		}},
		{"tags: [ok, bad]", []string{"1:12: tags[1]: bad tag"}},
		{"paths: ['a/[']", []string{`1:9: paths[0]: invalid glob "a/["`}},
		// The string items of a list with other items are still checked,
		// with their indexes in the YAML list.
		{"tags: [bad, 1, ok, bad]", []string{
			"1:8: tags[0]: bad tag",
			"1:13: tags[1]: expected a string, found integer 1",
			"1:20: tags[3]: bad tag",
		}},
		{"paths: ['[', 1]", []string{
			`1:9: paths[0]: invalid glob "["`,
			"1:14: paths[1]: expected a string, found integer 1",
		}},
		{"levl: 1\nquiet: yes\nlevel: 0\ntags: [bad]", []string{
			`1:1: levl: unknown setting "levl" ` + available,
			`2:8: quiet: expected a boolean, found string "yes"`,
			"3:8: level: must be at least 1",
			"4:8: tags[0]: bad tag",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			got, err := decodeYAML(t, testPreset, tt.src)
			if err == nil {
				t.Fatalf("Decode = %+v, want errors %q", got, tt.want)
			}
			if got != nil {
				t.Errorf("Decode returned settings %+v with an error", got)
			}
			var msgs []string
			for _, e := range unwrapAll(err) {
				var de *DecodeError
				if !errors.As(e, &de) {
					t.Fatalf("error %v is not a *DecodeError", e)
				}
				msgs = append(msgs, de.Error())
			}
			if !slices.Equal(msgs, tt.want) {
				t.Errorf("errors =\n%q\nwant\n%q", msgs, tt.want)
			}
		})
	}
}
