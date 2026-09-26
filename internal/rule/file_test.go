package rule

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

const fileSrc = `package p

import (
	ctx2 "context"
	_ "embed"
	. "fmt"
	yaml "gopkg.in/yaml.v3"
	rand "math/rand/v2"
)

// Doc for Name.
func (u *User) Name() string { return u.name } // trailing

func (u User) Age() int {
	/* inner */
	return u.age
}

func (c *Cache[K, V]) Get(k K) V { var v V; return v }

func (s (*Stack[T])) Len() int { return 0 }

func plain() {}
`

func parseFile(t *testing.T) *File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", fileSrc, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	return &File{Path: "p.go", Fset: fset, AST: f, Src: []byte(fileSrc)}
}

func funcNamed(f *File, name string) *ast.FuncDecl {
	for _, d := range f.AST.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

func TestRecvBase(t *testing.T) {
	f := parseFile(t)
	tests := []struct {
		fn      string
		name    string
		pointer bool
		ok      bool
	}{
		{"Name", "User", true, true},
		{"Age", "User", false, true},
		{"Get", "Cache", true, true},
		{"Len", "Stack", true, true},
		{"plain", "", false, false},
	}
	for _, tt := range tests {
		name, pointer, ok := RecvBase(funcNamed(f, tt.fn))
		if name != tt.name || pointer != tt.pointer || ok != tt.ok {
			t.Errorf("RecvBase(%s) = %q, %v, %v; want %q, %v, %v", tt.fn, name, pointer, ok, tt.name, tt.pointer, tt.ok)
		}
	}
}

func TestMethodsOf(t *testing.T) {
	f := parseFile(t)
	if got := f.MethodsOf("User"); !got["Name"] || !got["Age"] || len(got) != 2 {
		t.Errorf("MethodsOf(User) = %v", got)
	}
	if got := f.MethodsOf("Cache"); !got["Get"] || len(got) != 1 {
		t.Errorf("MethodsOf(Cache) = %v", got)
	}
	if got := f.MethodsOf("Missing"); len(got) != 0 {
		t.Errorf("MethodsOf(Missing) = %v", got)
	}
}

func TestHasCommentInLines(t *testing.T) {
	f := parseFile(t)
	name := funcNamed(f, "Name")
	line := f.Line(name.Pos())
	if !f.HasCommentInLines(line, line, name.Doc) {
		t.Error("trailing comment on the func line was not found")
	}
	if f.HasCommentInLines(line-1, line-1, name.Doc) {
		t.Error("doc comment was not skipped")
	}
	if !f.HasCommentInLines(line-1, line-1, nil) {
		t.Error("doc comment was not found without skip")
	}
	age := funcNamed(f, "Age")
	if !f.HasCommentInLines(f.Line(age.Pos()), f.Line(age.End()), age.Doc) {
		t.Error("block comment inside Age was not found")
	}
	get := funcNamed(f, "Get")
	if f.HasCommentInLines(f.Line(get.Pos()), f.Line(get.End()), nil) {
		t.Error("comment found in Get")
	}
}

func TestLineAndOffset(t *testing.T) {
	f := parseFile(t)
	fd := funcNamed(f, "plain")
	if got := f.Line(fd.Pos()); got != 23 {
		t.Errorf("Line(plain) = %d, want 23", got)
	}
	if off := f.Offset(fd.Pos()); string(f.Src[off:off+11]) != "func plain(" {
		t.Errorf("Offset(plain) points at %q", f.Src[off:off+11])
	}
}

func TestStripParens(t *testing.T) {
	e, err := parser.ParseExpr("((x))")
	if err != nil {
		t.Fatal(err)
	}
	if id, ok := StripParens(e).(*ast.Ident); !ok || id.Name != "x" {
		t.Errorf("StripParens = %#v", StripParens(e))
	}
}
