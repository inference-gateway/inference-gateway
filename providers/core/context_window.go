package core

import (
	"encoding/json"
	"math"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// providerContextWindowKeys are field names upstream providers use to publish a
// model's context window in their model listings (e.g. Cohere context_length,
// Mistral max_context_length, vLLM max_model_len).
var providerContextWindowKeys = []string{"context_window", "context_length", "max_context_length", "max_model_len"}

// modelEntries decodes the raw upstream models payload into one map per model
// entry. Entries are matched to transformed models by position, so it returns
// nil unless the payload holds exactly want entries.
func modelEntries(raw []byte, want int) []map[string]any {
	var payload struct {
		Data   []map[string]any `json:"data"`
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	entries := payload.Data
	if entries == nil {
		entries = payload.Models
	}
	if len(entries) != want {
		return nil
	}
	return entries
}

// applyProviderContextWindows scans the raw upstream models payload for
// published context-window fields and fills ContextWindow (source: provider) on
// the transformed models. A runtime-sourced value set later overrides these.
func applyProviderContextWindows(raw []byte, models []types.Model) {
	for i, entry := range modelEntries(raw, len(models)) {
		if models[i].ContextWindow != nil {
			continue
		}
		for _, key := range providerContextWindowKeys {
			tokens, ok := entry[key].(float64)
			if !ok || tokens <= 0 || tokens >= math.MaxInt {
				continue
			}
			models[i].ContextWindow = &types.ContextWindow{
				Tokens: int(tokens),
				Source: types.ContextWindowSourceProvider,
			}
			break
		}
	}
}
