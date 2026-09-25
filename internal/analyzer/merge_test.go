package analyzer

import (
	"testing"

	"github.com/miyamo2/go-tebanare/internal/lines"
	"github.com/miyamo2/go-tebanare/internal/result"
)

func hit(id, label string) result.Hit {
	return result.Hit{RuleID: id, Target: result.TargetStmt, Node: "ExprStmt", Label: label}
}

func TestMergeSpans(t *testing.T) {
	// Lines 1-9; lines 4 and 5 are blank, line 7 is not.
	src := []byte("a\nb\nc\n\n  \nd\ne\nf\ng\n")
	blank := lines.BlankLines(src)
	spans := []span{
		{start: 6, end: 6, off: 12, endOff: 13, rule: 0, hit: hit("r1", "d")},
		{start: 2, end: 3, off: 2, endOff: 5, rule: 1, hit: hit("r2", "b")},
		{start: 2, end: 2, off: 2, endOff: 3, rule: 0, hit: hit("r1", "b")},
		{start: 1, end: 1, off: 0, endOff: 1, rule: 0, hit: hit("r1", "a")},
		{start: 8, end: 9, off: 16, endOff: 19, rule: 0, hit: hit("r1", "x")},
		{start: 9, end: 9, off: 18, endOff: 19, rule: 0, hit: hit("r1", "x")},
	}
	got := mergeSpans(spans, blank)
	check(t, "ranges", show(got), []string{"1-6 r1:a, r2:b, r1:b, r1:d", "8-9 r1:x"})
	if h := got[0].Hits[0]; h != hit("r1", "a") {
		t.Errorf("first hit = %+v", h)
	}
}

func TestMergeSpansEmpty(t *testing.T) {
	got := mergeSpans(nil, nil)
	if got == nil || len(got) != 0 {
		t.Errorf("got %#v, want an empty non-nil slice", got)
	}
}
