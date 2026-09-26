package analyzer

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// show renders ranges as "start-end rule[:label], ...".
func show(rs []result.Range) []string {
	out := []string{}
	for _, r := range rs {
		var hits []string
		for _, h := range r.Hits {
			hits = append(hits, h.RuleID+":"+h.Label)
		}
		out = append(out, fmt.Sprintf("%d-%d %s", r.Start, r.End, strings.Join(hits, ", ")))
	}
	return out
}

func check(t *testing.T, what string, got, want []string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got  %q\n want %q", what, got, want)
	}
}
