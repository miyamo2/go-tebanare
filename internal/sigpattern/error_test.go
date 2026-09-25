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
