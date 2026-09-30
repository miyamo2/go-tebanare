package config

import (
	"errors"
	"fmt"

	"github.com/miyamo2/go-tebanare/internal/nodematch"
	"github.com/miyamo2/go-tebanare/internal/result"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// Keys of the `stmt` and `expr` mappings.
var (
	stmtKeys = []string{"kind", "regex", "not_regex", "include_leading_comments"}
	exprKeys = []string{"kind", "regex", "not_regex", "hide", "include_leading_comments"}
)

// nodeRule compiles the `stmt` or `expr` value of a rule into r, whose
// Target is already set. It reports false after any error.
func (c *compiler) nodeRule(e entry, r *rule.Rule) bool {
	keys := stmtKeys
	if r.Target == result.TargetExpr {
		keys = exprKeys
	}
	start := len(c.errs)
	es, ok := c.mapping(e.value, e.field, keys...)
	if !ok {
		return false
	}

	lists := map[string][]item{}
	listOK := map[string]bool{}
	for _, key := range []string{"kind", "regex", "not_regex"} {
		if le, ok := es.get(key); ok {
			lists[key], listOK[key] = c.stringList(le, true)
		}
	}
	regexEntry, hasRegex := es.get("regex")
	if !hasRegex {
		c.errorf(e.value, e.field, "missing required key %q", "regex")
	}

	m, err := nodematch.New(r.Target, values(lists["kind"]), values(lists["regex"]), values(lists["not_regex"]))
	badRegex := map[int]bool{}
	if err != nil {
		for _, err := range unwrapAll(err) {
			var ne *nodematch.Error
			if !errors.As(err, &ne) {
				c.errorf(e.value, e.field, "%s", err.Error())
				continue
			}
			items := lists[ne.Key]
			switch {
			case ne.Index >= 0 && ne.Index < len(items):
				if ne.Key == "regex" {
					badRegex[ne.Index] = true
				}
				c.errorf(items[ne.Index].node, items[ne.Index].field, "%s", ne.Msg)
			case ne.Key == "regex" && (!hasRegex || !listOK["regex"]):
				// The missing key or its bad items are already reported.
			case ne.Key == "regex":
				c.errorf(regexEntry.value, regexEntry.field, "%s", ne.Msg)
			default:
				c.errorf(e.value, e.field.key(ne.Key), "%s", ne.Msg)
			}
		}
	}
	for i, it := range lists["regex"] {
		if !badRegex[i] && !nodematch.IsAnchored(it.value) {
			c.warnUnanchored(it.node, it.field, fmt.Sprintf(
				"regex %q is not anchored with ^ or $, so it can match any part of the normalized text", it.value))
		}
	}

	if he, ok := es.get("hide"); ok {
		if v, ok := c.str(he); ok {
			switch v {
			case "self":
				r.Hide = rule.HideSelf
			case "statement":
				r.Hide = rule.HideStatement
			default:
				c.errorf(he.value, he.field, "unknown value %q (valid values: self, statement)", v)
			}
		}
	}
	if ce, ok := es.get("include_leading_comments"); ok {
		r.IncludeLeadingComments, _ = c.boolean(ce)
	}
	if len(c.errs) > start {
		return false
	}
	r.Node = m
	return true
}

func values(items []item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.value
	}
	return out
}
