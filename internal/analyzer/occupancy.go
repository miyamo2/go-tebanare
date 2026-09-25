package analyzer

import (
	"bytes"
	"go/token"
	"sort"
)

// scanInfo is what one go/scanner pass learns about a file: where each
// line starts and, per line, where the code tokens on it begin and end.
// Code tokens are all tokens except comments, commas, and semicolons
// (including the ones the scanner inserts at line ends).
type scanInfo struct {
	// lineStart[i] is the offset of line i+1.
	lineStart []int
	// firstCode[i] is the start offset of the first code token that
	// touches line i+1, or -1 when there is none. A token that starts on
	// an earlier line (a raw string) touches every line it spans.
	firstCode []int
	// lastCode[i] is the end offset of the last code token that touches
	// line i+1, or -1 when there is none.
	lastCode []int
}

func newScanInfo(src []byte) *scanInfo {
	starts := make([]int, 1, bytes.Count(src, []byte{'\n'})+1)
	for i, c := range src {
		// A final newline ends the last line and does not start a new
		// one, as in token.File.
		if c == '\n' && i+1 < len(src) {
			starts = append(starts, i+1)
		}
	}
	first := make([]int, len(starts))
	last := make([]int, len(starts))
	for i := range first {
		first[i], last[i] = -1, -1
	}
	return &scanInfo{lineStart: starts, firstCode: first, lastCode: last}
}

// lines returns the number of lines of the file.
func (s *scanInfo) lines() int { return len(s.lineStart) }

// lineOf returns the 1-based line that holds offset off.
func (s *scanInfo) lineOf(off int) int {
	return sort.Search(len(s.lineStart), func(i int) bool { return s.lineStart[i] > off })
}

// lineCol returns the 1-based line and byte column of offset off.
func (s *scanInfo) lineCol(off int) (line, col int) {
	line = s.lineOf(off)
	return line, off - s.lineStart[line-1] + 1
}

// mark records the code token that covers the offsets [start, end).
// Tokens must be marked in source order.
func (s *scanInfo) mark(start, end, firstLine, lastLine int) {
	for l := firstLine; l <= lastLine; l++ {
		if s.firstCode[l-1] < 0 {
			s.firstCode[l-1] = start
		}
		s.lastCode[l-1] = end
	}
}

// tokenEnd returns the end offset of the token tok that starts at off.
// go/scanner removes carriage returns from raw strings, so the end of a
// raw string comes from the source.
func tokenEnd(src []byte, off int, tok token.Token, lit string) int {
	switch {
	case tok == token.STRING && lit != "" && lit[0] == '`':
		if i := bytes.IndexByte(src[off+1:], '`'); i >= 0 {
			return off + i + 2
		}
		return len(src)
	case lit != "":
		return off + len(lit)
	}
	return off + len(tok.String())
}

// occupies reports whether the offsets [start, end) share no line with
// other code: on the first line no code token starts before start, and on
// the last line no code token ends after end. Comments, commas, and
// semicolons may share the lines. When the check fails, line is the first
// shared line.
func (s *scanInfo) occupies(start, end int) (ok bool, line int) {
	first := s.lineOf(start)
	if c := s.firstCode[first-1]; c >= 0 && c < start {
		return false, first
	}
	last := first
	if end > start {
		last = s.lineOf(end - 1)
	}
	if c := s.lastCode[last-1]; c > end {
		return false, last
	}
	return true, 0
}
