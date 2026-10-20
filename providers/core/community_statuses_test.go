package core

import "testing"

// TestIsDeprecated exercises the embedded community status table: a synced
// deprecated entry resolves directly and through an ID variant, while a
// current model and a model absent from the table are not deprecated.
func TestIsDeprecated(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{id: "minimax/MiniMax-M2", want: true},
		{id: "minimax/MiniMax-M2:latest", want: true},
		{id: "minimax/MiniMax-M3", want: false},
		{id: "openai/gpt-nonexistent", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsDeprecated(tt.id); got != tt.want {
				t.Errorf("IsDeprecated(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
