package core

import (
	"reflect"
	"testing"
)

func TestCommunityLookupKeys(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want []string
	}{
		{"plain", "openai/gpt-4o", []string{"openai/gpt-4o"}},
		{"google models prefix", "google/models/gemini-2.0-flash", []string{"google/models/gemini-2.0-flash", "google/gemini-2.0-flash", "google/models/gemini-2_0-flash", "google/gemini-2_0-flash"}},
		{"latest alias", "anthropic/claude-3-5-sonnet-latest", []string{"anthropic/claude-3-5-sonnet-latest", "anthropic/claude-3-5-sonnet"}},
		{"date pin", "anthropic/claude-sonnet-4-5-20250929", []string{"anthropic/claude-sonnet-4-5-20250929", "anthropic/claude-sonnet-4-5"}},
		{"not a date suffix", "groq/llama-3.3-70b", []string{"groq/llama-3.3-70b", "groq/llama-3_3-70b"}},
		{"no provider prefix", "gpt-4o", []string{"gpt-4o"}},
		{"dotted nim id", "nvidia/upstage/solar-10.7b-instruct", []string{"nvidia/upstage/solar-10.7b-instruct", "nvidia/upstage/solar-10_7b-instruct"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := communityLookupKeys(tt.id); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("communityLookupKeys(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
