package config

import (
	"errors"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/presets"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// enabledPresets maps each preset listed under `presets` to the line of
// its name.
type enabledPresets map[string]int

// presets decodes the `presets` list. It returns the rules of the valid
// entries in list order.
func (c *compiler) presets(e entry) []*rule.Rule {
	enabled := enabledPresets{}
	items, ok := c.sequence(e.value, e.field)
	if !ok {
		return nil
	}
	var rules []*rule.Rule
	for i, raw := range items {
		if r := c.preset(raw, e.field.index(i), enabled); r != nil {
			rules = append(rules, r)
		}
	}
	return rules
}

// preset decodes one entry of `presets`: a name, or a mapping with one
// key, the name, whose value holds the settings.
func (c *compiler) preset(raw *yaml.Node, f field, enabled enabledPresets) *rule.Rule {
	n := resolve(raw)
	nameNode, settings := raw, (*yaml.Node)(nil)
	switch {
	case isString(n):
		// A name alone enables the preset with its defaults.
	case isMapping(n) && len(n.Content) == 2 && isString(resolve(n.Content[0])):
		nameNode, settings = n.Content[0], n.Content[1]
	case isMapping(n) && len(n.Content) == 2:
		c.errorf(n.Content[0], f, "expected a preset name as the key, found %s", describe(resolve(n.Content[0])))
		return nil
	case isMapping(n):
		c.errorf(raw, f, "expected a mapping with one key, the preset name, found %d keys", len(n.Content)/2)
		return nil
	default:
		c.errorf(raw, f, "expected a preset name or a mapping from a preset name to its settings, found %s", describe(n))
		return nil
	}

	name := resolve(nameNode).Value
	f = f.named(name)
	p, ok := presets.Lookup(name)
	if !ok {
		c.errorf(nameNode, f, "unknown preset %q (available presets: %s)", name, strings.Join(presets.Names(), ", "))
		return nil
	}
	c.ruleID = name
	defer func() { c.ruleID = "" }()
	if line, dup := enabled[name]; dup {
		c.errorf(nameNode, f, "preset %q is already enabled on line %d", name, line)
		return nil
	}
	enabled[name] = nameNode.Line

	s, err := presets.Decode(p, settings)
	if err == nil {
		var r *rule.Rule
		if r, err = p.Compile(s); err == nil {
			return r
		}
	}
	at := settings
	if at == nil {
		at = nameNode
	}
	c.presetErrors(err, at, f)
	return nil
}

// presetErrors reports the errors of presets.Decode and Preset.Compile.
// Errors without a position are reported at the position of at.
func (c *compiler) presetErrors(err error, at *yaml.Node, f field) {
	for _, e := range unwrapAll(err) {
		var de *presets.DecodeError
		if !errors.As(e, &de) {
			c.errorf(at, f, "%s", e.Error())
			continue
		}
		ef := f
		if de.Field != "" {
			ef = f.key(de.Field)
		}
		pos := at
		if de.Line > 0 {
			pos = &yaml.Node{Line: de.Line, Column: de.Column}
		}
		c.errorf(pos, ef, "%s", de.Msg)
	}
}
