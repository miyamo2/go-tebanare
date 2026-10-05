package presets

import (
	"github.com/miyamo2/go-tebanare/internal/configschema"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// The settings types of the presets are generated from the schema, the
// source of truth for the settings: their names, types, defaults,
// descriptions, and constraints (see internal/configschema).
type (
	GetterSettings = configschema.GetterSettings
	IferrSettings  = configschema.IferrSettings
	NoopSettings   = configschema.NoopSettings
)

// SettingInfo describes one setting of a preset.
type SettingInfo struct {
	Name string `json:"name"`
	// Type is "bool", "int", "string", or "[]string". The schema accepts
	// int values from -2147483648 to 2147483647 (the int32 range).
	Type string `json:"type"`
	// Default is the default value as shown in docs, such as "true",
	// "[err]", or "none".
	Default     string `json:"default"`
	Description string `json:"description"`
	// Enum lists the allowed values of a string setting.
	Enum []string `json:"enum,omitempty"`
}

// Settings describes the settings of p in schema order. The settings that
// all presets of the kind share come first. It returns nil for a preset
// that the schema does not declare.
func Settings(p *Preset) []SettingInfo {
	settings, err := configschema.Settings(p.Name)
	if err != nil {
		return nil
	}
	out := make([]SettingInfo, len(settings))
	for i, s := range settings {
		out[i] = SettingInfo{Name: s.Name, Type: s.Type, Default: s.Default, Description: s.Description, Enum: s.Enum}
	}
	return out
}

// Values of the settings, which the schema fills with their defaults.
func boolValue(p *bool) bool { return p != nil && *p }

func intValue(p *int64) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

// funcRule returns the func rule of the preset with the given name.
func funcRule(name, summary string, paths, excludePaths []string, includeDoc bool, m rule.FuncMatcher) *rule.Rule {
	return &rule.Rule{
		ID:           name,
		Description:  summary,
		Preset:       name,
		Target:       result.TargetFunc,
		Paths:        paths,
		ExcludePaths: excludePaths,
		IncludeDoc:   includeDoc,
		Func:         m,
	}
}

// stmtRule returns the stmt rule of the preset with the given name.
func stmtRule(name, summary string, paths, excludePaths []string, m rule.NodeMatcher) *rule.Rule {
	return &rule.Rule{
		ID:           name,
		Description:  summary,
		Preset:       name,
		Target:       result.TargetStmt,
		Paths:        paths,
		ExcludePaths: excludePaths,
		Node:         m,
	}
}
