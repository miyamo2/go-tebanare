package tebanare

import "github.com/miyamo2/go-tebanare/internal/presets"

// Preset description types.
type (
	// SettingInfo describes one setting of a preset.
	SettingInfo = presets.SettingInfo
	// PresetExample is a code sample of a preset and whether it matches.
	PresetExample = presets.Example
)

// PresetInfo describes a built-in preset for documentation and settings
// screens.
type PresetInfo struct {
	Name string `json:"name"`
	// Summary is one line for tooltips and listings.
	Summary string `json:"summary"`
	// Criteria lists the requirements that code must meet to match with
	// the default settings, one sentence each.
	Criteria []string `json:"criteria"`
	// Kind is "func" for presets that hide function declarations and
	// "stmt" for presets that hide statements.
	Kind string `json:"kind"`
	// Settings lists the settings in declaration order, the settings that
	// all presets of the kind share first.
	Settings []SettingInfo   `json:"settings"`
	Examples []PresetExample `json:"examples"`
}

// Presets returns the built-in presets sorted by name. The slices in the
// result are never nil, and the caller may modify them.
func Presets() []PresetInfo {
	all := presets.All()
	out := make([]PresetInfo, 0, len(all))
	for _, p := range all {
		out = append(out, PresetInfo{
			Name:     p.Name,
			Summary:  p.Summary,
			Criteria: append([]string{}, p.Criteria...),
			Kind:     p.Kind.String(),
			Settings: append([]SettingInfo{}, presets.Settings(p)...),
			Examples: append([]PresetExample{}, p.Examples...),
		})
	}
	return out
}
