package config

import (
	"testing"

	"github.com/miyamo2/go-tebanare/internal/result"
)

func TestParse(t *testing.T) {
	const empty = `the config is empty; it needs at least "version: 1"`
	invalid, syntax := result.CodeConfigInvalid, result.CodeConfigSyntax
	tests := []struct {
		name string
		src  string
		want []string
		code string
	}{
		{"mapping", "version: 1\n", nil, ""},
		{"list", "- a\n", nil, ""},
		{"document markers", "---\nversion: 1\n...\n", nil, ""},
		{"empty", "", []string{empty}, invalid},
		{"only comments", "# nothing\n", []string{empty}, invalid},
		{"empty document", "---\n", []string{empty}, invalid},
		{"null document", "~\n", []string{empty}, invalid},
		{"two documents", "version: 1\n---\nversion: 1\n", []string{
			"2:1: the config must be one YAML document, and another document starts here",
		}, invalid},
		{"trailing document marker", "version: 1\n---\n", nil, ""},
		{"trailing document with a comment", "version: 1\n---\n# c\n", nil, ""},
		{"trailing null documents", "version: 1\n--- ~\n---\n...\n", nil, ""},
		{"leading empty document", "---\n---\nversion: 1\n", nil, ""},
		{"document after an empty one", "version: 1\n---\n---\nversion: 1\n", []string{
			"3:1: the config must be one YAML document, and another document starts here",
		}, invalid},
		{"empty string document", "version: 1\n--- ''\n", []string{
			"2:1: the config must be one YAML document, and another document starts here",
		}, invalid},
		{"only empty documents", "---\n---\n", []string{empty}, invalid},
		{"syntax error in the second document", "version: 1\n---\n[\n", []string{
			"3: did not find expected node content",
		}, syntax},
		{"unclosed list", "version: 1\npresets: [\n", []string{"2: did not find expected node content"}, syntax},
		{"bad indentation", "version: 1\n  x: 2\n", []string{"2: mapping values are not allowed in this context"}, syntax},
		// yaml.v3 gives no line for an unknown anchor.
		{"unknown anchor", "version: 1\npresets: *nope\n", []string{"unknown anchor 'nope' referenced"}, syntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiler{}
			root, ok := c.parse([]byte(tt.src))
			if ok != (len(tt.want) == 0) || (root != nil) != ok {
				t.Errorf("parse = %v, %v", root, ok)
			}
			checkDiags(t, "errors", c.errs, tt.want)
			for _, d := range c.errs {
				if d.Severity != result.SeverityError || d.Code != tt.code {
					t.Errorf("%s: severity %s, code %s", d.Format(""), d.Severity, d.Code)
				}
			}
		})
	}
}
