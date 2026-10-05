package presets

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// TestSchemaPresets checks that the presets in the schema and the
// registered presets are the same.
func TestSchemaPresets(t *testing.T) {
	if got, want := Names(), slices.Sorted(maps.Keys(schemaPresets)); !slices.Equal(got, want) {
		t.Errorf("registered presets %q, presets in the schema %q", got, want)
	}
	for _, p := range All() {
		if p.NewSettings == nil {
			t.Errorf("%s: NewSettings is nil", p.Name)
		}
	}
}

// TestCommonSettings checks that the settings that all presets of a kind
// share come first, in the same order.
func TestCommonSettings(t *testing.T) {
	common := map[Kind][]string{
		FuncKind: {"paths", "exclude_paths", "include_doc"},
		StmtKind: {"paths", "exclude_paths"},
	}
	for _, p := range All() {
		want := common[p.Kind]
		infos := Settings(p)
		var got []string
		for i := 0; i < len(want) && i < len(infos); i++ {
			got = append(got, infos[i].Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: first settings %q, want %q", p.Name, got, want)
		}
		for i := len(want); i < len(infos); i++ {
			if slices.Contains(common[FuncKind], infos[i].Name) {
				t.Errorf("%s: common setting %s is not first", p.Name, infos[i].Name)
			}
		}
	}
}

// TestDefaults checks that the default of every setting describes the
// value that NewSettings returns. A nil value is described by a word.
func TestDefaults(t *testing.T) {
	for _, p := range All() {
		v := reflect.ValueOf(p.NewSettings()).Elem()
		infos := Settings(p)
		if v.NumField() != len(infos) {
			t.Fatalf("%s: %d fields, %d settings", p.Name, v.NumField(), len(infos))
		}
		for i, info := range infos {
			f := v.Type().Field(i)
			if f.Tag.Get("yaml") != info.Name {
				t.Errorf("%s: field %s has yaml tag %q, want %q", p.Name, f.Name, f.Tag.Get("yaml"), info.Name)
			}
			if info.Description == "" {
				t.Errorf("%s.%s: no description", p.Name, info.Name)
			}
			if len(info.Enum) > 0 && !slices.Contains(info.Enum, info.Default) {
				t.Errorf("%s.%s: default %q is not in enum %q", p.Name, info.Name, info.Default, info.Enum)
			}
			fv := v.Field(i)
			if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Slice) && fv.IsNil() {
				if info.Default != "none" && info.Default != "unlimited" {
					t.Errorf("%s.%s: value is nil, default is %q", p.Name, info.Name, info.Default)
				}
				continue
			}
			want := reflect.New(f.Type)
			if err := yaml.Unmarshal([]byte(info.Default), want.Interface()); err != nil {
				t.Errorf("%s.%s: default %q: %v", p.Name, info.Name, info.Default, err)
				continue
			}
			if !reflect.DeepEqual(want.Elem().Interface(), fv.Interface()) {
				t.Errorf("%s.%s: default %q, NewSettings has %v", p.Name, info.Name, info.Default, fv)
			}
		}
	}
}

func TestDecode(t *testing.T) {
	p, _ := Lookup("iferr")
	s, err := Decode(p, map[string]any{"names": []any{"e", "*Err"}, "init": nil, "allow_comments": true})
	if err != nil {
		t.Fatal(err)
	}
	want := &IferrSettings{Names: []string{"e", "*Err"}, Init: InitExclude, AllowComments: true}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("Decode = %+v, want %+v", s, want)
	}
	if s, err := Decode(p, nil); err != nil || !reflect.DeepEqual(s, p.NewSettings()) {
		t.Errorf("Decode(nil) = %+v, %v; want the defaults", s, err)
	}
	g, _ := Lookup("getter")
	for _, v := range []any{int64(3), 3, float64(3)} {
		s, err := Decode(g, map[string]any{"max_depth": v})
		if err != nil || *s.(*GetterSettings).MaxDepth != 3 {
			t.Errorf("Decode(max_depth %T) = %+v, %v", v, s, err)
		}
	}
	if _, err := Decode(p, []any{"x"}); err == nil {
		t.Error("Decode(a list) succeeded")
	}
	if _, err := Decode(&Preset{Name: "nope"}, nil); err == nil {
		t.Error("Decode of a preset without settings in the schema succeeded")
	}
}

func TestRules(t *testing.T) {
	paths, exclude := []string{"a/**"}, []string{"b/**"}
	r := funcRule("p", "Summary.", paths, exclude, false, nil)
	if r.ID != "p" || r.Preset != "p" || r.Description != "Summary." || r.Target != result.TargetFunc ||
		r.IncludeDoc || !slices.Equal(r.Paths, paths) || !slices.Equal(r.ExcludePaths, exclude) {
		t.Errorf("funcRule = %+v", r)
	}
	if r := stmtRule("q", "S.", paths, nil, nil); r.ID != "q" || r.Target != result.TargetStmt || !slices.Equal(r.Paths, paths) {
		t.Errorf("stmtRule = %+v", r)
	}
}

func TestDescriptionsPairQuotes(t *testing.T) {
	for _, p := range All() {
		for _, s := range Settings(p) {
			if strings.Count(s.Description, "`")%2 != 0 {
				t.Errorf("%s.%s: unpaired code span in %q", p.Name, s.Name, s.Description)
			}
		}
	}
}
