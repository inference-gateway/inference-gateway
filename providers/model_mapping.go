package providers

import "strings"

// DetermineProviderAndModelName analyzes a model string and tries to determine
// the provider based on naming conventions. It returns both the detected provider
// and the model name (which might be modified to strip provider prefixes).
//
// It first checks for explicit provider prefixes like "ollama/", "groq/", etc.
// Then checks for model-name based prefixes like "gpt-", "claude-", etc.
//
// Returns nil provider if no provider could be determined.
func DetermineProviderAndModelName(model string) (provider *Provider, modelName string) {
	modelLower := strings.ToLower(model)

	// First check for explicit provider prefixes (ollama/, groq/, etc.)
	providerPrefixMapping := map[string]Provider{
		"ollama/":     OllamaID,
		"groq/":       GroqID,
		"cloudflare/": CloudflareID,
		"openai/":     OpenaiID,
		"anthropic/":  AnthropicID,
		"cohere/":     CohereID,
		"deepseek/":   DeepseekID,
	}

	for prefix, providerID := range providerPrefixMapping {
		if strings.HasPrefix(modelLower, prefix) {
			return &providerID, strings.TrimPrefix(model, prefix)
		}
	}

	// Then check for model-name based prefixes (gpt-, claude-, etc.)
	modelPrefixMapping := map[string]Provider{
		"gpt-":      OpenaiID,
		"claude-":   AnthropicID,
		"llama-":    GroqID,
		"command-":  CohereID,
		"deepseek-": GroqID,
	}

	for prefix, providerID := range modelPrefixMapping {
		if strings.HasPrefix(modelLower, prefix) {
			return &providerID, model
		}
	}

	return nil, model
}
