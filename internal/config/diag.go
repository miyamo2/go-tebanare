package config

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// field is the path of a value in the config, such as
// "presets[2](iferr).names[1]".
type field string

// key returns the path of the value under key k.
func (f field) key(k string) field {
	if f == "" {
		return field(k)
	}
	return f + "." + field(k)
}

// index returns the path of the i-th item of a list.
func (f field) index(i int) field {
	return f + "[" + field(strconv.Itoa(i)) + "]"
}

// named appends a rule id or preset name, as in "rules[4](tracing)".
func (f field) named(name string) field {
	if name == "" {
		return f
	}
	return f + "(" + field(name) + ")"
}

// compiler collects the diagnostics of one Compile call.
type compiler struct {
	errs  []result.Diagnostic
	warns []result.Diagnostic
	// ruleID is the id of the rule or the name of the preset being
	// decoded. Diagnostics carry it in RuleID.
	ruleID string
}

// errorf reports an invalid value at the position of n.
func (c *compiler) errorf(n *yaml.Node, f field, format string, args ...any) {
	c.errs = append(c.errs, c.diag(result.SeverityError, result.CodeConfigInvalid, n, f, fmt.Sprintf(format, args...)))
}

// warnUnanchored reports a regular expression that is not anchored.
func (c *compiler) warnUnanchored(n *yaml.Node, f field, msg string) {
	c.warns = append(c.warns, c.diag(result.SeverityWarning, result.CodeUnanchoredRegexp, n, f, msg))
}

func (c *compiler) diag(sev result.Severity, code string, n *yaml.Node, f field, msg string) result.Diagnostic {
	d := result.Diagnostic{Severity: sev, Code: code, Message: msg, Field: string(f), RuleID: c.ruleID}
	if n != nil {
		d.Line, d.Column = n.Line, n.Column
	}
	return d
}

// sortByPosition sorts diagnostics by line and column, keeping the order
// of diagnostics at the same position.
func sortByPosition(ds []result.Diagnostic) {
	slices.SortStableFunc(ds, func(a, b result.Diagnostic) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	})
}

// unwrapAll returns the errors joined in err (see errors.Join), or err
// itself.
func unwrapAll(err error) []error {
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		return joined.Unwrap()
	}
	return []error{err}
}
