package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineProviderAndModelName(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		expectedProvider *Provider
		expectedModel    string
	}{
		{
			name:             "OpenAI model with prefix",
			model:            "openai/gpt-4",
			expectedProvider: pointerToProvider(OpenaiID),
			expectedModel:    "gpt-4",
		},
		{
			name:             "Anthropic model with prefix",
			model:            "anthropic/claude-3",
			expectedProvider: pointerToProvider(AnthropicID),
			expectedModel:    "claude-3",
		},
		{
			name:             "Groq model with prefix",
			model:            "groq/llama-7b",
			expectedProvider: pointerToProvider(GroqID),
			expectedModel:    "llama-7b",
		},
		{
			name:             "Ollama model with prefix",
			model:            "ollama/mistral",
			expectedProvider: pointerToProvider(OllamaID),
			expectedModel:    "mistral",
		},
		{
			name:             "OpenAI model by name",
			model:            "gpt-4",
			expectedProvider: pointerToProvider(OpenaiID),
			expectedModel:    "gpt-4",
		},
		{
			name:             "Anthropic model by name",
			model:            "claude-3",
			expectedProvider: pointerToProvider(AnthropicID),
			expectedModel:    "claude-3",
		},
		{
			name:             "Llama model by name",
			model:            "llama-7b",
			expectedProvider: pointerToProvider(GroqID),
			expectedModel:    "llama-7b",
		},
		{
			name:             "Deepseek model by name",
			model:            "deepseek-coder",
			expectedProvider: pointerToProvider(GroqID),
			expectedModel:    "deepseek-coder",
		},
		{
			name:             "Cohere model by name",
			model:            "command-nightly",
			expectedProvider: pointerToProvider(CohereID),
			expectedModel:    "command-nightly",
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

// Helper function to convert Provider to *Provider
func pointerToProvider(p Provider) *Provider {
	return &p
}
