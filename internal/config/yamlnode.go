package config

import (
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// resolve follows document and alias nodes to the node that holds the
// value. It returns nil for a missing node and an empty document.
func resolve(n *yaml.Node) *yaml.Node {
	// An alias never points at another alias, so a few steps are enough;
	// the bound guards against malformed trees.
	for i := 0; n != nil && i < 8; i++ {
		switch {
		case n.Kind == yaml.DocumentNode && len(n.Content) == 0:
			return nil
		case n.Kind == yaml.DocumentNode:
			n = n.Content[0]
		case n.Kind == yaml.AliasNode && n.Alias != nil:
			n = n.Alias
		default:
			return n
		}
	}
	return n
}

func isMapping(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.MappingNode && n.ShortTag() == "!!map"
}

func isString(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.ScalarNode && effectiveTag(n) == "!!str"
}

func isSequence(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.SequenceNode && n.ShortTag() == "!!seq"
}

func isNull(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.ScalarNode && effectiveTag(n) == "!!null"
}

// effectiveTag returns the tag of n. On a scalar, an explicit tag counts
// only when it is !!str or agrees with the tag the plain value resolves
// to, so "!!int abc" gives "!!int?" and fails every type check.
func effectiveTag(n *yaml.Node) string {
	if n.Kind != yaml.ScalarNode {
		return n.ShortTag()
	}
	implicit := (&yaml.Node{Kind: yaml.ScalarNode, Style: n.Style &^ yaml.TaggedStyle, Value: n.Value}).ShortTag()
	if n.Style&yaml.TaggedStyle == 0 {
		return implicit
	}
	explicit := n.ShortTag()
	if explicit == "!!str" || explicit == implicit {
		return explicit
	}
	return explicit + "?"
}

// describe names the kind and value of n for error messages.
func describe(n *yaml.Node) string {
	if n == nil {
		return "nothing"
	}
	switch n.Kind {
	case yaml.MappingNode:
		return withTag("a mapping", n, "!!map")
	case yaml.SequenceNode:
		return withTag("a list", n, "!!seq")
	case yaml.ScalarNode:
	default:
		return "an unsupported node"
	}
	value := n.Value
	if r := []rune(value); len(r) > 40 {
		value = string(r[:40]) + "..."
	}
	switch tag := effectiveTag(n); tag {
	case "!!null":
		return "null"
	case "!!str":
		return "string " + strconv.Quote(value)
	case "!!bool":
		return "boolean " + value
	case "!!int":
		return "integer " + value
	case "!!float":
		return "float " + value
	default:
		return strings.TrimSuffix(tag, "?") + " " + strconv.Quote(value)
	}
}

// withTag appends the tag of the collection n to kind when it differs
// from def, the tag of an untagged node of that kind.
func withTag(kind string, n *yaml.Node, def string) string {
	if tag := n.ShortTag(); tag != def {
		return kind + " tagged " + tag
	}
	return kind
}
