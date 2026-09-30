// Package presets holds the built-in presets: named rules that a config
// enables by listing them under `presets`.
//
// Each preset has a settings struct with defaults (see Settings and
// Decode), criteria written for people, examples, and a Compile function
// that turns validated settings into a rule.Rule. The analyzer runs preset
// rules the same way as user rules.
//
// A change to a preset never makes it hide more code. A broader rule ships
// as a new preset, or as a new setting whose default keeps the current
// behavior. Setting defaults follow the same rule.
package presets

import (
	"slices"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// Kind is the unit a preset hides.
type Kind int

// Preset kinds.
const (
	// FuncKind presets hide whole function declarations.
	FuncKind Kind = iota
	// StmtKind presets hide statements.
	StmtKind
)

// String returns "func" or "stmt".
func (k Kind) String() string {
	return string(k.Target())
}

// Target returns the rule target of presets of this kind.
func (k Kind) Target() result.Target {
	if k == StmtKind {
		return result.TargetStmt
	}
	return result.TargetFunc
}

// Preset is one built-in preset.
type Preset struct {
	Name string
	// Summary is one line for tooltips and listings.
	Summary string
	// Criteria lists the requirements a declaration or statement must
	// meet with the default settings, one sentence each.
	Criteria []string
	Kind     Kind
	// NewSettings returns a pointer to a new settings struct that holds
	// the default values.
	NewSettings func() any
	// Compile validates settings (a value returned by NewSettings or
	// Decode) and builds the rule. The rule's ID and Preset are Name. An
	// invalid value gives an error that wraps one *DecodeError per
	// problem, without positions.
	Compile  func(settings any) (*rule.Rule, error)
	Examples []Example
}

// Example is a code sample for the documentation. The tests run every
// example.
type Example struct {
	// Settings is the preset's settings as YAML. Empty means defaults.
	Settings string `json:"settings,omitempty"`
	// Code is one function declaration for func presets, or one if
	// statement for stmt presets.
	Code  string `json:"code"`
	Match bool   `json:"match"`
	// Note says why the code matches or not.
	Note string `json:"note,omitempty"`
}

// registry holds every preset, sorted by name.
var registry []*Preset

// register adds p to the registry. Each preset file calls it from init.
func register(p *Preset) {
	i, _ := slices.BinarySearchFunc(registry, p.Name, func(q *Preset, name string) int {
		return strings.Compare(q.Name, name)
	})
	registry = slices.Insert(registry, i, p)
}

// All returns every preset, sorted by name.
func All() []*Preset {
	return slices.Clone(registry)
}

// Lookup returns the preset with the given name.
func Lookup(name string) (*Preset, bool) {
	for _, p := range registry {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}

// Names returns the names of all presets, sorted.
func Names() []string {
	names := make([]string, len(registry))
	for i, p := range registry {
		names[i] = p.Name
	}
	return names
}
