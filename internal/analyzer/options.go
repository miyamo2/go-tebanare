package analyzer

import "github.com/miyamo2/go-tebanare/internal/canon"

// Options holds the safety limits of an analysis. A zero or negative field
// means the default from DefaultOptions.
type Options struct {
	// MaxFileSize is the largest source, in bytes, that is analyzed.
	MaxFileSize int
	// MaxBracketDepth limits the nesting of (, [, and { counted together.
	MaxBracketDepth int
	// MaxElseIfChain limits the length of else-if chains, summed over the
	// enclosing chains.
	MaxElseIfChain int
	// MaxASTDepth limits the depth of the syntax tree. The *ast.File node
	// has depth 1. Before parsing, the limit also applies to the chains
	// of prefix operators, type constructors, and labels that the parser
	// handles by recursion.
	MaxASTDepth int
	// MaxNodeSize is the largest node, in source bytes, whose normalized
	// text stmt and expr rules see.
	MaxNodeSize int
}

// Default limits.
const (
	DefaultMaxFileSize     = 1 << 20
	DefaultMaxBracketDepth = 200
	DefaultMaxElseIfChain  = 1000
	DefaultMaxASTDepth     = 1500
)

// DefaultOptions returns the default limits.
func DefaultOptions() Options {
	return Options{
		MaxFileSize:     DefaultMaxFileSize,
		MaxBracketDepth: DefaultMaxBracketDepth,
		MaxElseIfChain:  DefaultMaxElseIfChain,
		MaxASTDepth:     DefaultMaxASTDepth,
		MaxNodeSize:     canon.DefaultMaxNodeSize,
	}
}

// withDefaults returns o with every unset limit replaced by its default.
func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.MaxFileSize <= 0 {
		o.MaxFileSize = d.MaxFileSize
	}
	if o.MaxBracketDepth <= 0 {
		o.MaxBracketDepth = d.MaxBracketDepth
	}
	if o.MaxElseIfChain <= 0 {
		o.MaxElseIfChain = d.MaxElseIfChain
	}
	if o.MaxASTDepth <= 0 {
		o.MaxASTDepth = d.MaxASTDepth
	}
	if o.MaxNodeSize <= 0 {
		o.MaxNodeSize = d.MaxNodeSize
	}
	return o
}
