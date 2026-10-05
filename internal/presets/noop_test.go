package presets

import (
	"testing"

	"github.com/miyamo2/go-tebanare/internal/rule"
)

func TestNoop(t *testing.T) {
	src := `
func (T) Unnamed() {}
func (_ *T) Blank() {}
func (c *Cache[K, V]) GenericRecv() {}

// Doc is a doc comment.
//
//go:noinline
func (t *T) Doc() {}
func (t *T) Body() { /* body */ }
func (t *T) Trailing() {} // trailing
func (t *T) Semicolon() { ; }
func (t *T) Result() (err error) { return }
func Plain() {}
func Generic[T any]() {}
func Params(x int) {}
`
	tests := []struct {
		settings string
		want     map[string]bool
	}{
		{"", map[string]bool{
			"Unnamed": true, "Blank": true, "GenericRecv": true, "Doc": true, "Body": true, "Trailing": true,
			"Semicolon": false, "Result": false, "Plain": false, "Generic": false, "Params": false,
		}},
		{"allow_comments: false", map[string]bool{
			"Unnamed": true, "Doc": true, "Body": false, "Trailing": false, "Plain": false,
		}},
		{"include_functions: true", map[string]bool{
			"Unnamed": true, "Plain": true, "Generic": true, "Params": false,
		}},
		{"include_functions: true\nallow_comments: false", map[string]bool{
			"Plain": true, "Body": false,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.settings, func(t *testing.T) {
			checkResults(t, funcResults(t, "noop", tt.settings, src), tt.want)
		})
	}
}

func TestNoopCompileErrors(t *testing.T) {
	p, _ := Lookup("noop")
	s := p.NewSettings().(*NoopSettings)
	s.Paths = []string{"["}
	if _, err := p.Compile(s); err == nil {
		t.Errorf("Compile(%+v) succeeded", s)
	}
}

var _ rule.FuncMatcher = (*noopMatcher)(nil)
