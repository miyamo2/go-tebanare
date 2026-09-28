package sigpattern

import "testing"

func TestErrorPosition(t *testing.T) {
	tests := []struct {
		src          string
		off          int
		line, column int
	}{
		{"func F(,)", 7, 1, 8},
		{"func F(,)", 100, 1, 10},
		{"func F(,)", -1, 1, 1},
		{"func é(,)", 8, 1, 8}, // columns count runes
		{"func F(\n\t,)", 9, 2, 2},
	}
	for _, tt := range tests {
		e := newError(tt.src, tt.off, "msg")
		if e.Line != tt.line || e.Column != tt.column {
			t.Errorf("%q at %d: got %d:%d, want %d:%d", tt.src, tt.off, e.Line, e.Column, tt.line, tt.column)
		}
	}
}

func TestErrorPositionTrailingNewlines(t *testing.T) {
	// A YAML block scalar ends the pattern with a newline, and an error at
	// the end of the input points at the end of the last non-empty line.
	tests := []struct {
		src          string
		off          int
		line, column int
		caret        string
	}{
		{"func F(\n", 8, 1, 8, "func F(\n       ^"},
		{"func F(\r\n\n", 10, 1, 8, "func F(\n       ^"},
		{"func F(\n  \n", 11, 1, 8, "func F(\n       ^"},
		{"func F(\n", 7, 1, 8, "func F(\n       ^"},
		{"func F(", 7, 1, 8, "func F(\n       ^"},
		{"func F(\n)", 9, 2, 2, ")\n ^"},
		{"\n", 1, 1, 1, "\n^"},
	}
	for _, tt := range tests {
		e := newError(tt.src, tt.off, "msg")
		if e.Line != tt.line || e.Column != tt.column {
			t.Errorf("%q at %d: got %d:%d, want %d:%d", tt.src, tt.off, e.Line, e.Column, tt.line, tt.column)
		}
		if got := e.Caret(""); got != tt.caret {
			t.Errorf("%q at %d: Caret() = %q, want %q", tt.src, tt.off, got, tt.caret)
		}
	}
}
