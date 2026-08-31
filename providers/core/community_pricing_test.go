package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
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
		{"dashed date pin", "openai/gpt-5-2025-08-07", []string{"openai/gpt-5-2025-08-07", "openai/gpt-5"}},
		{"ollama tag", "ollama_cloud/deepseek-v4-pro:0813", []string{"ollama_cloud/deepseek-v4-pro:0813", "ollama_cloud/deepseek-v4-pro"}},
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

func TestCommunityPricingOverridesMatchGeneratedTable(t *testing.T) {
	data, err := os.ReadFile("community_pricing.overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	var overrides map[string]types.Pricing
	if err := json.Unmarshal(data, &overrides); err != nil {
		t.Fatal(err)
	}

	table := communityPricing()
	for model, override := range overrides {
		if got, ok := table[model]; !ok || !reflect.DeepEqual(got, override) {
			t.Errorf("generated pricing for %q does not match its override", model)
		}
	}
}
