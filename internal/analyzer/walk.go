package analyzer

import "go/ast"

// scope says which rules consider the nodes of a subtree (plan 4.6).
type scope uint8

const (
	// outside covers package-level declarations, function signatures,
	// type declarations, and imports. Stmt rules skip it.
	outside scope = iota
	// body covers function bodies, including the bodies of function
	// literals. Stmt rules apply.
	body
)

// visitor is called for every node in depth-first order with the scope of
// the node and whether the node is the body block of a FuncDecl or FuncLit
// (which stmt rules skip). Returning false skips the children of n.
type visitor func(n ast.Node, sc scope, bodyBlock bool) bool

// walk visits the syntax tree of file. The file must have passed deepNode,
// so the recursion stays within the depth limit.
func walk(file *ast.File, visit visitor) {
	var stack []ast.Node
	var scopes []scope
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			scopes = scopes[:len(scopes)-1]
			return false
		}
		sc, bodyBlock := outside, false
		if len(stack) > 0 {
			sc, bodyBlock = childScope(stack[len(stack)-1], scopes[len(scopes)-1], n)
		}
		if !visit(n, sc, bodyBlock) {
			return false
		}
		stack = append(stack, n)
		scopes = append(scopes, sc)
		return true
	})
}

// childScope returns the scope of n, a child of parent whose scope is ps.
func childScope(parent ast.Node, ps scope, n ast.Node) (sc scope, bodyBlock bool) {
	switch p := parent.(type) {
	case *ast.FuncDecl:
		if n == p.Body {
			return body, true
		}
		return outside, false
	case *ast.FuncLit:
		// A function literal starts a body wherever it appears.
		if n == p.Body {
			return body, true
		}
		return outside, false
	case *ast.TypeSpec:
		return outside, false
	}
	return ps, false
}

// isStmtCandidate reports whether stmt rules consider n.
func isStmtCandidate(n ast.Node, sc scope, bodyBlock bool) bool {
	_, ok := n.(ast.Stmt)
	return ok && sc == body && !bodyBlock
}
