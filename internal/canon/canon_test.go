package canon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// parseBody parses body as the statements of a function and returns them
// together with the FileSet.
func parseBody(t *testing.T, body string) ([]ast.Stmt, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	src := "package p\n\nfunc _() {\n" + body + "\n}\n"
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	return f.Decls[0].(*ast.FuncDecl).Body.List, fset
}

// TestNormalize covers every conversion example in plan section 4.6. Each
// case lists the expected text of each top-level statement of body.
func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "call arguments on separate lines",
			body: "log.Printf(\n\t\"find %s\",\n\tid,\n)",
			want: []string{`log.Printf("find %s", id)`},
		},
		{
			name: "if with init and multi-line calls",
			body: "if err := r.check(\n\tctx,\n); err != nil {\n" +
				"\treturn *new(T), fmt.Errorf(\"check: %w\",\n\t\terr)\n}",
			want: []string{`if err := r.check(ctx); err != nil { return *new(T), fmt.Errorf("check: %w", err) }`},
		},
		{
			name: "nested composite literals",
			body: "v := Config{\n\tA: 1,\n\tB: []int{\n\t\t1, 2,\n\t},\n}",
			want: []string{`v := Config{A: 1, B: []int{1, 2}}`},
		},
		{
			name: "block statements are separated by one space",
			body: "if x {\n\ta()\n\tb()\n}\nf := func() {\n\ta()\n\tb()\n}",
			want: []string{`if x { a() b() }`, `f := func() { a() b() }`},
		},
		{
			name: "header semicolons stay",
			body: "for i := 0; i < n; i++ {\n\ta()\n}\nif err := f(); err != nil {\n\treturn err\n}",
			want: []string{
				`for i := 0; i < n; i++ { a() }`,
				`if err := f(); err != nil { return err }`,
			},
		},
		{
			name: "binary operator spacing follows gofmt",
			body: "x := a * b + c\ny := a*b",
			want: []string{`x := a*b + c`, `y := a * b`},
		},
		{
			name: "parentheses around a condition are removed",
			body: "if (err != nil) {\n\treturn err\n}",
			want: []string{`if err != nil { return err }`},
		},
		{
			name: "whitespace inside a string literal collapses",
			body: `log.Printf("a  b")`,
			want: []string{`log.Printf("a b")`},
		},
		{
			name: "raw string spanning lines becomes one line",
			body: "s := `a\n\t  b\r\n`",
			want: []string{"s := `a b `"},
		},
		{
			name: "comments are dropped",
			body: "a() // trailing\nif x {\n\t// leading\n\tb() /* inline */\n}",
			want: []string{`a()`, `if x { b() }`},
		},
		{
			// go/printer prints the comments that declarations, specs,
			// and fields hold, and they change its layout.
			name: "comments held by nodes are dropped",
			body: "var x = 1 // trailing\n// doc\nvar m = map[string]int{}\n" +
				"var (\n\t// doc\n\tz = 3 // z\n)\n/* doc */ type S struct{ A int // a\n}",
			want: []string{`var x = 1`, `var m = map[string]int{}`, `var ( z = 3 )`, `type S struct{ A int }`},
		},
		{
			name: "line breaks inside an expression do not matter",
			body: "ok := a &&\n\tb ||\n\tc",
			want: []string{`ok := a && b || c`},
		},
		{
			name: "labeled statement",
			body: "outer:\n\tfor {\n\t\tbreak outer\n\t}",
			want: []string{`outer: for { break outer }`},
		},
		{
			name: "non-ASCII text is kept",
			body: "log.Print(\"caf\u00e9\u00a0x\")",
			want: []string{"log.Print(\"caf\u00e9\u00a0x\")"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmts, _ := parseBody(t, tt.body)
			if len(stmts) != len(tt.want) {
				t.Fatalf("got %d statements, want %d", len(stmts), len(tt.want))
			}
			for i, s := range stmts {
				if got := Normalize(s); got != tt.want[i] {
					t.Errorf("statement %d:\n got %q\nwant %q", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeExpr(t *testing.T) {
	stmts, _ := parseBody(t, "log.Debug(\n\t\"user\",\n\tid,\n)")
	call := stmts[0].(*ast.ExprStmt).X
	if got, want := Normalize(call), `log.Debug("user", id)`; got != want {
		t.Errorf("Normalize(call) = %q, want %q", got, want)
	}
}

// TestNormalizeKeepsComments checks that Normalize leaves the comment
// fields of the tree as they were.
func TestNormalizeKeepsComments(t *testing.T) {
	src := "package p\n\n// F does it.\nfunc F() {\n\tvar x = 1 // x\n}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	spec := fd.Body.List[0].(*ast.DeclStmt).Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
	doc, comment := fd.Doc, spec.Comment
	if got, want := Normalize(fd), "func F() { var x = 1 }"; got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
	if fd.Doc != doc || doc == nil || spec.Comment != comment || comment == nil {
		t.Errorf("comment fields changed: Doc %p to %p, Comment %p to %p", doc, fd.Doc, comment, spec.Comment)
	}
}

func TestNormalizeUnsupported(t *testing.T) {
	if got := Normalize(nil); got != "" {
		t.Errorf("Normalize(nil) = %q, want empty", got)
	}
	if got := Normalize(&ast.FieldList{}); got != "" {
		t.Errorf("Normalize(FieldList) = %q, want empty", got)
	}
}

func TestCollapse(t *testing.T) {
	tests := map[string]string{
		"":                    "",
		" \t\n":               "",
		"a":                   "a",
		"  a  ":               "a",
		"a \t\r\n\f\vb":       "a b",
		"a\n\n\tb\n  c\n":     "a b c",
		"\u00a0a\u3000":       "\u00a0a\u3000",
		"x := `a\r\n\tb`\n\n": "x := `a b`",
	}
	for in, want := range tests {
		if got := collapse([]byte(in)); got != want {
			t.Errorf("collapse(%q) = %q, want %q", in, got, want)
		}
	}
}

// lit parses a string literal that is exactly size bytes long.
func lit(t *testing.T, fset *token.FileSet, size int) ast.Expr {
	t.Helper()
	src := `"` + strings.Repeat("a", size-2) + `"`
	e, err := parser.ParseExprFrom(fset, "lit.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestCacheSizeLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		size  int
		want  bool
	}{
		{"below limit", 16, 15, true},
		{"at limit", 16, 16, true},
		{"one byte over", 16, 17, false},
		{"far over", 16, 4000, false},
		{"default limit reached", 0, DefaultMaxNodeSize, true},
		{"default limit exceeded", 0, DefaultMaxNodeSize + 1, false},
		{"negative limit means default", -1, DefaultMaxNodeSize + 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			n := lit(t, fset, tt.size)
			got, ok := NewCache(fset, tt.limit).Text(n)
			if ok != tt.want {
				t.Fatalf("Text ok = %v, want %v", ok, tt.want)
			}
			if ok && got != Normalize(n) {
				t.Errorf("Text = %q, want %q", got, Normalize(n))
			}
			if !ok && got != "" {
				t.Errorf("Text = %q with ok false, want empty", got)
			}
		})
	}
}

// TestCacheNodeSize checks that the size of a multi-line node counts source
// bytes, not the length of the normalized text.
func TestCacheNodeSize(t *testing.T) {
	body := "if x {\n\t\t\t\t\t\t\t\ta()\n}"
	stmts, fset := parseBody(t, body)
	if _, ok := NewCache(fset, len(body)).Text(stmts[0]); !ok {
		t.Errorf("node of %d bytes rejected with limit %d", len(body), len(body))
	}
	if _, ok := NewCache(fset, len(body)-1).Text(stmts[0]); ok {
		t.Errorf("node of %d bytes accepted with limit %d", len(body), len(body)-1)
	}
}

func TestNormalizeHasNoLimit(t *testing.T) {
	n := lit(t, token.NewFileSet(), 3*DefaultMaxNodeSize)
	if got := Normalize(n); len(got) != 3*DefaultMaxNodeSize {
		t.Errorf("len(Normalize) = %d, want %d", len(got), 3*DefaultMaxNodeSize)
	}
}

func TestCacheMemo(t *testing.T) {
	stmts, fset := parseBody(t, "a(\n\t1,\n)\nb()")
	c := NewCache(fset, 0)
	first, ok := c.Text(stmts[0])
	if !ok || first != "a(1)" {
		t.Fatalf("Text = %q, %v", first, ok)
	}
	// A memoized entry is returned as is.
	c.memo[stmts[0]] = "memo"
	if got, _ := c.Text(stmts[0]); got != "memo" {
		t.Errorf("Text after memo = %q, want %q", got, "memo")
	}
	if got, _ := c.Text(stmts[1]); got != "b()" {
		t.Errorf("Text = %q, want %q", got, "b()")
	}
	if len(c.memo) != 2 {
		t.Errorf("memo has %d entries, want 2", len(c.memo))
	}
}

func TestCacheUnavailable(t *testing.T) {
	stmts, fset := parseBody(t, "a()")
	tests := []struct {
		name string
		c    *Cache
		n    ast.Node
	}{
		{"nil node", NewCache(fset, 0), nil},
		{"position from another FileSet", NewCache(token.NewFileSet(), 0), stmts[0]},
		{"node without position", NewCache(fset, 0), &ast.Ident{Name: "x"}},
		{"no FileSet and no position", NewCache(nil, 0), &ast.Ident{Name: "x"}},
		{"printer cannot print", NewCache(fset, 0), &ast.FieldList{Opening: stmts[0].Pos()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := tt.c.Text(tt.n); ok || got != "" {
				t.Errorf("Text = %q, %v; want \"\", false", got, ok)
			}
		})
	}
	// Without a FileSet the size comes from the positions alone.
	if got, ok := NewCache(nil, 0).Text(stmts[0]); !ok || got != "a()" {
		t.Errorf("Text without FileSet = %q, %v; want \"a()\", true", got, ok)
	}
}
