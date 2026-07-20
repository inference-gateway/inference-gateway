package core

import (
	"encoding/json"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// providerContextWindowKeys are field names upstream providers use to publish a
// model's context window in their model listings (e.g. Cohere context_length,
// Mistral max_context_length, vLLM max_model_len).
var providerContextWindowKeys = []string{"context_window", "context_length", "max_context_length", "max_model_len"}

// applyProviderContextWindows scans the raw upstream models payload for
// published context-window fields and fills ContextWindow (source: provider) on
// the transformed models. Entries are matched to models by position, so it
// bails unless both lists have the same length. A runtime-sourced value set
// later overrides these.
func applyProviderContextWindows(raw []byte, models []types.Model) {
	var payload struct {
		Data   []map[string]any `json:"data"`
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	entries := payload.Data
	if entries == nil {
		entries = payload.Models
	}
	if len(entries) != len(models) {
		return
	}

	for i, entry := range entries {
		if models[i].ContextWindow != nil {
			continue
		}
		for _, key := range providerContextWindowKeys {
			tokens, ok := entry[key].(float64)
			if !ok || tokens <= 0 {
				continue
			}
			models[i].ContextWindow = &types.ModelContextWindow{
				Tokens: int64(tokens),
				Source: types.ContextWindowSourceProvider,
			}
			break
		}
	}
}
