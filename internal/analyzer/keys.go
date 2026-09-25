package analyzer

import (
	"fmt"
	"go/ast"

	"github.com/miyamo2/go-tebanare/internal/canon"
	"github.com/miyamo2/go-tebanare/internal/rule"
)

// pairingKeys returns the pairing key of every top-level function
// declaration (plan 4.8): "Recv.Name" for a method, where Recv is the
// base type name of the receiver, and "Name" for a function. Functions
// named init and declarations named _ get "#n" appended, where n counts
// the earlier declarations with the same key.
func pairingKeys(file *ast.File) map[*ast.FuncDecl]string {
	keys := map[*ast.FuncDecl]string{}
	seen := map[string]int{}
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		key := baseKey(fd)
		if name := fd.Name.Name; name == "_" || name == "init" && fd.Recv == nil {
			n := seen[key]
			seen[key]++
			key = fmt.Sprintf("%s#%d", key, n)
		}
		keys[fd] = key
	}
	return keys
}

// baseKey returns "Recv.Name" for a method and "Name" for a function.
// A receiver that is not a (pointer to a) named type is printed in
// parentheses.
func baseKey(fd *ast.FuncDecl) string {
	if fd.Recv == nil {
		return fd.Name.Name
	}
	if base, _, ok := rule.RecvBase(fd); ok {
		return base + "." + fd.Name.Name
	}
	recv := ""
	if len(fd.Recv.List) > 0 {
		recv = canon.Normalize(rule.StripParens(fd.Recv.List[0].Type))
	}
	return "(" + recv + ")." + fd.Name.Name
}
