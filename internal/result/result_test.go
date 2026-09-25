package result

import (
	"encoding/json"
	"testing"
)

func TestDiagnosticFormat(t *testing.T) {
	tests := []struct {
		d    Diagnostic
		file string
		want string
	}{
		{Diagnostic{Message: "m"}, "", "m"},
		{Diagnostic{Message: "m"}, "a.yml", "a.yml: m"},
		{Diagnostic{Message: "m", Line: 3}, "a.yml", "a.yml:3: m"},
		{Diagnostic{Message: "m", Line: 3, Column: 7}, "a.yml", "a.yml:3:7: m"},
		{Diagnostic{Message: "m", Line: 3, Column: 7, Field: "rules[0](x).func"}, "", "3:7: rules[0](x).func: m"},
		{Diagnostic{Message: "m", Column: 7}, "", "m"},
	}
	for _, tt := range tests {
		if got := tt.d.Format(tt.file); got != tt.want {
			t.Errorf("%+v.Format(%q) = %q, want %q", tt.d, tt.file, got, tt.want)
		}
	}
}

func TestChangeResultNormalizeJSON(t *testing.T) {
	r := ChangeResult{New: []Range{{Start: 1, End: 2}}}
	r.Normalize()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"old":[],"new":[{"start":1,"end":2,"hits":[]}],"diagnostics":[],"skipped":""}`
	if string(b) != want {
		t.Errorf("got %s\nwant %s", b, want)
	}
}
