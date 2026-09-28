package sigpattern

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Error is a syntax error in a pattern.
type Error struct {
	// Offset is the byte offset of the error in Src. An offset past the
	// last non-empty line is moved to the end of that line.
	Offset int
	// Line is the 1-based line of Offset. It is 1 unless the pattern
	// contains newlines.
	Line int
	// Column is the 1-based column of Offset within its line, counted in
	// runes.
	Column int
	Msg    string
	Src    string
}

func newError(src string, off int, msg string) *Error {
	off = min(max(off, 0), len(src))
	// A pattern from a YAML block scalar ends with a newline. An error
	// past the last non-empty line, such as at the end of the input,
	// points at the end of that line instead of at an empty line.
	if last := len(strings.TrimRight(src, " \t\r\n")); off > last && strings.Contains(src[last:off], "\n") {
		off = last + strings.IndexByte(src[last:], '\n')
		if off > last && src[off-1] == '\r' {
			off--
		}
	}
	start := strings.LastIndexByte(src[:off], '\n') + 1
	return &Error{
		Offset: off,
		Line:   strings.Count(src[:off], "\n") + 1,
		Column: utf8.RuneCountInString(src[start:off]) + 1,
		Msg:    msg,
		Src:    src,
	}
}

// Error returns "<line>:<column>: <message>", such as
// "1:29: expected type, found ','".
func (e *Error) Error() string {
	return strconv.Itoa(e.Line) + ":" + strconv.Itoa(e.Column) + ": " + e.Msg
}
