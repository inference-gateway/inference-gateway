package videos

import (
	"testing"

	require "github.com/stretchr/testify/require"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		name  string
		size  string
		want  Size
		valid bool
	}{
		{name: "a portrait size parses", size: "1024x1536", want: Size{Width: 1024, Height: 1536}, valid: true},
		{name: "a square size parses", size: "720x720", want: Size{Width: 720, Height: 720}, valid: true},
		{name: "a missing separator is refused", size: "1024", valid: false},
		{name: "a non numeric side is refused", size: "1024xabc", valid: false},
		{name: "a zero side is refused", size: "0x1024", valid: false},
		{name: "a negative side is refused", size: "1024x-1536", valid: false},
		{name: "an empty size is refused", size: "", valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseSize(tt.size)
			require.Equal(t, tt.valid, ok, "ParseSize(%q) ok", tt.size)
			require.Equal(t, tt.want, got, "ParseSize(%q)", tt.size)
		})
	}
}

func TestSizeAspectRatio(t *testing.T) {
	s, ok := ParseSize("1024x1536")
	require.True(t, ok)

	width, height := s.AspectRatio()
	require.Equal(t, 2, width, "1024x1536 reduces to 2:3")
	require.Equal(t, 3, height, "1024x1536 reduces to 2:3")
}
