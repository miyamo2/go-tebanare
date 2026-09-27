// Package schemagen generates the JSON Schema of the configuration file,
// schema/gotebanare.schema.json, from the preset declarations.
package schemagen

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"

	"go.yaml.in/yaml/v3"

	tebanare "github.com/miyamo2/go-tebanare"
)

// SchemaID is the URL where the JSON Schema of the configuration is
// published: the file schema/gotebanare.schema.json on the main branch.
const SchemaID = "https://raw.githubusercontent.com/miyamo2/go-tebanare/main/schema/gotebanare.schema.json"

// settingConstraints holds the checks of the settings that the preset
// validate methods make beyond type and enum, keyed by "preset.setting".
// The schema cannot read them from the settings structs.
var settingConstraints = map[string]func(*schema){
	"getter.max_depth": func(s *schema) { s.Minimum = ptr[int64](1) },
	"iferr.names": func(s *schema) {
		s.MinItems = ptr(1)
		// validNameGlob: letters, digits, "_", "*", and "?".
		s.Items.Pattern = `^[\p{L}\p{Nd}_*?]+$`
	},
}

// schema is a JSON Schema (draft-07) object. Fields are in the order
// they appear in the output.
type schema struct {
	Schema      string `json:"$schema,omitempty"`
	ID          string `json:"$id,omitempty"`
	Ref         string `json:"$ref,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	// MarkdownDescription is read by editors such as VS Code in place of
	// Description.
	MarkdownDescription  string    `json:"markdownDescription,omitempty"`
	Type                 any       `json:"type,omitempty"`
	Const                any       `json:"const,omitempty"`
	Enum                 []any     `json:"enum,omitempty"`
	Default              any       `json:"default,omitempty"`
	Minimum              *int64    `json:"minimum,omitempty"`
	Maximum              *int64    `json:"maximum,omitempty"`
	Pattern              string    `json:"pattern,omitempty"`
	Items                *schema   `json:"items,omitempty"`
	MinItems             *int      `json:"minItems,omitempty"`
	Required             []string  `json:"required,omitempty"`
	Properties           props     `json:"properties,omitempty"`
	AdditionalProperties *bool     `json:"additionalProperties,omitempty"`
	MinProperties        *int      `json:"minProperties,omitempty"`
	MaxProperties        *int      `json:"maxProperties,omitempty"`
	AnyOf                []*schema `json:"anyOf,omitempty"`
	Definitions          props     `json:"definitions,omitempty"`
}

// prop is one entry of props.
type prop struct {
	name   string
	schema *schema
}

// props is a JSON object that keeps its keys in order.
type props []prop

func (ps props) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range ps {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := marshal(p.name)
		if err != nil {
			return nil, err
		}
		v, err := marshal(p.schema)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshal encodes v without escaping HTML characters.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func ptr[T any](v T) *T { return &v }

// nullable returns the type t, or null. A null value counts as unset.
func nullable(t string) []string { return []string{t, "null"} }

// describe sets the description of s. text may hold Markdown code spans.
func (s *schema) describe(text string) *schema {
	s.Description = text
	s.MarkdownDescription = text
	return s
}

// JSONSchema returns the JSON Schema of the configuration file,
// schema/gotebanare.schema.json. The output ends with one newline.
//
// The schema is for editors and linters. It accepts every valid
// configuration, but it cannot catch every error that the engine reports,
// such as a preset listed twice or an invalid glob.
func JSONSchema() string {
	return jsonSchema(tebanare.Presets())
}

func jsonSchema(ps []tebanare.PresetInfo) string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}

	globs := func(text string) *schema {
		return (&schema{
			Type:  nullable("array"),
			Items: &schema{Type: "string"},
		}).describe(text)
	}

	var defs props
	var byName props
	for _, p := range ps {
		def := p.Name + "Settings"
		defs = append(defs, prop{def, presetSettings(p)})
		byName = append(byName, prop{p.Name, &schema{Ref: "#/definitions/" + def}})
	}

	root := &schema{
		Schema:               "http://json-schema.org/draft-07/schema#",
		ID:                   SchemaID,
		Title:                "go-tebanare configuration",
		Type:                 "object",
		Required:             []string{"version"},
		AdditionalProperties: ptr(false),
		Properties: props{
			{"version", (&schema{Type: "integer", Const: 1}).describe("Version of the configuration format. Must be `1`.")},
			{"files", (&schema{
				Type:                 nullable("object"),
				AdditionalProperties: ptr(false),
				Properties: props{
					{"include", globs("Files to analyze, as doublestar globs relative to the repository root. The default, also used for an empty list, is `[\"**/*.go\"]`.")},
					{"exclude", globs("Files to skip even when `include` matches them.")},
				},
			}).describe("The files to analyze.")},
			{"presets", (&schema{
				Type: nullable("array"),
				Items: (&schema{
					AnyOf: []*schema{
						(&schema{Type: "string", Enum: anys(names)}).describe("A preset with its default settings."),
						(&schema{
							Type:                 "object",
							Properties:           byName,
							AdditionalProperties: ptr(false),
							MinProperties:        ptr(1),
							MaxProperties:        ptr(1),
						}).describe("A preset name as the only key, with its settings as the value."),
					},
				}).describe("A preset name, or `name: settings` to change its settings."),
			}).describe("Built-in presets to enable. Listing a preset twice is an error.")},
		},
		Definitions: defs,
	}
	root.describe("Configuration of go-tebanare, read from `.gotebanare.yml` or `.gotebanare.yaml` at the repository root. See https://github.com/miyamo2/go-tebanare/blob/main/docs/configuration.md.")

	b, err := marshalIndent(root)
	if err != nil {
		panic(err) // the schema holds only encodable values
	}
	return string(b) + "\n"
}

// presetSettings returns the schema of the settings of p.
func presetSettings(p tebanare.PresetInfo) *schema {
	var ps props
	for _, s := range p.Settings {
		ps = append(ps, prop{s.Name, setting(p.Name, s)})
	}
	return (&schema{
		Type:                 nullable("object"),
		Properties:           ps,
		AdditionalProperties: ptr(false),
	}).describe("Settings of the `" + p.Name + "` preset. " + p.Summary +
		" Settings left out keep their defaults. See https://github.com/miyamo2/go-tebanare/blob/main/docs/presets.md#" + p.Name + ".")
}

// setting returns the schema of setting s of the preset with the given
// name.
func setting(preset string, s tebanare.SettingInfo) *schema {
	out := &schema{}
	switch s.Type {
	case "bool":
		out.Type = nullable("boolean")
	case "int":
		out.Type = nullable("integer")
		out.Minimum, out.Maximum = ptr[int64](math.MinInt32), ptr[int64](math.MaxInt32)
	case "string":
		out.Type = nullable("string")
		if len(s.Enum) > 0 {
			// null stays valid: it keeps the default.
			out.Enum = append(anys(s.Enum), nil)
		}
	case "[]string":
		out.Type = nullable("array")
		out.Items = &schema{Type: "string"}
	default:
		panic("presetdoc: unsupported setting type " + s.Type)
	}
	out.Default = defaultValue(s)
	if c := settingConstraints[preset+"."+s.Name]; c != nil {
		c(out)
	}
	return out.describe(s.Description + " Default: " + s.Default + ".")
}

// defaultValue returns the default of s as a JSON value, or nil when the
// default is a word such as "none" rather than a value of the setting's
// type.
func defaultValue(s tebanare.SettingInfo) any {
	var v any
	if err := yaml.Unmarshal([]byte(s.Default), &v); err != nil {
		return nil
	}
	ok := false
	switch s.Type {
	case "bool":
		_, ok = v.(bool)
	case "int":
		_, ok = v.(int)
	case "string":
		_, ok = v.(string)
		ok = ok && (len(s.Enum) == 0 || slices.Contains(s.Enum, v.(string)))
	case "[]string":
		var items []any
		items, ok = v.([]any)
		for _, it := range items {
			if _, isStr := it.(string); !isStr {
				ok = false
			}
		}
	}
	if !ok {
		return nil
	}
	return v
}

// anys converts list to []any.
func anys(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// marshalIndent encodes v with two-space indentation and without escaping
// HTML characters.
func marshalIndent(v any) ([]byte, error) {
	b, err := marshal(v)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
