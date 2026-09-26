// Package lines does arithmetic on sets of source lines stored as closed,
// 1-based ranges.
//
// Every function except Contains accepts ranges in any order, including
// overlapping and invalid ones. Every returned slice is normalized: sorted
// by Start, without overlapping or adjacent ranges, and without invalid
// ranges.
package lines

import (
	"bytes"
	"cmp"
	"slices"
	"sort"
)

// Range is the closed line interval [Start, End]. Lines are 1-based.
type Range struct {
	Start int
	End   int
}

func (r Range) valid() bool { return r.Start >= 1 && r.End >= r.Start }

// Normalize returns rs sorted by Start, with overlapping and adjacent
// ranges merged. Ranges [1, 2] and [3, 4] are adjacent and become [1, 4].
// It drops invalid ranges (Start < 1 or End < Start) and does not modify
// rs. The result is nil when no valid range remains.
func Normalize(rs []Range) []Range {
	out := make([]Range, 0, len(rs))
	for _, r := range rs {
		if r.valid() {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortFunc(out, func(a, b Range) int {
		if c := cmp.Compare(a.Start, b.Start); c != 0 {
			return c
		}
		return cmp.Compare(a.End, b.End)
	})
	merged := out[:1]
	for _, r := range out[1:] {
		last := &merged[len(merged)-1]
		// r.Start-1 cannot overflow because r.Start >= 1.
		if r.Start-1 <= last.End {
			last.End = max(last.End, r.End)
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// MergeAcrossBlank normalizes rs and then also merges two neighboring
// ranges when blank reports true for every line strictly between them.
// A nil blank merges nothing beyond Normalize.
func MergeAcrossBlank(rs []Range, blank func(line int) bool) []Range {
	rs = Normalize(rs)
	if len(rs) < 2 || blank == nil {
		return rs
	}
	merged := rs[:1]
	for _, r := range rs[1:] {
		last := &merged[len(merged)-1]
		if allBlank(last.End+1, r.Start-1, blank) {
			last.End = r.End
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

func allBlank(from, to int, blank func(line int) bool) bool {
	for line := from; line <= to; line++ {
		if !blank(line) {
			return false
		}
	}
	return true
}

// Contains reports whether line is in one of the ranges. rs must be
// normalized; Contains uses binary search.
func Contains(rs []Range, line int) bool {
	i := sort.Search(len(rs), func(i int) bool { return rs[i].End >= line })
	return i < len(rs) && rs[i].Start <= line
}

// Union returns the lines in a or in b.
func Union(a, b []Range) []Range {
	all := make([]Range, 0, len(a)+len(b))
	all = append(all, a...)
	all = append(all, b...)
	return Normalize(all)
}

// Intersect returns the lines in both a and b.
func Intersect(a, b []Range) []Range {
	a, b = Normalize(a), Normalize(b)
	var out []Range
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		start := max(a[i].Start, b[j].Start)
		end := min(a[i].End, b[j].End)
		if start <= end {
			out = append(out, Range{Start: start, End: end})
		}
		if a[i].End < b[j].End {
			i++
		} else {
			j++
		}
	}
	return out
}

// Subtract returns the lines in a that are not in b.
func Subtract(a, b []Range) []Range {
	a, b = Normalize(a), Normalize(b)
	var out []Range
	j := 0
	for _, r := range a {
		for j < len(b) && b[j].End < r.Start {
			j++
		}
		start := r.Start
		covered := false
		for k := j; k < len(b) && b[k].Start <= r.End; k++ {
			if b[k].Start > start {
				out = append(out, Range{Start: start, End: b[k].Start - 1})
			}
			if b[k].End >= r.End {
				covered = true
				break
			}
			start = b[k].End + 1
		}
		if !covered {
			out = append(out, Range{Start: start, End: r.End})
		}
	}
	return out
}

// Count returns the number of distinct lines covered by rs.
func Count(rs []Range) int {
	n := 0
	for _, r := range Normalize(rs) {
		n += r.End - r.Start + 1
	}
	return n
}

// BlankLines returns a function that reports whether a line of src is
// empty or holds only ASCII whitespace (space, \t, \r, \f, \v). Lines end
// at "\n", so a "\r\n" line ending leaves a "\r" that counts as
// whitespace. A final "\n" ends the last line and does not start a new
// one. Lines outside the file, including every line of an empty src, are
// not blank.
func BlankLines(src []byte) func(line int) bool {
	blank := make([]bool, 0, bytes.Count(src, []byte{'\n'})+1)
	cur := true
	for _, c := range src {
		switch {
		case c == '\n':
			blank = append(blank, cur)
			cur = true
		case !isSpace(c):
			cur = false
		}
	}
	if len(src) > 0 && src[len(src)-1] != '\n' {
		blank = append(blank, cur)
	}
	return func(line int) bool {
		return line >= 1 && line <= len(blank) && blank[line-1]
	}
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}
