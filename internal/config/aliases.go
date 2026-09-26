package config

import "go.yaml.in/yaml/v3"

// maxAliasCopy bounds the content that aliases copy into a config. Each
// copied node counts one, and a copied scalar adds the length of its
// value, so the measure is close to the length of the YAML text the
// aliases stand for.
//
// yaml.v3 applies no such bound when it decodes into yaml.Node, and the
// compiler compiles the patterns under an alias again at every use. With
// a list of N aliases to a list of M aliases, it compiles N*M patterns;
// a valid 23 KB config of that shape would take a minute and 10 GB.
const maxAliasCopy = 64 << 10

// aliasSizer measures the size of nodes with their aliases expanded,
// capped at maxAliasCopy+1.
type aliasSizer struct {
	// sizes holds the size of each anchored node measured so far.
	sizes map[*yaml.Node]int
	// open holds the anchored nodes being measured.
	open map[*yaml.Node]bool
	// cycle is the first alias found inside the value it refers to.
	cycle *yaml.Node
	// copied is the size of the values of the aliases visited so far.
	copied int
}

// checkAliases reports a config whose aliases copy more than maxAliasCopy,
// or that has an alias inside the value it refers to. It returns false
// after such an error; the compiler then stops, so this error is the only
// one.
func (c *compiler) checkAliases(root *yaml.Node) bool {
	a := &aliasSizer{sizes: map[*yaml.Node]int{}, open: map[*yaml.Node]bool{}}
	stop := a.visit(root)
	switch {
	case stop == nil:
		return true
	case a.cycle != nil:
		c.errorf(a.cycle, "", "alias *%s refers to a value that contains it", a.cycle.Value)
	default:
		c.errorf(stop, "", "aliases expand the config by more than %d KiB", maxAliasCopy>>10)
	}
	return false
}

// visit walks the tree under n as written, in document order, and adds
// the size of the value of each alias to a.copied. It stops at the first
// alias that takes a.copied past maxAliasCopy or closes a cycle, and
// returns that alias.
func (a *aliasSizer) visit(n *yaml.Node) (stop *yaml.Node) {
	if n.Kind == yaml.AliasNode {
		if a.copied += a.size(n); a.cycle != nil || a.copied > maxAliasCopy {
			return n
		}
		return nil
	}
	for _, child := range n.Content {
		if stop := a.visit(child); stop != nil {
			return stop
		}
	}
	return nil
}

// size returns the size of n with its aliases expanded. Once a cycle is
// found, the check fails whatever the sizes are, so it stops measuring:
// otherwise an anchor under an open one is measured again at each level of
// nested anchors that refer to each other, and the work and the stack grow
// with the square of their depth.
func (a *aliasSizer) size(n *yaml.Node) int {
	if a.cycle != nil {
		return maxAliasCopy + 1
	}
	switch {
	case n.Kind == yaml.AliasNode && n.Alias == nil:
		return 1
	case n.Kind == yaml.AliasNode && a.open[n.Alias]:
		if a.cycle == nil {
			a.cycle = n
		}
		return maxAliasCopy + 1
	case n.Kind == yaml.AliasNode:
		return a.anchored(n.Alias)
	case n.Anchor != "":
		return a.anchored(n)
	}
	return a.content(n)
}

// anchored returns the size of n, the value of an anchor, and measures
// each such node once, so the work stays linear in the number of nodes.
func (a *aliasSizer) anchored(n *yaml.Node) int {
	if s, ok := a.sizes[n]; ok {
		return s
	}
	a.open[n] = true
	s := a.content(n)
	delete(a.open, n)
	a.sizes[n] = s
	return s
}

// content returns the size of n and its children.
func (a *aliasSizer) content(n *yaml.Node) int {
	s := min(1+len(n.Value), maxAliasCopy+1)
	for _, child := range n.Content {
		s = min(s+a.size(child), maxAliasCopy+1)
	}
	return s
}
