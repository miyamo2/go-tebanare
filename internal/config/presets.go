package config

import (
	"errors"
	"strconv"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/presets"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// presets compiles the items of `presets`, which passed the schema: each
// is a preset name, or a mapping from the name to its settings. It returns
// the rules of the valid items in list order.
func (c *compiler) presets(items []any) []*rule.Rule {
	// enabled maps each preset listed so far to the line of its name.
	enabled := map[string]int{}
	var rules []*rule.Rule
	for i, item := range items {
		if r := c.preset(i, item, enabled); r != nil {
			rules = append(rules, r)
		}
	}
	return rules
}

func (c *compiler) preset(i int, item any, enabled map[string]int) *rule.Rule {
	itemPath := []string{"presets", strconv.Itoa(i)}
	var name string
	var settings any
	settingsPath := itemPath
	switch v := item.(type) {
	case string:
		name = v
	case map[string]any:
		for k, s := range v {
			name, settings = k, s
		}
		settingsPath = append(itemPath, name)
	}
	p, ok := presets.Lookup(name)
	if !ok {
		// The schema lists the preset names; a preset without code is a
		// bug.
		c.errorf(c.nodeAt(itemPath), c.fieldOf(itemPath), "preset %q is in the schema but not built in", name)
		return nil
	}
	nameNode := c.nodeAt(itemPath)
	if settingsPath[len(settingsPath)-1] == name {
		nameNode = keyNode(nameNode, name)
	}
	c.ruleID = name
	defer func() { c.ruleID = "" }()
	if line, dup := enabled[name]; dup {
		c.errorf(nameNode, c.fieldOf(itemPath), "preset %q is already enabled on line %d", name, line)
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
	c.presetErrors(err, settingsPath)
	return nil
}

// presetErrors reports the errors of presets.Decode and Preset.Compile for
// the settings at path. A *presets.DecodeError names a setting such as
// "paths[1]", which locates the error.
func (c *compiler) presetErrors(err error, path []string) {
	for _, e := range unwrapAll(err) {
		var de *presets.DecodeError
		if !errors.As(e, &de) {
			c.errorf(c.nodeAt(path), c.fieldOf(path), "%s", e.Error())
			continue
		}
		at := append(path[:len(path):len(path)], settingPath(de.Field)...)
		c.errorf(c.nodeAt(at), c.fieldOf(at), "%s", de.Msg)
	}
}

// settingPath splits a setting field such as "paths[1]" into path
// segments.
func settingPath(f string) []string {
	if f == "" {
		return nil
	}
	name, idx, ok := strings.Cut(strings.TrimSuffix(f, "]"), "[")
	if !ok {
		return []string{f}
	}
	return []string{name, idx}
}
