// Package configschema holds the Go side of the configuration's JSON
// Schema, schema/gotebanare.schema.json, which is the source of truth for
// the keys, types, defaults, and constraints of the configuration.
//
// types_gen.go holds the Go types of a configuration, generated from the
// schema with quicktype (make generate). This file reads what the
// generated types do not carry from the embedded schema: the order and
// the descriptions of the preset settings, and the defaults, which
// Decode fills in before it decodes a value into a generated type.
package configschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/miyamo2/go-tebanare/schema"
)

// Setting describes one setting of a preset.
type Setting struct {
	Name string
	// Type is "bool", "int", "string", or "[]string".
	Type string
	// Default is the default as shown in docs, such as "true", "[err]", or
	// "none": the text of the sentence "Default: <text>." that ends the
	// setting's description.
	Default string
	// Description is the setting's description without that sentence.
	Description string
	// Enum lists the allowed values of a string setting.
	Enum []string
}

// PresetNames returns the names of the presets in the schema, in schema
// order.
func PresetNames() ([]string, error) {
	root, err := load()
	if err != nil {
		return nil, err
	}
	anyOf, _ := root.path("properties", "presets", "items").get("anyOf").([]any)
	for _, alt := range anyOf {
		if o, ok := alt.(*object); ok && o.get("type") == "string" {
			var names []string
			enum, _ := o.get("enum").([]any)
			for _, e := range enum {
				if s, ok := e.(string); ok {
					names = append(names, s)
				}
			}
			return names, nil
		}
	}
	return nil, errors.New("configschema: the schema lists no preset names")
}

// Keys returns the property names of the object at path in the schema,
// such as the top-level keys for an empty path or the keys of `files` for
// ["files"], in schema order.
func Keys(path ...string) ([]string, error) {
	o, err := load()
	if err != nil {
		return nil, err
	}
	for _, seg := range path {
		o = o.path("properties", seg)
	}
	props := o.obj("properties")
	if props == nil {
		return nil, fmt.Errorf("configschema: no properties at %q", path)
	}
	return props.keys, nil
}

// defaultSentence splits a setting description into its text and the
// default.
var defaultSentence = regexp.MustCompile(`^(.*) Default: (.+)\.$`)

// Settings describes the settings of preset in schema order:
// definitions/<preset>Settings.
func Settings(preset string) ([]Setting, error) {
	def, err := settingsSchema(preset)
	if err != nil {
		return nil, err
	}
	props := def.obj("properties")
	if props == nil {
		return nil, fmt.Errorf("configschema: %sSettings has no properties", preset)
	}
	var out []Setting
	for _, name := range props.keys {
		p := props.obj(name)
		desc, _ := p.get("description").(string)
		m := defaultSentence.FindStringSubmatch(desc)
		if m == nil {
			return nil, fmt.Errorf("configschema: %sSettings.%s: the description does not end with \"Default: <text>.\"", preset, name)
		}
		s := Setting{Name: name, Description: m[1], Default: m[2]}
		switch t := p.nonNullType(); t {
		case "boolean":
			s.Type = "bool"
		case "integer":
			s.Type = "int"
		case "string":
			s.Type = "string"
			enum, _ := p.get("enum").([]any)
			for _, e := range enum {
				if e, ok := e.(string); ok {
					s.Enum = append(s.Enum, e)
				}
			}
		case "array":
			if p.obj("items").get("type") != "string" {
				return nil, fmt.Errorf("configschema: %sSettings.%s: a list of something other than strings", preset, name)
			}
			s.Type = "[]string"
		default:
			return nil, fmt.Errorf("configschema: %sSettings.%s: unsupported type %q", preset, name, t)
		}
		out = append(out, s)
	}
	return out, nil
}

// DecodeSettings decodes settings, the settings of preset after they passed
// the schema (nil or a map[string]any), into out, a pointer to the
// preset's generated settings type. A missing or null setting gets the
// schema's default, if any.
func DecodeSettings(preset string, settings any, out any) error {
	def, err := settingsSchema(preset)
	if err != nil {
		return err
	}
	m := map[string]any{}
	switch v := settings.(type) {
	case nil:
	case map[string]any:
		for k, x := range v {
			m[k] = x
		}
	default:
		return fmt.Errorf("configschema: %s: settings have type %T, want a mapping", preset, settings)
	}
	if props := def.obj("properties"); props != nil {
		for _, name := range props.keys {
			if d, ok := props.obj(name).values["default"]; ok && m[name] == nil {
				m[name] = d
			}
		}
	}
	return roundTrip(m, out)
}

// DecodeConfig decodes v, a configuration that passed the schema, into a
// Config.
func DecodeConfig(v any) (*Config, error) {
	var c Config
	if err := roundTrip(v, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// roundTrip decodes the JSON value v into out through its JSON encoding,
// which the generated types read.
func roundTrip(v, out any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func settingsSchema(preset string) (*object, error) {
	root, err := load()
	if err != nil {
		return nil, err
	}
	def := root.path("definitions", preset+"Settings")
	if def == nil {
		return nil, fmt.Errorf("configschema: the schema has no settings for preset %q", preset)
	}
	return def, nil
}

var (
	loadOnce   sync.Once
	loadedRoot *object
	loadErr    error
)

// load parses the embedded schema once, keeping the order of the keys.
func load() (*object, error) {
	loadOnce.Do(func() {
		dec := json.NewDecoder(bytes.NewReader(schema.JSON))
		v, err := parseValue(dec)
		if err != nil {
			loadErr = fmt.Errorf("configschema: %w", err)
			return
		}
		root, ok := v.(*object)
		if !ok {
			loadErr = errors.New("configschema: the schema is not an object")
			return
		}
		loadedRoot = root
	})
	return loadedRoot, loadErr
}

// object is a JSON object that keeps the order of its keys.
type object struct {
	keys   []string
	values map[string]any
}

func (o *object) get(key string) any {
	if o == nil {
		return nil
	}
	return o.values[key]
}

func (o *object) obj(key string) *object {
	v, _ := o.get(key).(*object)
	return v
}

func (o *object) path(keys ...string) *object {
	for _, k := range keys {
		o = o.obj(k)
	}
	return o
}

// nonNullType returns the "type" of o other than "null".
func (o *object) nonNullType() string {
	switch t := o.get("type").(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if s, _ := v.(string); s != "null" {
				return s
			}
		}
	}
	return ""
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		o := &object{values: map[string]any{}}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			k, _ := kt.(string)
			v, err := parseValue(dec)
			if err != nil {
				return nil, err
			}
			o.keys = append(o.keys, k)
			o.values[k] = v
		}
		_, err := dec.Token()
		return o, err
	case json.Delim('['):
		l := []any{}
		for dec.More() {
			v, err := parseValue(dec)
			if err != nil {
				return nil, err
			}
			l = append(l, v)
		}
		_, err := dec.Token()
		return l, err
	}
	return tok, nil
}
