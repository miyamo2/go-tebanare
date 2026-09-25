package analyzer

import "go/ast"

// scope says which rules consider the nodes of a subtree (plan 4.6).
type scope uint8

const (
	// outside covers package-level declarations, function signatures,
	// type declarations, and imports. Stmt and expr rules skip it.
	outside scope = iota
	// body covers function bodies, including the bodies of function
	// literals. Stmt and expr rules apply.
	body
	// value covers the initializers of package-level var and const
	// declarations. Expr rules apply.
	value
)

// visitor is called for every node in depth-first order with the scope of
// the node, whether the node is the body block of a FuncDecl or FuncLit
// (which stmt rules skip), and the ancestors of the node, outermost first.
// The ancestors slice is only valid during the call. Returning false skips
// the children of n.
type visitor func(n ast.Node, sc scope, bodyBlock bool, ancestors []ast.Node) bool

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
		if !visit(n, sc, bodyBlock, stack) {
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
	case *ast.ValueSpec:
		if ps != outside {
			break
		}
		// A package-level var or const: only the values are in scope.
		for _, v := range p.Values {
			if n == v {
				return value, false
			}
		}
		return outside, false
	}
	return ps, false
}

// isStmtCandidate reports whether stmt rules consider n.
func isStmtCandidate(n ast.Node, sc scope, bodyBlock bool) bool {
	_, ok := n.(ast.Stmt)
	return ok && sc == body && !bodyBlock
}

// isExprCandidate reports whether expr rules consider n.
func isExprCandidate(n ast.Node, sc scope) bool {
	_, ok := n.(ast.Expr)
	return ok && (sc == body || sc == value)
}

// innermostStmt returns the innermost statement among ancestors, or nil.
func innermostStmt(ancestors []ast.Node) ast.Stmt {
	for i := len(ancestors) - 1; i >= 0; i-- {
		if s, ok := ancestors[i].(ast.Stmt); ok {
			return s
		}
	}
	return nil
}
