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
			name:             "Ollama Cloud model with prefix",
			model:            "ollama_cloud/llama3.2:latest",
			expectedProvider: pointerToProvider(OllamaCloudID),
			expectedModel:    "llama3.2:latest",
		},
		{
			name:             "Cloudflare model with prefix",
			model:            "cloudflare/@cf/meta/llama-2-7b-chat-fp16",
			expectedProvider: pointerToProvider(CloudflareID),
			expectedModel:    "@cf/meta/llama-2-7b-chat-fp16",
		},
		{
			name:             "Cohere model with prefix",
			model:            "cohere/command-nightly",
			expectedProvider: pointerToProvider(CohereID),
			expectedModel:    "command-nightly",
		},
		{
			name:             "Deepseek model with prefix",
			model:            "deepseek/deepseek-coder",
			expectedProvider: pointerToProvider(DeepseekID),
			expectedModel:    "deepseek-coder",
		},
		{
			name:             "Case insensitive prefix matching",
			model:            "OpenAI/GPT-4",
			expectedProvider: pointerToProvider(OpenaiID),
			expectedModel:    "GPT-4",
		},
		{
			name:             "Model without explicit prefix - OpenAI style",
			model:            "gpt-4",
			expectedProvider: nil,
			expectedModel:    "gpt-4",
		},
		{
			name:             "Model without explicit prefix - Anthropic style",
			model:            "claude-3",
			expectedProvider: nil,
			expectedModel:    "claude-3",
		},
		{
			name:             "Model without explicit prefix - DeepSeek style",
			model:            "deepseek-coder",
			expectedProvider: nil,
			expectedModel:    "deepseek-coder",
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
