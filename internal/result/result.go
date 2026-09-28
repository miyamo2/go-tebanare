// Package result defines the analysis result types that the analyzer
// produces and the public facade returns.
//
// The JSON field names are part of the contract with the TypeScript engine
// package. Change them only together with packages/engine/src/types.ts.
// dev/tsconstgen copies the exported constants and string types into
// packages/engine/src/generated/constants.ts; run make constants after
// changing them.
package result

import (
	"fmt"
	"strings"
)

// SkipReason says why a file was not analyzed. The empty value means the
// file was analyzed.
type SkipReason string

// Skip reasons. When either side of a change is skipped, nothing is hidden on
// either side.
const (
	NotSkipped     SkipReason = ""
	SkipNotTarget  SkipReason = "not-target"
	SkipTooLarge   SkipReason = "too-large"
	SkipTooDeep    SkipReason = "too-deep"
	SkipParseError SkipReason = "parse-error"
)

// Target is the kind of rule that produced a hit.
type Target string

// Rule targets.
const (
	TargetFunc Target = "func"
	TargetStmt Target = "stmt"
)

// Hit describes one rule match that produced (part of) a hidden range.
type Hit struct {
	RuleID string `json:"ruleId"`
	Target Target `json:"target"`
	// Node is the go/ast type name of the matched node, such as "FuncDecl".
	Node string `json:"node"`
	// Label is a short human-readable description of the match, such as
	// "func (*Repository[T]) FindByID" or the start of the normalized text.
	Label string `json:"label"`
	// Preset is the preset name when the rule comes from `presets`.
	Preset string `json:"preset,omitempty"`
}

// Range is a closed, 1-based line range together with the hits that
// produced it.
type Range struct {
	Start int   `json:"start"`
	End   int   `json:"end"`
	Hits  []Hit `json:"hits"`
}

// Severity of a diagnostic.
type Severity string

// Diagnostic severities.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Diagnostic codes. The messages are not part of the compatibility
// contract; the codes are.
const (
	// Configuration diagnostics.
	CodeConfigSyntax  = "config-syntax"
	CodeConfigInvalid = "config-invalid"
	// CodeConfigIgnored marks a configuration file that is ignored because
	// a file earlier in the lookup order exists.
	CodeConfigIgnored = "config-ignored"

	// Analysis diagnostics.
	CodeLineShared    = "line-shared"
	CodeMatchChanged  = "match-changed"
	CodeDuplicateDecl = "duplicate-decl"
	CodeSkipped       = "skipped"
)

// Side names used in Diagnostic.Side.
const (
	SideOld = "old"
	SideNew = "new"
)

// Diagnostic is a message about the configuration or about one analysis.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	// Field is the configuration path, such as "presets[0](getter).max_depth".
	Field string `json:"field,omitempty"`
	// Side is "old" or "new" for analysis diagnostics.
	Side   string `json:"side,omitempty"`
	RuleID string `json:"ruleId,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// Format renders the diagnostic as "file:line:col: field: message". Parts
// that are empty or zero are left out.
func (d Diagnostic) Format(file string) string {
	var b strings.Builder
	if file != "" {
		b.WriteString(file)
		b.WriteString(":")
	}
	if d.Line > 0 {
		fmt.Fprintf(&b, "%d:", d.Line)
		if d.Column > 0 {
			fmt.Fprintf(&b, "%d:", d.Column)
		}
	}
	if b.Len() > 0 {
		b.WriteString(" ")
	}
	if d.Field != "" {
		b.WriteString(d.Field)
		b.WriteString(": ")
	}
	b.WriteString(d.Message)
	return b.String()
}

// String implements fmt.Stringer.
func (d Diagnostic) String() string { return d.Format("") }

// ChangeResult is the outcome of analyzing one changed file.
type ChangeResult struct {
	// Old and New hold the hidden line ranges of each side, sorted and
	// without overlaps.
	Old         []Range      `json:"old"`
	New         []Range      `json:"new"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Skipped     SkipReason   `json:"skipped"`
}

// Normalize replaces nil slices with empty ones so that the JSON encoding
// always has arrays, never null.
func (r *ChangeResult) Normalize() {
	if r.Old == nil {
		r.Old = []Range{}
	}
	if r.New == nil {
		r.New = []Range{}
	}
	if r.Diagnostics == nil {
		r.Diagnostics = []Diagnostic{}
	}
	for i := range r.Old {
		if r.Old[i].Hits == nil {
			r.Old[i].Hits = []Hit{}
		}
	}
	for i := range r.New {
		if r.New[i].Hits == nil {
			r.New[i].Hits = []Hit{}
		}
	}
}
