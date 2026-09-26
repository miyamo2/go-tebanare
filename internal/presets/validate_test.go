package presets

import (
	"errors"
	"slices"
	"testing"
)

func (s *testSettings) validate() []problem {
	out := s.FuncCommon.validate()
	if s.Level != nil && *s.Level < 1 {
		out = append(out, problem{"level", -1, "must be at least 1"})
	}
	for i, tag := range s.Tags {
		if tag == "bad" {
			out = append(out, problem{"tags", i, "bad tag"})
		}
	}
	return out
}

func TestDecodeErrorString(t *testing.T) {
	tests := []struct {
		err  DecodeError
		want string
	}{
		{DecodeError{Line: 3, Column: 7, Field: "names[1]", Msg: "bad"}, "3:7: names[1]: bad"},
		{DecodeError{Line: 3, Column: 7, Msg: "bad"}, "3:7: bad"},
		{DecodeError{Field: "init", Msg: "bad"}, "init: bad"},
		{DecodeError{Msg: "bad"}, "bad"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestCheckSettings(t *testing.T) {
	s := testPreset.NewSettings().(*testSettings)
	if got, err := checkSettings[testSettings]("test", s); err != nil || got != s {
		t.Errorf("checkSettings(defaults) = %v, %v", got, err)
	}
	for _, bad := range []any{nil, (*testSettings)(nil), testSettings{}, &FuncCommon{}} {
		if _, err := checkSettings[testSettings]("test", bad); err == nil {
			t.Errorf("checkSettings(%#v) succeeded", bad)
		}
	}

	level := 0
	s.Level = &level
	s.Paths = []string{"ok/**", "bad/["}
	s.ExcludePaths = []string{"x/[", "y/**"}
	s.Tags = []string{"bad"}
	_, err := checkSettings[testSettings]("test", s)
	var got []string
	for _, e := range unwrapAll(err) {
		var de *DecodeError
		if !errors.As(e, &de) {
			t.Fatalf("error %v is not a *DecodeError", e)
		}
		got = append(got, de.Error())
	}
	want := []string{
		`paths[1]: invalid glob "bad/["`,
		`exclude_paths[0]: invalid glob "x/["`,
		"level: must be at least 1",
		"tags[0]: bad tag",
	}
	if !slices.Equal(got, want) {
		t.Errorf("errors =\n%q\nwant\n%q", got, want)
	}
}

// unwrapAll returns the errors joined in err.
func unwrapAll(err error) []error {
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		return joined.Unwrap()
	}
	if err == nil {
		return nil
	}
	return []error{err}
}
