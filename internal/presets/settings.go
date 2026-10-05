package presets

import (
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// The settings structs of the presets, their defaults, and their decoders
// are generated from the schema (see settings_gen.go and dev/gogen). The
// schema, schema/gotebanare.schema.json, is the source of truth for the
// settings: their names, types, defaults, descriptions, and constraints.

// settingsSpec holds the generated code for the settings of one preset.
type settingsSpec struct {
	// new returns a pointer to a new settings struct that holds the
	// defaults.
	new func() any
	// decode reads a settings value that passed the schema.
	decode func(map[string]any) any
	// infos describes the settings in schema order.
	infos []SettingInfo
}

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
// all presets of the kind share come first.
func Settings(p *Preset) []SettingInfo {
	return schemaPresets[p.Name].infos
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
