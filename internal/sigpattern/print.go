package sigpattern

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"

	"github.com/miyamo2/go-tebanare/internal/astmatch"
)

// String returns the canonical form of the pattern. Parsing it gives a
// pattern with the same meaning and the same String. The canonical form
// drops receiver and parameter names, gives each type parameter its own
// constraint, and keeps parentheses as written.
func (p *Pattern) String() string {
	var b strings.Builder
	b.WriteString("func ")
	if r := p.recv; r != nil {
		b.WriteByte('(')
		if r.pointer {
			b.WriteByte('*')
		}
		switch {
		case r.anyType:
			b.WriteByte('_')
		case !r.pointer && !r.name.regexp && strings.Contains(strings.TrimSuffix(r.name.text, "*"), "*"):
			// "(h*Handler)" is an error (see receiverName), so the value
			// receiver glob h*Handler prints as "(_ h*Handler)".
			b.WriteString("_ ")
			writeName(&b, r.name)
		default:
			writeName(&b, r.name)
		}
		if r.args != nil {
			writeTypeParams(&b, r.args)
		}
		b.WriteString(") ")
	}
	writeName(&b, p.name)
	if p.tparams != nil {
		writeTypeParams(&b, p.tparams)
	}
	if p.sig != nil {
		writeSignature(&b, p.sig)
	}
	return b.String()
}

func writeName(b *strings.Builder, n name) {
	if !n.regexp {
		b.WriteString(n.text)
		return
	}
	b.WriteByte('/')
	b.WriteString(strings.ReplaceAll(n.text, "/", `\/`))
	b.WriteByte('/')
}

func writeTypeParams(b *strings.Builder, fl *ast.FieldList) {
	b.WriteByte('[')
	for i, f := range fields(fl) {
		if i > 0 {
			b.WriteString(", ")
		}
		for j, n := range f.Names {
			if j > 0 {
				b.WriteString(", ")
			}
			writeType(b, n)
		}
		if f.Type != nil {
			b.WriteByte(' ')
			writeType(b, f.Type)
		}
	}
	b.WriteByte(']')
}

func writeSignature(b *strings.Builder, ft *ast.FuncType) {
	writeList(b, ft.Params)
	results := expand(ft.Results)
	switch {
	case len(results) == 0:
	case len(results) == 1 && astmatch.IsSeq(results[0]):
		b.WriteString(" ...")
	case len(results) == 1 && !isParen(results[0]):
		b.WriteByte(' ')
		writeType(b, results[0])
	default:
		b.WriteByte(' ')
		writeList(b, ft.Results)
	}
}

func isParen(e ast.Expr) bool {
	_, ok := e.(*ast.ParenExpr)
	return ok
}

// writeList writes a parameter or result list without names.
func writeList(b *strings.Builder, fl *ast.FieldList) {
	b.WriteByte('(')
	for i, t := range expand(fl) {
		if i > 0 {
			b.WriteString(", ")
		}
		writeType(b, t)
	}
	b.WriteByte(')')
}

func fields(fl *ast.FieldList) []*ast.Field {
	if fl == nil {
		return nil
	}
	return fl.List
}

// expand returns one type per element, so "a, b int" gives two.
func expand(fl *ast.FieldList) []ast.Expr {
	var out []ast.Expr
	for _, f := range fields(fl) {
		for range max(len(f.Names), 1) {
			out = append(out, f.Type)
		}
	}
	return out
}

func writeType(b *strings.Builder, e ast.Expr) {
	switch x := e.(type) {
	case *ast.Ident:
		switch {
		case astmatch.IsAny(x):
			b.WriteByte('_')
		case astmatch.IsSeq(x):
			b.WriteString("...")
		default:
			if n, ok := astmatch.TParamName(x); ok {
				b.WriteString(n)
			} else {
				b.WriteString(x.Name)
			}
		}
	case *ast.SelectorExpr:
		writeType(b, x.X)
		b.WriteByte('.')
		b.WriteString(x.Sel.Name)
	case *ast.StarExpr:
		b.WriteByte('*')
		writeType(b, x.X)
	case *ast.ParenExpr:
		b.WriteByte('(')
		writeType(b, x.X)
		b.WriteByte(')')
	case *ast.ArrayType:
		b.WriteByte('[')
		writeLength(b, x.Len)
		b.WriteByte(']')
		writeType(b, x.Elt)
	case *ast.MapType:
		b.WriteString("map[")
		writeType(b, x.Key)
		b.WriteByte(']')
		writeType(b, x.Value)
	case *ast.ChanType:
		switch x.Dir {
		case ast.SEND:
			b.WriteString("chan<- ")
		case ast.RECV:
			b.WriteString("<-chan ")
		default:
			b.WriteString("chan ")
		}
		writeType(b, x.Value)
	case *ast.FuncType:
		b.WriteString("func")
		writeSignature(b, x)
	case *ast.InterfaceType:
		b.WriteString("interface{")
		elems := fields(x.Methods)
		for i, f := range elems {
			writeSep(b, i)
			if ft, ok := f.Type.(*ast.FuncType); ok && len(f.Names) > 0 {
				b.WriteString(f.Names[0].Name)
				writeSignature(b, ft)
			} else {
				writeType(b, f.Type)
			}
		}
		writeEnd(b, len(elems))
	case *ast.StructType:
		b.WriteString("struct{")
		elems := fields(x.Fields)
		for i, f := range elems {
			writeSep(b, i)
			writeField(b, f)
		}
		writeEnd(b, len(elems))
	case *ast.IndexExpr:
		writeType(b, x.X)
		b.WriteByte('[')
		writeType(b, x.Index)
		b.WriteByte(']')
	case *ast.IndexListExpr:
		writeType(b, x.X)
		b.WriteByte('[')
		for i, t := range x.Indices {
			if i > 0 {
				b.WriteString(", ")
			}
			writeType(b, t)
		}
		b.WriteByte(']')
	case *ast.UnaryExpr:
		b.WriteString(x.Op.String())
		writeType(b, x.X)
	case *ast.BinaryExpr:
		writeType(b, x.X)
		b.WriteString(" | ")
		writeType(b, x.Y)
	case *ast.Ellipsis:
		b.WriteString("...")
		writeType(b, x.Elt)
	}
}

func writeLength(b *strings.Builder, e ast.Expr) {
	switch {
	case e == nil:
	case astmatch.IsAny(e):
		b.WriteByte('_')
	default:
		var buf bytes.Buffer
		if printer.Fprint(&buf, token.NewFileSet(), e) == nil {
			b.Write(buf.Bytes())
		}
	}
}

func writeField(b *strings.Builder, f *ast.Field) {
	for i, n := range f.Names {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(n.Name)
	}
	if len(f.Names) > 0 {
		b.WriteByte(' ')
	}
	writeType(b, f.Type)
	if f.Tag != nil {
		b.WriteByte(' ')
		b.WriteString(f.Tag.Value)
	}
}

// writeSep and writeEnd lay out interface and struct bodies as
// "interface{}" or "interface{ A; B }".
func writeSep(b *strings.Builder, i int) {
	if i > 0 {
		b.WriteByte(';')
	}
	b.WriteByte(' ')
}

func writeEnd(b *strings.Builder, n int) {
	if n > 0 {
		b.WriteByte(' ')
	}
	b.WriteByte('}')
}
