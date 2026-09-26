package presets

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// testSettings has one setting of each supported type.
type testSettings struct {
	FuncCommon `yaml:",inline"`
	Level      *int     `yaml:"level" default:"unlimited" doc:"A level."`
	Mode       string   `yaml:"mode" default:"fast" enum:"fast,slow" doc:"A mode."`
	Tags       []string `yaml:"tags" default:"[a, \"*b\"]" doc:"Some tags."`
	Quiet      bool     `yaml:"quiet" default:"false" doc:"Quiet or not."`
	Skipped    int      `yaml:"-"`
}

var testPreset = &Preset{
	Name: "test",
	Kind: FuncKind,
	NewSettings: func() any {
		return &testSettings{FuncCommon: newFuncCommon(), Mode: "fast", Tags: []string{"a", "*b"}}
	},
}

func TestSettings(t *testing.T) {
	want := []SettingInfo{
		{Name: "paths", Type: "[]string", Default: "none",
			Description: "Globs that limit the files this preset applies to, on top of `files.include` and `files.exclude`."},
		{Name: "exclude_paths", Type: "[]string", Default: "none",
			Description: "Globs of files this preset skips, on top of `files.exclude`."},
		{Name: "include_doc", Type: "bool", Default: "true",
			Description: "Hide the doc comment together with the function."},
		{Name: "level", Type: "int", Default: "unlimited", Description: "A level."},
		{Name: "mode", Type: "string", Default: "fast", Description: "A mode.", Enum: []string{"fast", "slow"}},
		{Name: "tags", Type: "[]string", Default: `[a, "*b"]`, Description: "Some tags."},
		{Name: "quiet", Type: "bool", Default: "false", Description: "Quiet or not."},
	}
	if got := Settings(testPreset); !reflect.DeepEqual(got, want) {
		t.Errorf("Settings() =\n%+v\nwant\n%+v", got, want)
	}
	stmt := settingInfos(reflect.TypeOf(StmtCommon{}))
	if len(stmt) != 2 || stmt[0].Name != "paths" || stmt[1].Name != "exclude_paths" {
		t.Errorf("StmtCommon settings = %+v", stmt)
	}
}

// TestDefaultTags checks that the default tag of every setting describes
// the value that NewSettings returns. A nil value is described by a word.
func TestDefaultTags(t *testing.T) {
	for _, p := range append(All(), testPreset) {
		checkDefaults(t, p.Name, reflect.ValueOf(p.NewSettings()).Elem())
	}
}

func checkDefaults(t *testing.T, preset string, v reflect.Value) {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if opts == "inline" {
			checkDefaults(t, preset, v.Field(i))
			continue
		}
		if name == "-" {
			continue
		}
		def, ok := f.Tag.Lookup("default")
		if !ok || f.Tag.Get("doc") == "" {
			t.Errorf("%s.%s: missing default or doc tag", preset, name)
			continue
		}
		if enum := f.Tag.Get("enum"); enum != "" && !slices.Contains(strings.Split(enum, ","), def) {
			t.Errorf("%s.%s: default %q is not in enum %q", preset, name, def, enum)
		}
		fv := v.Field(i)
		if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Slice) && fv.IsNil() {
			if def != "none" && def != "unlimited" {
				t.Errorf("%s.%s: value is nil, default tag is %q", preset, name, def)
			}
			continue
		}
		want := reflect.New(f.Type)
		if err := yaml.Unmarshal([]byte(def), want.Interface()); err != nil {
			t.Errorf("%s.%s: default tag %q: %v", preset, name, def, err)
			continue
		}
		if !reflect.DeepEqual(want.Elem().Interface(), fv.Interface()) {
			t.Errorf("%s.%s: default tag %q, NewSettings has %v", preset, name, def, fv)
		}
	}
}

func TestNewRule(t *testing.T) {
	no := false
	fc := FuncCommon{Paths: []string{"a/**"}, ExcludePaths: []string{"b/**"}, IncludeDoc: &no}
	r := fc.newRule("p", "Summary.", nil)
	if r.ID != "p" || r.Preset != "p" || r.Description != "Summary." || r.Target != result.TargetFunc ||
		r.IncludeDoc || !slices.Equal(r.Paths, fc.Paths) || !slices.Equal(r.ExcludePaths, fc.ExcludePaths) {
		t.Errorf("FuncCommon.newRule = %+v", r)
	}
	if r := (&FuncCommon{}).newRule("p", "", nil); !r.IncludeDoc {
		t.Error("nil include_doc must mean true")
	}
	sc := StmtCommon{Paths: []string{"a/**"}}
	if r := sc.newRule("q", "S.", nil); r.ID != "q" || r.Target != result.TargetStmt || !slices.Equal(r.Paths, sc.Paths) {
		t.Errorf("StmtCommon.newRule = %+v", r)
	}
}

func TestDocTagsPairQuotes(t *testing.T) {
	for _, p := range All() {
		for _, s := range Settings(p) {
			if strings.Count(s.Description, "`")%2 != 0 {
				t.Errorf("%s.%s: unpaired code span in %q", p.Name, s.Name, s.Description)
			}
		}
	}
}
