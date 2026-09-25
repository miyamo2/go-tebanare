package sigpattern

import "strings"

// Caret returns two lines, each prefixed with indent: the pattern line that
// holds the error, and a line with a '^' under the error column. The
// result has no trailing newline. Tabs before the column are copied so the
// caret lines up in a terminal.
func (e *Error) Caret(indent string) string {
	off := min(max(e.Offset, 0), len(e.Src))
	start := strings.LastIndexByte(e.Src[:off], '\n') + 1
	end := len(e.Src)
	if i := strings.IndexByte(e.Src[off:], '\n'); i >= 0 {
		end = off + i
	}
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(strings.TrimSuffix(e.Src[start:end], "\r"))
	b.WriteByte('\n')
	b.WriteString(indent)
	for _, r := range e.Src[start:off] {
		if r == '\t' {
			b.WriteByte('\t')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte('^')
	return b.String()
}
