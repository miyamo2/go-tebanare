package presets

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		glob, name string
		want       bool
	}{
		{"err", "err", true},
		{"err", "errs", false},
		{"err", "er", false},
		{"*Err", "parseErr", true},
		{"*Err", "Err", true},
		{"*Err", "parseError", false},
		{"*Err", "parseErrErr", true},
		{"e?", "e1", true},
		{"e?", "eé", true},
		{"e?", "e", false},
		{"e?", "e12", false},
		{"*", "", true},
		{"*", "anything", true},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "abc", true},
		{"a*b*c", "acb", false},
		{"*a*", "bab", true},
		{"?*", "", false},
		{"é*", "éa", true},
		{"", "", true},
		{"", "a", false},
	}
	for _, tt := range tests {
		if got := matchGlob(tt.glob, tt.name); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.glob, tt.name, got, tt.want)
		}
	}
}
