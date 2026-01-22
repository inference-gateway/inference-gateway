package providers

import "strings"

// DetermineProviderAndModelName analyzes a model string and tries to determine
// the provider based on explicit naming conventions only. It returns both the detected provider
// and the model name (which might be modified to strip provider prefixes).
//
// It only checks for explicit provider prefixes like "ollama/", "groq/", etc.
// Implicit model name-based routing (like "gpt-" -> OpenAI) is not supported.
//
// Returns nil provider if no explicit provider prefix is found.
// In such cases, the provider must be specified via query parameter.
func DetermineProviderAndModelName(model string) (provider *Provider, modelName string) {
	modelLower := strings.ToLower(model)

	providerPrefixMapping := map[string]Provider{
		"ollama/":       OllamaID,
		"ollama_cloud/": OllamaCloudID,
		"groq/":         GroqID,
		"cloudflare/":   CloudflareID,
		"openai/":       OpenaiID,
		"anthropic/":    AnthropicID,
		"cohere/":       CohereID,
		"deepseek/":     DeepseekID,
		"google/":       GoogleID,
		"mistral/":      MistralID,
		"moonshot/":     MoonshotID,
	}

	for prefix, providerID := range providerPrefixMapping {
		if strings.HasPrefix(modelLower, prefix) {
			originalPrefix := model[:len(prefix)]
			return &providerID, strings.TrimPrefix(model, originalPrefix)
		}
	}

	return nil, model
}
