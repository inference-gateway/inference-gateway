package routing

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestResolveProvider(t *testing.T) {
	tests := []struct {
		name             string
		queryParam       string
		model            string
		expectedProvider types.Provider
		expectedModel    string
		expectedOK       bool
	}{
		{
			name:             "Query parameter wins over model prefix",
			queryParam:       "groq",
			model:            "openai/gpt-4",
			expectedProvider: types.Provider("groq"),
			expectedModel:    "openai/gpt-4",
			expectedOK:       true,
		},
		{
			name:             "Model prefix is used when no query parameter",
			model:            "openai/gpt-4",
			expectedProvider: types.Provider("openai"),
			expectedModel:    "gpt-4",
			expectedOK:       true,
		},
		{
			name:          "Neither query parameter nor prefix",
			model:         "gpt-4",
			expectedModel: "gpt-4",
		},
		{
			name: "Empty model without query parameter",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, model, ok := ResolveProvider(tc.queryParam, tc.model)

			assert.Equal(t, tc.expectedOK, ok, "ok should match expected value")
			assert.Equal(t, tc.expectedProvider, provider, "provider should match expected value")
			assert.Equal(t, tc.expectedModel, model, "model should match expected value")
		})
	}
}

func TestDetermineProviderAndModelNameForEveryRegisteredProvider(t *testing.T) {
	for id := range registry.Registry {
		t.Run(string(id), func(t *testing.T) {
			provider, model := DetermineProviderAndModelName(string(id) + "/some-model")
			require.NotNil(t, provider)
			assert.Equal(t, id, *provider)
			assert.Equal(t, "some-model", model)
		})
	}
}

func TestDetermineProviderAndModelNameEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		expectedProvider *types.Provider
		expectedModel    string
	}{
		{
			name:             "Case insensitive prefix matching",
			model:            "OpenAI/GPT-5.5",
			expectedProvider: new(constants.OpenaiID),
			expectedModel:    "GPT-5.5",
		},
		{
			name:             "Model name containing slashes",
			model:            "cloudflare/@cf/meta/llama-2-7b-chat-fp16",
			expectedProvider: new(constants.CloudflareID),
			expectedModel:    "@cf/meta/llama-2-7b-chat-fp16",
		},
		{
			name:             "Model without explicit prefix",
			model:            "gpt-4",
			expectedProvider: nil,
			expectedModel:    "gpt-4",
		},
		{
			name:             "Unknown provider prefix",
			model:            "unknownai/some-model",
			expectedProvider: nil,
			expectedModel:    "unknownai/some-model",
		},
		{
			name:             "Unknown model",
			model:            "unknown-model",
			expectedProvider: nil,
			expectedModel:    "unknown-model",
		},
		{
			name:             "Empty model",
			model:            "",
			expectedProvider: nil,
			expectedModel:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, model := DetermineProviderAndModelName(tc.model)

			if tc.expectedProvider == nil {
				assert.Nil(t, provider, "provider should be nil")
			} else {
				assert.NotNil(t, provider, "provider should not be nil")
				assert.Equal(t, *tc.expectedProvider, *provider, "provider should match expected value")
			}

			assert.Equal(t, tc.expectedModel, model, "model should match expected value")
		})
	}
}
