package routing

import (
	"strings"

	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ProviderHintFormat is the client-facing sentence explaining how to select a
// provider. The single verb is an example model, e.g. "openai/gpt-4".
const ProviderHintFormat = "Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., %s)."

// ResolveProvider applies the gateway's provider precedence rule: an explicit
// ?provider= value wins, otherwise the model's "provider/" prefix. It returns
// the provider, the model with the prefix stripped, and false when neither is
// present.
func ResolveProvider(queryParam, model string) (provider types.Provider, modelName string, ok bool) {
	if queryParam != "" {
		return types.Provider(queryParam), model, true
	}

	if detected, stripped := DetermineProviderAndModelName(model); detected != nil {
		return *detected, stripped, true
	}

	return "", model, false
}

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
	prefix, rest, ok := strings.Cut(model, "/")
	if !ok {
		return nil, model
	}

	id := types.Provider(strings.ToLower(prefix))
	if _, exists := registry.Registry[id]; !exists {
		return nil, model
	}

	return &id, rest
}
