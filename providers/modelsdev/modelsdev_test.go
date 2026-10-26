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

func TestModelVariants(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  []string
	}{
		{"plain", "gpt-4o", []string{"gpt-4o"}},
		{"google models prefix", "models/gemini-2.0-flash", []string{"models/gemini-2.0-flash", "gemini-2.0-flash"}},
		{"latest alias", "claude-3-5-sonnet-latest", []string{"claude-3-5-sonnet-latest", "claude-3-5-sonnet"}},
		{"date pin", "claude-sonnet-4-5-20250929", []string{"claude-sonnet-4-5-20250929", "claude-sonnet-4-5"}},
		{"dashed date pin", "gpt-5-2025-08-07", []string{"gpt-5-2025-08-07", "gpt-5"}},
		{"ollama tag", "deepseek-v4-pro:0813", []string{"deepseek-v4-pro:0813", "deepseek-v4-pro"}},
		{"not a date suffix", "llama-3.3-70b", []string{"llama-3.3-70b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modelVariants(tt.model, nil); !slices.Equal(got, tt.want) {
				t.Fatalf("modelVariants(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// TestLookupResolvesDottedNames checks a dotted upstream name finds the
// underscored file name models.dev commits, directly and behind a tag.
func TestLookupResolvesDottedNames(t *testing.T) {
	catalog := group(map[string]Entry{"nvidia/upstage/solar-10_7b-instruct": {ReleaseDate: "2024-06-01"}})[types.Provider("nvidia")]

	assert.NotZero(t, catalog.Lookup("upstage/solar-10.7b-instruct").Created)
	assert.NotZero(t, catalog.Lookup("upstage/solar-10.7b-instruct:latest").Created)
	assert.Zero(t, catalog.Lookup("upstage/solar-11.7b-instruct").Created)
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

	catalog := group(entries)[types.Provider("openai")]
	got := catalog.Lookup("gpt-x-2025-08-07")

	require.NotNil(t, got.Pricing)
	assert.Equal(t, pinnedPricing.InputPerToken, got.Pricing.InputPerToken)
	assert.Equal(t, &types.ContextWindow{Tokens: 128000, Source: types.ContextWindowSourceCommunity}, got.ContextWindow)
	assert.True(t, got.Deprecated())
	assert.Equal(t, gptXReleased, got.Created)
	assert.Equal(t, Metadata{}, catalog.Lookup("gpt-unknown"))
}

// TestLookupMatchesUnresolvedPath checks the names pre-resolved at load return
// exactly what resolving their variants on the fly returns.
func TestLookupMatchesUnresolvedPath(t *testing.T) {
	for provider, catalog := range catalogs() {
		for name, known := range catalog.resolved {
			require.Equal(t, resolve(catalog.rows, modelVariants(name, nil)), known, "%s/%s", provider, name)
		}
	}
}

// TestLookupEmbeddedTable spot-checks the synced table through ID variants:
// date-pinned and tagged IDs resolve, image-generation models are image-out
// without text-out, and the hand-curated speech overrides survive the sync.
func TestLookupEmbeddedTable(t *testing.T) {
	assert.NotNil(t, Lookup("anthropic/claude-sonnet-4-5-19990101").ContextWindow, "date-pinned ID must resolve")
	assert.True(t, Lookup("minimax/MiniMax-M2:latest").Deprecated(), "tagged deprecated ID must resolve")
	assert.False(t, Lookup("openai/gpt-nonexistent").Deprecated())
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

	var entries map[string]Entry
	require.NoError(t, json.Unmarshal(modelsJSON, &entries))
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
