package sigpattern

import "testing"

func TestErrorCaret(t *testing.T) {
	// The example from the plan: an error at column 29, shown with a
	// four-space indent.
	src := "func (*Repository[_]) Find*(, ...) (_, error)"
	e := newError(src, 28, "expected type, found ','")
	if got, want := e.Error(), "1:29: expected type, found ','"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	want := "    func (*Repository[_]) Find*(, ...) (_, error)\n" +
		"                                ^"
	if got := e.Caret("    "); got != want {
		t.Errorf("Caret() =\n%s\nwant\n%s", got, want)
	}

	// Only the line with the error is shown, and tabs before the column
	// are kept so the caret lines up.
	e = newError("func F(\n\tx,\r\n\t?)", 14, "msg")
	if got, want := e.Caret(""), "\t?)\n\t^"; got != want {
		t.Errorf("Caret() = %q, want %q", got, want)
	}
	if got, want := e.Error(), "3:2: msg"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	e = newError("func F(,\n)", 7, "msg")
	if got, want := e.Caret("  "), "  func F(,\n         ^"; got != want {
		t.Errorf("Caret() = %q, want %q", got, want)
	}
}
