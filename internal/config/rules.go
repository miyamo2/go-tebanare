package config

import (
	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// ruleKeys lists the keys of a `rules` entry.
var ruleKeys = []string{"id", "description", "func", "stmt", "expr", "paths", "exclude_paths", "include_doc", "enabled"}

// rules decodes the `rules` list. It returns the valid enabled rules in
// list order. enabled holds the presets the config enables; no rule may
// use one of their names as its id.
func (c *compiler) rules(e entry, enabled enabledPresets) []*rule.Rule {
	items, ok := c.sequence(e.value, e.field)
	if !ok {
		return nil
	}
	ids := map[string]int{}
	var out []*rule.Rule
	for i, raw := range items {
		if r := c.userRule(raw, e.field.index(i), ids, enabled); r != nil {
			out = append(out, r)
		}
	}
	return out
}

// userRule decodes one entry of `rules`. ids maps each id seen so far to
// the line where it is set. It returns nil when the entry is invalid or
// disabled. A disabled rule is still checked for errors, but its warnings
// are dropped because it hides nothing.
func (c *compiler) userRule(raw *yaml.Node, f field, ids map[string]int, enabled enabledPresets) *rule.Rule {
	id := peekID(raw)
	f = f.named(id)
	c.ruleID = id
	defer func() { c.ruleID = "" }()
	errs, warns := len(c.errs), len(c.warns)
	es, ok := c.mapping(raw, f, ruleKeys...)
	if !ok {
		return nil
	}

	r := &rule.Rule{ID: id}
	c.checkID(es, raw, f, ids, enabled)
	if de, ok := es.get("description"); ok {
		r.Description, _ = c.str(de)
	}
	if pe, ok := es.get("paths"); ok {
		r.Paths = c.globs(pe)
	}
	if xe, ok := es.get("exclude_paths"); ok {
		r.ExcludePaths = c.globs(xe)
	}
	on := true
	if ee, ok := es.get("enabled"); ok {
		on, _ = c.boolean(ee)
	}

	var target entry
	for _, e := range es {
		switch e.name {
		case "func", "stmt", "expr":
		default:
			continue
		}
		if target.name != "" {
			c.errorf(e.key, e.field, "a rule has exactly one of func, stmt, expr, and this rule already has %s", target.name)
			continue
		}
		target = e
	}
	switch target.name {
	case "":
		c.errorf(raw, f, "a rule needs one of func, stmt, expr")
	case "func":
		r.Target = result.TargetFunc
		r.IncludeDoc = true
		if ie, ok := es.get("include_doc"); ok {
			r.IncludeDoc, _ = c.boolean(ie)
		}
		r.Func, _ = c.funcRule(target)
	case "stmt", "expr":
		r.Target = result.Target(target.name)
		if ie, ok := es.get("include_doc"); ok {
			c.errorf(ie.key, ie.field, "include_doc applies to func rules only")
		}
		c.nodeRule(target, r)
	}

	if !on {
		c.warns = c.warns[:warns]
	}
	if len(c.errs) > errs || !on {
		return nil
	}
	return r
}

// checkID reports a missing, empty, or duplicate id, and an id equal to
// the name of an enabled preset.
func (c *compiler) checkID(es entries, raw *yaml.Node, f field, ids map[string]int, enabled enabledPresets) {
	ie, ok := es.get("id")
	if !ok {
		c.errorf(raw, f, "missing required key %q", "id")
		return
	}
	id, ok := c.str(ie)
	if !ok {
		return
	}
	if id == "" {
		c.errorf(ie.value, ie.field, "id must not be empty")
		return
	}
	if line, dup := ids[id]; dup {
		c.errorf(ie.value, ie.field, "duplicate id %q (first set on line %d)", id, line)
	} else {
		ids[id] = ie.value.Line
	}
	if line, on := enabled[id]; on {
		c.errorf(ie.value, ie.field, "id %q is the name of the preset enabled on line %d", id, line)
	}
}

// peekID returns the id of a rule entry when it is a string, so that
// errors in the entry can name the rule. It returns "" otherwise.
func peekID(raw *yaml.Node) string {
	m := resolve(raw)
	if !isMapping(m) {
		return ""
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := resolve(m.Content[i]); isString(k) && k.Value == "id" {
			if v := resolve(m.Content[i+1]); isString(v) {
				return v.Value
			}
			return ""
		}
	}
	return ""
}
