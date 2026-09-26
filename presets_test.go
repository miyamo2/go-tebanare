package tebanare_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

func TestPresets(t *testing.T) {
	got := tebanare.Presets()
	var names, kinds []string
	for _, p := range got {
		names = append(names, p.Name)
		kinds = append(kinds, p.Kind)
		if p.Summary == "" || len(p.Criteria) == 0 || len(p.Settings) == 0 || len(p.Examples) == 0 {
			t.Errorf("preset %s has an empty summary, criteria, settings, or examples: %+v", p.Name, p)
		}
	}
	if want := []string{"getter", "iferr", "noop"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %q, want %q", names, want)
	}
	if want := []string{"func", "stmt", "func"}; !reflect.DeepEqual(kinds, want) {
		t.Errorf("kinds = %q, want %q", kinds, want)
	}
}

func TestPresetsJSON(t *testing.T) {
	b, err := json.Marshal(tebanare.Presets())
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]json.RawMessage
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	want := []string{"criteria", "examples", "kind", "name", "settings", "summary"}
	for _, p := range decoded {
		var keys []string
		for k := range p {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, want) {
			t.Errorf("keys = %q, want %q", keys, want)
		}
	}

	// The nested types keep their own lowerCamel keys.
	var getter struct {
		Settings []map[string]any `json:"settings"`
		Examples []map[string]any `json:"examples"`
	}
	if err := json.Unmarshal(decoded[0]["settings"], &getter.Settings); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(decoded[0]["examples"], &getter.Examples); err != nil {
		t.Fatal(err)
	}
	for key, obj := range map[string]map[string]any{
		"name": getter.Settings[0], "description": getter.Settings[0],
		"code": getter.Examples[0], "match": getter.Examples[0],
	} {
		if _, ok := obj[key]; !ok {
			t.Errorf("%v has no key %q", obj, key)
		}
	}
}

func TestPresetsReturnsCopies(t *testing.T) {
	first := tebanare.Presets()
	first[0].Criteria[0] = "changed"
	first[0].Examples[0].Code = "changed"
	first[0].Settings[0].Name = "changed"
	second := tebanare.Presets()
	if second[0].Criteria[0] == "changed" || second[0].Examples[0].Code == "changed" || second[0].Settings[0].Name == "changed" {
		t.Error("changing the result of Presets changed the built-in presets")
	}
}
