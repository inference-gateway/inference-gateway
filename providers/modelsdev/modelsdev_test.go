package modelsdev

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	gptXReleased           int64 = 1754524800
	commandA032025Released int64 = 1741824000
)

func TestLookupKeys(t *testing.T) {
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
			if got := lookupKeys(tt.id); !slices.Equal(got, tt.want) {
				t.Fatalf("lookupKeys(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// TestLookupFallsThroughPerField checks each attribute comes from the most
// specific ID variant carrying it: the exact date-pinned row supplies pricing,
// while status, release date and context window fall through to the base row.
func TestLookupFallsThroughPerField(t *testing.T) {
	pinnedPricing := types.Pricing{Currency: "USD", InputPerToken: "0.000001", OutputPerToken: "0.000002", Source: types.PricingSourceCommunity}
	basePricing := types.Pricing{Currency: "USD", InputPerToken: "0.000009", OutputPerToken: "0.000009", Source: types.PricingSourceCommunity}
	entries := map[string]Entry{
		"openai/gpt-x-2025-08-07": {Pricing: &pinnedPricing, ContextWindow: &ContextWindow{Context: -1}},
		"openai/gpt-x": {
			Pricing:       &basePricing,
			ContextWindow: &ContextWindow{Context: 128000},
			Status:        deprecatedStatus,
			ReleaseDate:   "2025-08-07",
		},
	}

	got := lookup(entries, "openai/gpt-x-2025-08-07")

	require.NotNil(t, got.Pricing)
	assert.Equal(t, pinnedPricing.InputPerToken, got.Pricing.InputPerToken)
	assert.Equal(t, &types.ContextWindow{Tokens: 128000, Source: types.ContextWindowSourceCommunity}, got.ContextWindow)
	assert.True(t, got.Deprecated)
	assert.Equal(t, gptXReleased, got.Created)
	assert.Equal(t, Metadata{}, lookup(entries, "openai/gpt-unknown"))
}

// TestLookupReturnsCopies guards the shared embedded table: mutating what one
// Lookup returned must not leak into the next.
func TestLookupReturnsCopies(t *testing.T) {
	first := Lookup("openai/gpt-4o")
	require.NotNil(t, first.Pricing)
	require.NotNil(t, first.Modalities)
	first.Pricing.InputPerToken = "mutated"
	first.Modalities.Input[0] = types.ModalityVideo

	second := Lookup("openai/gpt-4o")
	assert.NotEqual(t, "mutated", second.Pricing.InputPerToken)
	assert.NotEqual(t, types.ModalityVideo, second.Modalities.Input[0])
}

// TestLookupEmbeddedTable spot-checks the synced table through ID variants:
// date-pinned and tagged IDs resolve, image-generation models are image-out
// without text-out, and the hand-curated speech overrides survive the sync.
func TestLookupEmbeddedTable(t *testing.T) {
	assert.NotNil(t, Lookup("anthropic/claude-sonnet-4-5-19990101").ContextWindow, "date-pinned ID must resolve")
	assert.True(t, Lookup("minimax/MiniMax-M2:latest").Deprecated, "tagged deprecated ID must resolve")
	assert.False(t, Lookup("openai/gpt-nonexistent").Deprecated)
	assert.Equal(t, commandA032025Released, Lookup("cohere/command-a-03-2025").Created)

	imageGen := Lookup("openai/gpt-image-2").Modalities
	require.NotNil(t, imageGen)
	assert.False(t, slices.Contains(imageGen.Output, types.ModalityText), "gpt-image-2 must be image-out only")

	tts := Lookup("openai/tts-1").Modalities
	require.NotNil(t, tts)
	assert.Contains(t, tts.Output, types.ModalityAudio)
	assert.NotContains(t, tts.Output, types.ModalityText)
	stt := Lookup("openai/whisper-1").Modalities
	require.NotNil(t, stt)
	assert.Contains(t, stt.Input, types.ModalityAudio)
}

// TestAcceptsImages verifies the per-model vision gate: table-confirmed vision
// models pass (including date-pinned IDs), table-confirmed text-only models
// fail, and unknown models pass so requests are never silently stripped.
func TestAcceptsImages(t *testing.T) {
	tests := []struct {
		name     string
		provider types.Provider
		model    string
		want     bool
	}{
		{"vision model with date pin", "anthropic", "claude-haiku-4-5-20251001", true},
		{"vision model", "anthropic", "claude-haiku-4-5", true},
		{"text-only model", "openai", "gpt-4-0613", false},
		{"unknown model is permissive", "openai", "gpt-nonexistent", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AcceptsImages(tt.provider, tt.model); got != tt.want {
				t.Errorf("AcceptsImages(%s, %s) = %v, want %v", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

// TestOverridesMatchEmbeddedTable checks every hand-maintained override is
// already applied to the committed table, so a manual edit to the overrides
// file can't ship without a re-sync.
func TestOverridesMatchEmbeddedTable(t *testing.T) {
	data, err := os.ReadFile("models.overrides.json")
	require.NoError(t, err)
	var overrides map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &overrides))

	entries := table()
	for key, override := range overrides {
		committed, err := json.Marshal(entries[key])
		require.NoError(t, err)
		var merged Entry
		require.NoError(t, json.Unmarshal(committed, &merged))
		require.NoError(t, json.Unmarshal(override, &merged))
		applied, err := json.Marshal(merged)
		require.NoError(t, err)
		assert.JSONEq(t, string(committed), string(applied), "override for %q is not reflected in models.json", key)
	}
}
