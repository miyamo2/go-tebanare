package analyzer

import (
	"cmp"
	"slices"
	"sort"

	"github.com/miyamo2/go-tebanare/internal/lines"
	"github.com/miyamo2/go-tebanare/internal/result"
)

// span is an accepted hidden range together with the hit that produced
// it.
type span struct {
	// start and end are the first and last line.
	start, end int
	// off and endOff are the byte offsets of the range; with rule they
	// put the hits of a merged range in source order.
	off, endOff int
	// rule is the index of the rule in the set.
	rule int
	hit  result.Hit
}

// mergeSpans merges the line ranges of spans with lines.MergeAcrossBlank:
// overlapping and adjacent ranges and ranges with only blank lines
// between them become one. Each merged range carries the hits of its
// spans in source order, without duplicates. The result is never nil.
func mergeSpans(spans []span, blank func(line int) bool) []result.Range {
	if len(spans) == 0 {
		return []result.Range{}
	}
	rs := make([]lines.Range, len(spans))
	for i, s := range spans {
		rs[i] = lines.Range{Start: s.start, End: s.end}
	}
	merged := lines.MergeAcrossBlank(rs, blank)

	ordered := slices.Clone(spans)
	slices.SortStableFunc(ordered, func(a, b span) int {
		if c := cmp.Compare(a.off, b.off); c != 0 {
			return c
		}
		if c := cmp.Compare(b.endOff, a.endOff); c != 0 {
			return c
		}
		return cmp.Compare(a.rule, b.rule)
	})

	out := make([]result.Range, len(merged))
	for i, m := range merged {
		out[i] = result.Range{Start: m.Start, End: m.End, Hits: []result.Hit{}}
	}
	type seenKey struct {
		rng int
		hit result.Hit
	}
	seen := make(map[seenKey]bool, len(ordered))
	for _, s := range ordered {
		i := sort.Search(len(merged), func(i int) bool { return merged[i].End >= s.start })
		if i == len(merged) || merged[i].Start > s.start {
			// Unreachable: every span lies inside a merged range.
			continue
		}
		if k := (seenKey{i, s.hit}); !seen[k] {
			seen[k] = true
			out[i].Hits = append(out[i].Hits, s.hit)
		}
	}
	return out
}
