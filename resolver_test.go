package g2

import (
	"context"
	"testing"
)

func TestResolveBash(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		variables map[string]string
		opts      []ResolveBashOption
		want      string
	}{
		{
			name:      "Simple substitution (fast path)",
			text:      "foo $VAR bar ${OTHER}",
			variables: map[string]string{"VAR": "123", "OTHER": "456"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "foo 123 bar 456",
		},
		{
			name:      "No variables (fast path fast exit)",
			text:      "just some literal text",
			variables: map[string]string{"VAR": "123"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "just some literal text",
		},
		{
			name:      "WithVars option merging",
			text:      "x $A y $B z $C",
			variables: map[string]string{"A": "1"},
			opts:      []ResolveBashOption{WithFastPath(), WithVars(map[string]string{"B": "2", "C": "3"})},
			want:      "x 1 y 2 z 3",
		},
		{
			name:      "Complex substitution fallback (Default Value)",
			text:      "foo ${VAR:-default}",
			variables: map[string]string{},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "foo default",
		},
		{
			name:      "Complex substitution fallback (Replacement)",
			text:      "foo ${VAR/x/y}",
			variables: map[string]string{"VAR": "1x2x3"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "foo 1y2x3",
		},
		{
			name:      "Bash Command fallback (echo)",
			text:      "echo $VAR",
			variables: map[string]string{"VAR": "hello"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "hello",
		},
		{
			name:      "Bash Logical fallback (if statement)",
			text:      "if [ -n \"$VAR\" ]; then echo \"yes\"; else echo \"no\"; fi",
			variables: map[string]string{"VAR": "1"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "yes",
		},
		{
			name:      "Blocklist trigger (Quotes) falling back correctly",
			text:      "\"$VAR\"",
			variables: map[string]string{"VAR": "quoted"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "\"quoted\"", // literal is preserving quotes
		},
		{
			name:      "Command Substitution fallback",
			text:      "echo $(echo $VAR)",
			variables: map[string]string{"VAR": "subshell"},
			opts:      []ResolveBashOption{WithFastPath()},
			want:      "subshell",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveBash(ctx, tt.text, tt.variables, tt.opts...); got != tt.want {
				t.Errorf("resolveBash() = %q, want %q", got, tt.want)
			}
		})
	}
}
