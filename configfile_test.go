package tebanare_test

import (
	"reflect"
	"testing"

	tebanare "github.com/miyamo2/go-tebanare"
)

func TestSelectConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		want     string
		wantDiag []string
	}{
		{name: "none"},
		{name: "yml", files: []string{".gotebanare.yml"}, want: ".gotebanare.yml"},
		{name: "yaml", files: []string{".gotebanare.yaml"}, want: ".gotebanare.yaml"},
		{
			name:     "both",
			files:    []string{".gotebanare.yaml", ".gotebanare.yml"},
			want:     ".gotebanare.yml",
			wantDiag: []string{"warning config-ignored: .gotebanare.yaml is ignored because .gotebanare.yml exists"},
		},
		{name: "other names", files: []string{"gotebanare.yml", ".gotebanare.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var asked []string
			exists := func(name string) bool {
				asked = append(asked, name)
				for _, f := range tt.files {
					if f == name {
						return true
					}
				}
				return false
			}
			got, diags := tebanare.SelectConfigFile(exists)
			if got != tt.want {
				t.Errorf("name = %q, want %q", got, tt.want)
			}
			var gotDiag []string
			for _, d := range diags {
				gotDiag = append(gotDiag, string(d.Severity)+" "+d.Code+": "+d.Message)
			}
			if !reflect.DeepEqual(gotDiag, tt.wantDiag) {
				t.Errorf("diagnostics = %q, want %q", gotDiag, tt.wantDiag)
			}
			if !reflect.DeepEqual(asked, tebanare.ConfigFileNames) {
				t.Errorf("exists was called with %q, want %q", asked, tebanare.ConfigFileNames)
			}
		})
	}

	if got, diags := tebanare.SelectConfigFile(nil); got != "" || diags != nil {
		t.Errorf("SelectConfigFile(nil) = %q, %v; want \"\", nil", got, diags)
	}
}
