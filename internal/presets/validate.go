package presets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DecodeError describes an invalid preset setting: one that passed the
// schema but fails a check that the schema cannot express, such as an
// invalid glob.
type DecodeError struct {
	// Line and Column give the 1-based position in the YAML source. Both
	// are 0 when the error has no position.
	Line   int
	Column int
	// Field is the setting, such as "paths" or "paths[1]". It is empty
	// when the error concerns the settings as a whole.
	Field string
	Msg   string
}

// Error returns "<line>:<column>: <field>: <msg>", leaving out the parts
// that are unset.
func (e *DecodeError) Error() string {
	var b strings.Builder
	if e.Line > 0 {
		fmt.Fprintf(&b, "%d:%d: ", e.Line, e.Column)
	}
	if e.Field != "" {
		b.WriteString(e.Field)
		b.WriteString(": ")
	}
	b.WriteString(e.Msg)
	return b.String()
}

// problem is an invalid setting value reported by a validate method.
type problem struct {
	key   string // setting name
	index int    // list index, or -1
	msg   string
}

func (p problem) field() string {
	if p.index < 0 {
		return p.key
	}
	return fmt.Sprintf("%s[%d]", p.key, p.index)
}

// validator checks the values of a settings struct. Each settings struct
// implements it.
type validator interface {
	validate() []problem
}

// checkSettings validates settings for Compile. The schema checks the
// types, ranges, and patterns; validate methods check the rest. It returns the settings as
// *T, or an error that wraps one *DecodeError per problem.
func checkSettings[T any](preset string, settings any) (*T, error) {
	s, ok := settings.(*T)
	if !ok || s == nil {
		return nil, fmt.Errorf("presets: %s: settings have type %T, want %T", preset, settings, s)
	}
	var errs []error
	if v, ok := settings.(validator); ok {
		for _, p := range v.validate() {
			errs = append(errs, &DecodeError{Field: p.field(), Msg: p.msg})
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return s, nil
}

func validatePaths(paths, excludePaths []string) []problem {
	var out []problem
	check := func(key string, globs []string) {
		for i, g := range globs {
			if !doublestar.ValidatePattern(g) {
				out = append(out, problem{key, i, fmt.Sprintf("invalid glob %q", g)})
			}
		}
	}
	check("paths", paths)
	check("exclude_paths", excludePaths)
	return out
}
