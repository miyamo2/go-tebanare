package presets

import (
	"reflect"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// Settings structs declare each setting as a field with these tags:
//
//	yaml:"name"          the key in the config
//	default:"text"       the default as shown in docs: YAML text of the
//	                     value, or a word such as "none" when the field is nil
//	doc:"sentence"       one sentence for the docs
//	enum:"a,b"           the allowed values of a string setting
//
// Settings reads the tags, and Decode uses them to check keys and types.
// A test checks that each default tag agrees with NewSettings.

// FuncCommon holds the settings shared by func presets. Presets embed it
// first with `yaml:",inline"`.
type FuncCommon struct {
	Paths        []string `yaml:"paths" default:"none" doc:"Globs that limit the files this preset applies to, on top of 'files.include' and 'files.exclude'."`
	ExcludePaths []string `yaml:"exclude_paths" default:"none" doc:"Globs of files this preset skips, on top of 'files.exclude'."`
	IncludeDoc   *bool    `yaml:"include_doc" default:"true" doc:"Hide the doc comment together with the function."`
}

// StmtCommon holds the settings shared by stmt presets. Presets embed it
// first with `yaml:",inline"`.
type StmtCommon struct {
	Paths        []string `yaml:"paths" default:"none" doc:"Globs that limit the files this preset applies to, on top of 'files.include' and 'files.exclude'."`
	ExcludePaths []string `yaml:"exclude_paths" default:"none" doc:"Globs of files this preset skips, on top of 'files.exclude'."`
}

func newFuncCommon() FuncCommon {
	includeDoc := true
	return FuncCommon{IncludeDoc: &includeDoc}
}

// includeDoc returns the include_doc setting. nil means the default, true.
func (c *FuncCommon) includeDoc() bool {
	return c.IncludeDoc == nil || *c.IncludeDoc
}

// newRule returns the func rule of the preset with the given name.
func (c *FuncCommon) newRule(name, summary string, m rule.FuncMatcher) *rule.Rule {
	return &rule.Rule{
		ID:           name,
		Description:  summary,
		Preset:       name,
		Target:       result.TargetFunc,
		Paths:        c.Paths,
		ExcludePaths: c.ExcludePaths,
		IncludeDoc:   c.includeDoc(),
		Func:         m,
	}
}

// newRule returns the stmt rule of the preset with the given name.
func (c *StmtCommon) newRule(name, summary string, m rule.NodeMatcher) *rule.Rule {
	return &rule.Rule{
		ID:           name,
		Description:  summary,
		Preset:       name,
		Target:       result.TargetStmt,
		Paths:        c.Paths,
		ExcludePaths: c.ExcludePaths,
		Node:         m,
	}
}

// SettingInfo describes one setting of a preset.
type SettingInfo struct {
	Name string `json:"name"`
	// Type is "bool", "int", "string", or "[]string". Decode accepts int
	// values from -2147483648 to 2147483647 (the int32 range).
	Type string `json:"type"`
	// Default is the default value as shown in docs, such as "true",
	// "[err]", or "none".
	Default     string `json:"default"`
	Description string `json:"description"`
	// Enum lists the allowed values of a string setting.
	Enum []string `json:"enum,omitempty"`
}

// Settings describes the settings of p in declaration order. The common
// settings come first.
func Settings(p *Preset) []SettingInfo {
	return settingInfos(reflect.TypeOf(p.NewSettings()))
}

func settingInfos(t reflect.Type) []SettingInfo {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var out []SettingInfo
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if opts == "inline" {
			out = append(out, settingInfos(f.Type)...)
			continue
		}
		if name == "" || name == "-" {
			continue
		}
		info := SettingInfo{
			Name:        name,
			Type:        typeName(f.Type),
			Default:     f.Tag.Get("default"),
			Description: docText(f.Tag.Get("doc")),
		}
		if e := f.Tag.Get("enum"); e != "" {
			info.Enum = strings.Split(e, ",")
		}
		out = append(out, info)
	}
	return out
}

func typeName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.Int:
		return "int"
	case reflect.String:
		return "string"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String {
			return "[]string"
		}
	}
	return t.String()
}

// docText turns the single-quoted code spans of a doc tag into Markdown code
// spans. Struct tags cannot contain backquotes, so doc tags write code as
// 'u.name' and never use apostrophes.
func docText(tag string) string {
	return strings.ReplaceAll(tag, "'", "`")
}
