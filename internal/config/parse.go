package config

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// emptyConfig is the error for a config without a document.
const emptyConfig = `the config is empty; it needs at least "version: 1"`

// parse reads the YAML stream in src and returns the root node of the
// document with content. Documents that are empty or hold only null are
// skipped, so a trailing "---" is fine. A stream without a document with
// content, or with two of them, is an error.
func (c *compiler) parse(src []byte) (*yaml.Node, bool) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var root *yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			c.syntaxError(err)
			return nil, false
		}
		n := resolve(&doc)
		if n == nil || isNull(n) {
			continue
		}
		if root != nil {
			c.errorf(&doc, "", "the config must be one YAML document, and another document starts here")
			return nil, false
		}
		root = n
	}
	if root == nil {
		c.errorf(nil, "", emptyConfig)
		return nil, false
	}
	return root, true
}

// syntaxError reports an error of the YAML parser. Its messages look like
// "yaml: line 3: mapping values are not allowed in this context".
func (c *compiler) syntaxError(err error) {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	line := 0
	if rest, ok := strings.CutPrefix(msg, "line "); ok {
		if num, text, ok := strings.Cut(rest, ": "); ok {
			if n, err := strconv.Atoi(num); err == nil {
				line, msg = n, text
			}
		}
	}
	c.errs = append(c.errs, result.Diagnostic{
		Severity: result.SeverityError,
		Code:     result.CodeConfigSyntax,
		Message:  msg,
		Line:     line,
	})
}
