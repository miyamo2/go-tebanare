package astmatch

import (
	"go/ast"
	"strings"
)

// Sentinel identifier names. Each contains '$', which cannot occur in a Go
// identifier, so a sentinel never collides with a name from source code.
const (
	// AnyName matches any single type except a variadic "...T".
	AnyName = "$_"
	// SeqName matches zero or more elements of a list.
	SeqName = "$..."
	// TParamPrefix starts a reference to a pattern type parameter:
	// "$tp:T" refers to the type parameter T declared by the pattern.
	TParamPrefix = "$tp:"
)

// Any returns a new sentinel that matches any single type.
func Any() *ast.Ident { return &ast.Ident{Name: AnyName} }

// Seq returns a new sentinel that matches zero or more list elements.
func Seq() *ast.Ident { return &ast.Ident{Name: SeqName} }

// TParam returns a new reference to the pattern type parameter name.
func TParam(name string) *ast.Ident { return &ast.Ident{Name: TParamPrefix + name} }

// IsAny reports whether e is an Any sentinel.
func IsAny(e ast.Expr) bool { return identName(e) == AnyName }

// IsSeq reports whether e is a Seq sentinel.
func IsSeq(e ast.Expr) bool { return identName(e) == SeqName }

// TParamName returns the name of the pattern type parameter that e refers
// to. ok is false when e was not built by TParam.
func TParamName(e ast.Expr) (name string, ok bool) {
	return strings.CutPrefix(identName(e), TParamPrefix)
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok && id != nil {
		return id.Name
	}
	return ""
}
