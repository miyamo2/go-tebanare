package presets

import (
	"slices"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

func TestRegister(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil

	for _, name := range []string{"noop", "getter", "iferr", "b"} {
		register(&Preset{Name: name})
	}
	want := []string{"b", "getter", "iferr", "noop"}
	if got := Names(); !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	all := All()
	for i, p := range all {
		if p.Name != want[i] {
			t.Errorf("All()[%d].Name = %q, want %q", i, p.Name, want[i])
		}
	}
	all[0] = nil
	if registry[0] == nil {
		t.Error("All() returned the registry itself")
	}
	if p, ok := Lookup("iferr"); !ok || p.Name != "iferr" {
		t.Errorf("Lookup(iferr) = %v, %v", p, ok)
	}
	if p, ok := Lookup("unknown"); ok || p != nil {
		t.Errorf("Lookup(unknown) = %v, %v, want nil, false", p, ok)
	}
}

func TestKind(t *testing.T) {
	tests := []struct {
		kind   Kind
		str    string
		target result.Target
	}{
		{FuncKind, "func", result.TargetFunc},
		{StmtKind, "stmt", result.TargetStmt},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.str {
			t.Errorf("String() = %q, want %q", got, tt.str)
		}
		if got := tt.kind.Target(); got != tt.target {
			t.Errorf("Target() = %q, want %q", got, tt.target)
		}
	}
}
