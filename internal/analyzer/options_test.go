package analyzer

import (
	"testing"

	"github.com/miyamo2/go-tebanare/internal/canon"
)

func TestOptions(t *testing.T) {
	want := Options{MaxFileSize: 1 << 20, MaxBracketDepth: 200, MaxElseIfChain: 1000, MaxASTDepth: 1500, MaxNodeSize: canon.DefaultMaxNodeSize}
	if got := DefaultOptions(); got != want {
		t.Errorf("DefaultOptions() = %+v, want %+v", got, want)
	}
	if got := (Options{}).withDefaults(); got != want {
		t.Errorf("zero options with defaults = %+v, want %+v", got, want)
	}
	custom := Options{MaxFileSize: 1, MaxBracketDepth: 2, MaxElseIfChain: 3, MaxASTDepth: 4, MaxNodeSize: 5}
	if got := custom.withDefaults(); got != custom {
		t.Errorf("custom options with defaults = %+v, want %+v", got, custom)
	}
}
