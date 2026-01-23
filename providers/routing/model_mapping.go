package routing

import (
	"strings"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// DetermineProviderAndModelName analyzes a model string and tries to determine
// the provider based on explicit naming conventions only. It returns both the detected provider
// and the model name (which might be modified to strip provider prefixes).
//
// It only checks for explicit provider prefixes like "ollama/", "groq/", etc.
// Implicit model name-based routing (like "gpt-" -> OpenAI) is not supported.
//
// Returns nil provider if no explicit provider prefix is found.
// In such cases, the provider must be specified via query parameter.
func DetermineProviderAndModelName(model string) (provider *types.Provider, modelName string) {
	modelLower := strings.ToLower(model)

	providerPrefixMapping := map[string]types.Provider{
		"ollama/":       constants.OllamaID,
		"ollama_cloud/": constants.OllamaCloudID,
		"groq/":         constants.GroqID,
		"cloudflare/":   constants.CloudflareID,
		"openai/":       constants.OpenaiID,
		"anthropic/":    constants.AnthropicID,
		"cohere/":       constants.CohereID,
		"deepseek/":     constants.DeepseekID,
		"google/":       constants.GoogleID,
		"mistral/":      constants.MistralID,
		"moonshot/":     constants.MoonshotID,
	}

	for prefix, providerID := range providerPrefixMapping {
		if strings.HasPrefix(modelLower, prefix) {
			originalPrefix := model[:len(prefix)]
			return &providerID, strings.TrimPrefix(model, originalPrefix)
		}
	}

	return nil, model
}
