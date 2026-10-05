package configschema

import (
	"reflect"
	"slices"
	"testing"
)

func TestPresetNames(t *testing.T) {
	names, err := PresetNames()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"getter", "iferr", "noop"}; !slices.Equal(names, want) {
		t.Errorf("PresetNames() = %q, want %q", names, want)
	}
}

func TestKeys(t *testing.T) {
	for _, tt := range []struct {
		path []string
		want []string
	}{
		{nil, []string{"version", "files", "presets"}},
		{[]string{"files"}, []string{"include", "exclude"}},
	} {
		got, err := Keys(tt.path...)
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("Keys(%q) = %q, %v; want %q", tt.path, got, err, tt.want)
		}
	}
	if _, err := Keys("version"); err == nil {
		t.Error(`Keys("version") succeeded`)
	}
}

func TestSettings(t *testing.T) {
	got, err := Settings("iferr")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}
	want := []string{"paths", "exclude_paths", "names", "allow_comments", "init", "allow_bare_return", "allow_calls_in_results"}
	if !slices.Equal(names, want) {
		t.Errorf("setting names = %q, want %q (schema order)", names, want)
	}
	init := got[4]
	wantInit := Setting{
		Name: "init", Type: "string", Default: "exclude", Enum: []string{"exclude", "fold-body"},
		Description: "How to treat if statements with an init statement: `exclude` keeps them visible; " +
			"`fold-body` hides the lines from `return` to the closing brace and keeps the header line visible.",
	}
	if !reflect.DeepEqual(init, wantInit) {
		t.Errorf("init = %+v, want %+v", init, wantInit)
	}
	if got[0].Type != "[]string" || got[0].Default != "none" || got[3].Type != "bool" {
		t.Errorf("settings = %+v", got)
	}
	if _, err := Settings("nope"); err == nil {
		t.Error(`Settings("nope") succeeded`)
	}
}

// TestSettingsOfEveryPreset checks that the schema describes the settings
// of every preset in the form that Settings reads.
func TestSettingsOfEveryPreset(t *testing.T) {
	names, _ := PresetNames()
	for _, name := range names {
		if s, err := Settings(name); err != nil || len(s) == 0 {
			t.Errorf("Settings(%q) = %v, %v", name, s, err)
		}
	}
}

func TestDecodeSettings(t *testing.T) {
	var s IferrSettings
	if err := DecodeSettings("iferr", map[string]any{"names": []any{"e"}, "allow_comments": nil}, &s); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Names, []string{"e"}) || s.Init == nil || *s.Init != Exclude ||
		s.AllowComments == nil || *s.AllowComments || s.Paths != nil {
		t.Errorf("DecodeSettings = %+v", s)
	}
	var g GetterSettings
	if err := DecodeSettings("getter", nil, &g); err != nil {
		t.Fatal(err)
	}
	if g.IncludeDoc == nil || !*g.IncludeDoc || g.MaxDepth != nil {
		t.Errorf("getter defaults = %+v", g)
	}
	if err := DecodeSettings("getter", []any{}, &g); err == nil {
		t.Error("DecodeSettings(a list) succeeded")
	}
	if err := DecodeSettings("nope", nil, &g); err == nil {
		t.Error(`DecodeSettings("nope") succeeded`)
	}
}

func TestDecodeConfig(t *testing.T) {
	c, err := DecodeConfig(map[string]any{
		"version": int64(1),
		"files":   map[string]any{"include": []any{"**/*.go"}},
		"presets": []any{"getter", map[string]any{"iferr": map[string]any{"names": []any{"e"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 1 || !slices.Equal(c.Files.Include, []string{"**/*.go"}) || len(c.Presets) != 2 ||
		c.Presets[0].Enum == nil || *c.Presets[0].Enum != Getter ||
		c.Presets[1].PresetClass == nil || !slices.Equal(c.Presets[1].PresetClass.Iferr.Names, []string{"e"}) {
		t.Errorf("DecodeConfig = %+v", c)
	}
}
