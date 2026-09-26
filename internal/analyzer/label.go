package analyzer

import (
	"go/ast"
	"strings"
	"unicode/utf8"

	"github.com/miyamo2/go-tebanare/internal/canon"
)

// maxLabelRunes is the length at which nodeLabel cuts the normalized text.
const maxLabelRunes = 60

// funcLabel returns the label of a func hit: "func (<receiver type>) Name"
// for a method and "func Name" for a function, followed by the type
// parameter list of a generic function, as in "func Map[T, U any]". Types
// are printed by canon.Normalize.
func funcLabel(fd *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if fd.Recv != nil {
		b.WriteString("(")
		if len(fd.Recv.List) > 0 {
			b.WriteString(canon.Normalize(fd.Recv.List[0].Type))
		}
		b.WriteString(") ")
	}
	b.WriteString(fd.Name.Name)
	if tp := fd.Type.TypeParams; tp != nil && len(tp.List) > 0 {
		b.WriteString("[")
		for i, field := range tp.List {
			if i > 0 {
				b.WriteString(", ")
			}
			for j, name := range field.Names {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(name.Name)
			}
			b.WriteString(" ")
			b.WriteString(canon.Normalize(field.Type))
		}
		b.WriteString("]")
	}
	return b.String()
}

// nodeLabel returns the label of a stmt hit: the normalized text
// of the node, cut to 60 runes followed by "..." when it is longer.
func nodeLabel(text string) string {
	if utf8.RuneCountInString(text) <= maxLabelRunes {
		return text
	}
	n := 0
	for i := range text {
		if n == maxLabelRunes {
			return text[:i] + "..."
		}
		n++
	}
	return text
}
