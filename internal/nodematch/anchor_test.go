package nodematch

import (
	"strings"
	"testing"
)

func TestIsAnchored(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{`^a`, true},
		{`a$`, true},
		{`\Aa`, true},
		{`a\z`, true},
		{`^a$`, true},
		{`^`, true},
		{`$`, true},
		{`^$`, true},
		{`(?i)^a`, true},
		{`(?m)^a`, true},
		{`(?m)a$`, true},
		{`^(a|b)`, true},
		{`(a|b)$`, true},
		{`^ctx, span := \w+\.Start\(ctx, .+\)$`, true},
		{`^defer span\.End\(\)$`, true},
		{`^(log|slog|logger)\.Debug\w*\(`, true},
		{`(^a)`, true},
		{`((^a))`, true},
		{`(^a)b`, true},
		{`a(b$)`, true},
		{`^a|^b`, true},
		{`^abc|^abd`, true},
		{`a$|b$`, true},
		{`(?m:^a)|(?m:^b)`, true},
		{`^a|(^b|^c)`, true},
		{`(^a|^b)c`, true},
		{`(?:^a|^b)c`, true},
		{`^a$|b$`, true},
		{`^a|b`, false},
		{`a|b$`, false},
		{`^a|b$`, false},
		{`^a|^b|c`, false},
		{`(^a|b)c`, false},
		{`x(^a)`, false},
		{`a`, false},
		{`a^`, false},
		{`$a`, false},
		{`^*a`, false},
		{`^?a`, false},
		{`.*^a`, false},
		{`\ba\b`, false},
		{`(?i)password|token|secret`, false},
		{``, false},
		{`(a`, false},
		{`a)`, false},
		{`[z-a]`, false},
		{"^" + strings.Repeat("(", 201) + "a" + strings.Repeat(")", 201), false},
	}
	for _, tt := range tests {
		if got := IsAnchored(tt.expr); got != tt.want {
			t.Errorf("IsAnchored(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}
