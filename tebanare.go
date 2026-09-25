package tebanare

import (
	"github.com/miyamo2/go-tebanare/internal/analyzer"
	"github.com/miyamo2/go-tebanare/internal/config"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// APIVersion is the version of the JSON interface of the WebAssembly
// engine. It changes when a field of an input or a result changes in a way
// that older callers cannot read.
const APIVersion = 1

// EngineVersion identifies the build. Release builds set it with
// -ldflags "-X github.com/miyamo2/go-tebanare.EngineVersion=<version>".
var EngineVersion = "dev"

// Result types. Their JSON encoding is the contract with the TypeScript
// engine package, which reads testdata/analyze/*/want.json in its tests.
type (
	// Range is a closed, 1-based line range together with the hits that
	// produced it.
	Range = result.Range
	// Hit describes one rule match behind a Range.
	Hit = result.Hit
	// Diagnostic is a message about the configuration or about one
	// analysis.
	Diagnostic = result.Diagnostic
	// SkipReason says why a change was not analyzed. It is empty when
	// the change was analyzed.
	SkipReason = result.SkipReason
	// ChangeResult is the outcome of analyzing one changed file.
	ChangeResult = result.ChangeResult
)

// ConfigError is the error Compile returns for an invalid configuration.
// Its Diagnostics hold every error found, sorted by position.
type ConfigError = config.Error

// FileChange is one changed file. The paths are slash-separated and
// relative to the repository root. Old is nil for an added file and New is
// nil for a deleted file. An empty, non-nil slice is an empty file.
type FileChange struct {
	OldPath, NewPath string
	Old, New         []byte
}

// Ruleset is a compiled configuration. A Ruleset can analyze any number of
// files. A nil Ruleset has no rules and treats every .go file as a target.
type Ruleset struct {
	set *rule.Set
}

// Compile validates configYAML, the content of a configuration file, and
// compiles it into a Ruleset.
//
// The returned diagnostics are warnings. Compile returns them for valid
// and invalid configurations. When the configuration is invalid, Compile
// returns a nil Ruleset and a *ConfigError.
//
// yaml.v3 reports YAML syntax errors by panicking and recovering inside
// its parser. In a build without recover, such as TinyGo for
// wasm-unknown, a syntax error traps before Compile returns.
func Compile(configYAML []byte) (*Ruleset, []Diagnostic, error) {
	set, warns, err := config.Compile(configYAML)
	if err != nil {
		return nil, warns, err
	}
	return &Ruleset{set: set}, warns, nil
}

// ruleSet returns the compiled rules, or nil for a nil Ruleset.
func (rs *Ruleset) ruleSet() *rule.Set {
	if rs == nil {
		return nil
	}
	return rs.set
}

// AnalyzeChange computes the lines to hide in both versions of one changed
// file, with the default safety limits. The slices of the result are never
// nil, so its JSON encoding holds arrays and no nulls.
//
// When a present side is not a target of the configuration, or when either
// side is skipped (too large, nested too deeply, or not parseable), nothing
// is hidden on either side and Skipped says why. The matches of stmt rules
// count on each side on its own. A func rule match is hidden
// when the declaration matches on both sides or exists on one side only.
// When one side matches and the other side declares the same function
// without matching (a declaration in a file that the paths or
// exclude_paths of the preset leave out never matches), both stay visible
// and a match-changed diagnostic reports it. When one side declares the
// same function or method twice, neither side hides it and a
// duplicate-decl diagnostic reports it.
func (rs *Ruleset) AnalyzeChange(ch FileChange) ChangeResult {
	res := analyzer.AnalyzeChange(rs.ruleSet(),
		analyzer.Side{Path: ch.OldPath, Src: ch.Old},
		analyzer.Side{Path: ch.NewPath, Src: ch.New},
		analyzer.DefaultOptions())
	res.Normalize()
	return res
}

// RuleInfo describes one enabled rule for tooltips and listings.
type RuleInfo struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	// Target is "func" or "stmt".
	Target string `json:"target"`
	// Preset is the preset name when the rule comes from `presets`.
	Preset string `json:"preset,omitempty"`
}

// Rules returns one RuleInfo per enabled preset, in the order of the
// `presets` list.
func (rs *Ruleset) Rules() []RuleInfo {
	out := []RuleInfo{}
	set := rs.ruleSet()
	if set == nil {
		return out
	}
	for _, r := range set.Rules {
		if r == nil {
			continue
		}
		out = append(out, RuleInfo{
			ID:          r.ID,
			Description: r.Description,
			Target:      string(r.Target),
			Preset:      r.Preset,
		})
	}
	return out
}
