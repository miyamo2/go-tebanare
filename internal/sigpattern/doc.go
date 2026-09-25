// Package sigpattern parses and matches signature patterns, which select
// function and method declarations by receiver, name, type parameters, and
// signature:
//
//	func (*Repository[T]) Save(context.Context, T) error
//	func (_) String() string
//	func New*
//
// A pattern looks like a Go function declaration and adds wildcards: "_"
// is any single type, a lone "..." is zero or more list elements, and
// names may be globs ("Find*", "Get?") or regular expressions ("/^Get/").
// Type parameters declared by the pattern bind to the source type
// parameters in the same position, whatever their names.
//
// Matching is syntactic. It uses only the parsed declaration, so package
// qualifiers must be spelled as in the source, and import aliases, dot
// imports, and type aliases are not resolved.
package sigpattern
