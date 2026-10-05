// Package schemagen generates the JSON Schema of the configuration file,
// schema/gotebanare.schema.json, from the preset declarations.
package schemagen

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	tebanare "github.com/miyamo2/go-tebanare"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
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
	MinLength            *int      `json:"minLength,omitempty"`
	Pattern              string    `json:"pattern,omitempty"`
	Items                *schema   `json:"items,omitempty"`
	MinItems             *int      `json:"minItems,omitempty"`
	Required             []string  `json:"required,omitempty"`
	Properties           props     `json:"properties,omitempty"`
	AdditionalProperties *bool     `json:"additionalProperties,omitempty"`
	MinProperties        *int      `json:"minProperties,omitempty"`
	MaxProperties        *int      `json:"maxProperties,omitempty"`
	AnyOf                []*schema `json:"anyOf,omitempty"`
	OneOf                []*schema `json:"oneOf,omitempty"`
	If                   *schema   `json:"if,omitempty"`
	Then                 *schema   `json:"then,omitempty"`
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

	defs := props{
		{"rule", ruleSchema(globs)},
		{"stmtRule", nodeSchema(result.TargetStmt)},
		{"exprRule", nodeSchema(result.TargetExpr)},
		{"stmtKind", (&schema{Type: "string", Enum: anys(rule.StmtKinds)}).describe("Name of a `go/ast` statement type.")},
		{"exprKind", (&schema{Type: "string", Enum: anys(rule.ExprKinds)}).describe("Name of a `go/ast` expression type.")},
	}
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
			{"rules", (&schema{
				Type:  nullable("array"),
				Items: &schema{Ref: "#/definitions/rule"},
			}).describe("User rules. Each rule hides the function declarations (`func`), statements (`stmt`), or expressions (`expr`) that it matches.")},
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

// ruleSchema returns the schema of one entry of `rules`. globs builds the
// schema of a list of globs.
func ruleSchema(globs func(string) *schema) *schema {
	pattern := &schema{Type: "string", MinLength: ptr(1)}
	return (&schema{
		Type:                 "object",
		Required:             []string{"id"},
		AdditionalProperties: ptr(false),
		Properties: props{
			{"id", (&schema{Type: "string", MinLength: ptr(1)}).describe("Unique id of the rule, shown in the UI and in diagnostics. It must differ from the name of every enabled preset.")},
			{"description", (&schema{Type: nullable("string")}).describe("Text shown in the tooltip of the code that the rule hides.")},
			{"func", (&schema{
				AnyOf: []*schema{
					pattern,
					{Type: "array", Items: pattern, MinItems: ptr(1)},
					{Type: "null"},
				},
			}).describe("Signature pattern of the functions and methods to hide, such as `func (_) String() string`. A list matches when any of its patterns matches.")},
			{"stmt", &schema{Ref: "#/definitions/stmtRule"}},
			{"expr", &schema{Ref: "#/definitions/exprRule"}},
			{"paths", globs("Globs that limit the files this rule applies to, on top of `files.include` and `files.exclude`.")},
			{"exclude_paths", globs("Globs of files this rule skips, on top of `files.exclude`.")},
			{"include_doc", (&schema{Type: nullable("boolean"), Default: true}).describe("Hide the doc comment together with the function. Only `func` rules have this key.")},
			{"enabled", (&schema{Type: nullable("boolean"), Default: true}).describe("Set to `false` to turn the rule off. A rule that is off is still checked for errors.")},
		},
		// A key set to null counts as unset, so these checks also look at
		// the type of the value.
		OneOf: []*schema{present("func", "string", "array"), present("stmt", "object"), present("expr", "object")},
		If:    present("include_doc", "boolean"),
		Then:  present("func", "string", "array"),
	}).describe("A user rule. It has an `id` and exactly one of `func`, `stmt`, and `expr` set to a value other than null.")
}

// nodeSchema returns the schema of the stmt or expr key of a rule.
func nodeSchema(target result.Target) *schema {
	kinds, defaults, kindDef := rule.StmtKinds, rule.DefaultStmtKinds, "#/definitions/stmtKind"
	desc := "Statement rule: hides the statements whose normalized text matches `regex` and no `not_regex`."
	kindDesc := "`go/ast` statement types to consider, such as `IfStmt` or `AssignStmt`."
	allKinds := "every statement type"
	if target == result.TargetExpr {
		kinds, defaults, kindDef = rule.ExprKinds, rule.DefaultExprKinds, "#/definitions/exprKind"
		desc = "Expression rule: hides the expressions whose normalized text matches `regex` and no `not_regex`."
		kindDesc = "`go/ast` expression types to consider, such as `CallExpr` or `CompositeLit`."
		allKinds = "every expression type"
	}
	var left []string
	for _, k := range kinds {
		if !slices.Contains(defaults, k) {
			left = append(left, "`"+k+"`")
		}
	}
	if len(left) > 0 {
		kindDesc += " Default: " + allKinds + " except " + strings.Join(left, " and ") + "."
	}
	str := &schema{Type: "string"}

	ps := props{
		{"kind", (&schema{
			AnyOf: []*schema{
				{Ref: kindDef},
				{Type: "array", Items: &schema{Ref: kindDef}},
				{Type: "null"},
			},
		}).describe(kindDesc)},
		{"regex", (&schema{
			AnyOf: []*schema{str, {Type: "array", Items: str, MinItems: ptr(1)}},
		}).describe("RE2 regular expression searched for in the normalized text of the node: its `go/printer` output without comments, with every run of whitespace replaced by one space. A list matches when any of its expressions matches. An expression gives a warning unless it starts with `^` or `\\A` or ends with `$` or `\\z`. In an alternation, every branch needs an anchor at the same end.")},
		{"not_regex", (&schema{
			AnyOf: []*schema{str, {Type: "array", Items: str}, {Type: "null"}},
		}).describe("RE2 regular expression, or a list of them. A node whose normalized text matches any of them stays visible.")},
	}
	if target == result.TargetExpr {
		ps = append(ps, prop{"hide", (&schema{
			Type:    nullable("string"),
			Enum:    []any{"self", "statement", nil},
			Default: "self",
		}).describe("The lines to hide. `self` hides the lines of the expression when no other code is on them. `statement` hides the innermost statement around the expression when it is a simple statement whose other parts are identifiers, `_`, or literals.")})
	}
	ps = append(ps, prop{"include_leading_comments", (&schema{Type: nullable("boolean"), Default: false}).describe("Also hide the comment lines right above the node, when no blank line comes between them.")})

	return (&schema{
		Type:                 nullable("object"),
		Required:             []string{"regex"},
		AdditionalProperties: ptr(false),
		Properties:           ps,
	}).describe(desc)
}

// present returns a schema that requires key to be set to a value of one
// of types. A key set to null counts as unset.
func present(key string, types ...string) *schema {
	return &schema{Required: []string{key}, Properties: props{{key, &schema{Type: types}}}}
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
