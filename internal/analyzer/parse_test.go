package analyzer

import (
	"strings"
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

func TestParseFileError(t *testing.T) {
	// The //line directive moves the reported position, but the skip
	// refers to the line of the file itself.
	src := "package p\n\n//line gen.y:100\nfunc f() {\n\tx :=\n}\n"
	info, sk := scanFile([]byte(src), DefaultOptions())
	if sk != nil {
		t.Fatalf("scan: %+v", sk)
	}
	_, _, sk = parseFile("x.go", []byte(src), info)
	if sk == nil || sk.reason != result.SkipParseError {
		t.Fatalf("got %+v, want parse-error", sk)
	}
	if sk.line != 6 || sk.column != 1 {
		t.Errorf("got %d:%d, want 6:1", sk.line, sk.column)
	}
	if sk.msg != "expected operand, found '}'" {
		t.Errorf("got message %q", sk.msg)
	}
}

func TestParseFileManyErrors(t *testing.T) {
	// More than ten errors must not stop the parser with a panic.
	src := "package p\n\n" + strings.Repeat("func {\n", 30)
	info, sk := scanFile([]byte(src), DefaultOptions())
	if sk != nil {
		t.Fatalf("scan: %+v", sk)
	}
	if _, _, sk := parseFile("x.go", []byte(src), info); sk == nil || sk.reason != result.SkipParseError {
		t.Fatalf("got %+v, want parse-error", sk)
	}
}

func TestPrepare(t *testing.T) {
	opt := Options{MaxASTDepth: 20}.withDefaults()
	p, sk := prepare("x.go", []byte(plusChain(16)), opt)
	if sk != nil {
		t.Fatalf("got skip %+v", sk)
	}
	if p.rf.Path != "x.go" || p.rf.AST != p.file || p.info.lines() != 3 {
		t.Errorf("parsed = %+v", p)
	}
	if text, ok := p.cache.Text(p.file.Decls[0]); !ok || !strings.HasPrefix(text, "var x = 1 + 1") {
		t.Errorf("Text = %q, %v", text, ok)
	}
	_, sk = prepare("x.go", []byte(plusChain(17)), opt)
	if sk == nil || sk.reason != result.SkipTooDeep || sk.line != 3 || !strings.Contains(sk.msg, "more than 20 levels") {
		t.Errorf("got %+v, want too-deep on line 3", sk)
	}
}
