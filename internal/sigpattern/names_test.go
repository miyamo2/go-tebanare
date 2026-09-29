package sigpattern

import (
	"go/token"
	"testing"
)

// parseNameText parses src as one name.
func parseNameText(src string) (name, error) {
	p := newParser(src)
	n := p.parseName("name")
	p.expect(token.EOF)
	if p.err != nil {
		return name{}, p.err
	}
	return n, nil
}

func TestParseName(t *testing.T) {
	tests := []struct {
		src   string
		match []string
		miss  []string
	}{
		{"Get", []string{"Get"}, []string{"GetX", "get"}},
		{"Find*", []string{"Find", "FindAll"}, []string{"find", "XFind"}},
		{"*Handler", []string{"Handler", "apiHandler"}, []string{"HandlerX"}},
		{"Get?", []string{"GetX", "Geté"}, []string{"Get", "GetXY"}},
		{"*go*", []string{"go", "undergone"}, []string{"Go"}},
		{"V*2i", []string{"V2i", "Vx2i"}, []string{"V2"}},
		{"/^(Get|Set)[A-Z]/", []string{"GetX", "SetName"}, []string{"Getx", "xGetX"}},
		{"/Handler/", []string{"apiHandlerFunc"}, []string{"handler"}},
		{`/a\/b/`, []string{"xa/b"}, []string{`a\/b`}},
	}
	for _, tt := range tests {
		n, err := parseNameText(tt.src)
		if err != nil {
			t.Errorf("%q: %v", tt.src, err)
			continue
		}
		for _, s := range tt.match {
			if !n.match(s) {
				t.Errorf("%q does not match %q", tt.src, s)
			}
		}
		for _, s := range tt.miss {
			if n.match(s) {
				t.Errorf("%q matches %q", tt.src, s)
			}
		}
	}
}

func TestParseNameErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"", "1:1: expected name, found 'EOF'"},
		{"(", "1:1: expected name, found '('"},
		{"/abc", "1:1: regexp not terminated"},
		{"/a(/", "1:1: error parsing regexp: missing closing ): `a(`"},
		{"/*H/", "1:1: error parsing regexp: missing argument to repetition operator: `*`"},
		{"Get$", "1:4: illegal character U+0024 '$'"},
		{"Get*0x", "1:7: hexadecimal literal has no digits"},
		{"a.b*", "1:2: expected 'EOF', found '.'"}, // '.' cannot be part of a glob
		{"F?\xef\xbb\xbf", "1:3: illegal byte order mark"},
		{"F?\x00", "1:3: illegal character NUL"},
		{"F?\xff", "1:3: illegal UTF-8 encoding"},
	}
	for _, tt := range tests {
		_, err := parseNameText(tt.src)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q: got error %v, want %q", tt.src, err, tt.want)
		}
	}
}
