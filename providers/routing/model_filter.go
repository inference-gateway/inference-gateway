package routing

import (
	"strings"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ParseModelSet parses a comma-separated model list into a lowercase lookup set.
func ParseModelSet(csv string) map[string]bool {
	set := make(map[string]bool)
	for entry := range strings.SplitSeq(csv, ",") {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			set[strings.ToLower(trimmed)] = true
		}
	}
	return set
}

// ModelMatches reports whether modelID matches the set, comparing both the
// full id and the provider-stripped model name case-insensitively.
func ModelMatches(set map[string]bool, modelID string) bool {
	id := strings.ToLower(modelID)
	if set[id] {
		return true
	}
	if _, name, ok := strings.Cut(id, "/"); ok && set[name] {
		return true
	}
	return false
}

// FilterModels applies the ALLOWED_MODELS / DISALLOWED_MODELS semantics: a
// non-empty allow list wins over the deny list; empty lists pass everything.
func FilterModels(models []types.Model, allowedModels, disallowedModels string) []types.Model {
	if allowedModels != "" {
		allowed := ParseModelSet(allowedModels)
		if len(allowed) == 0 {
			return models
		}
		filtered := make([]types.Model, 0)
		for _, model := range models {
			if ModelMatches(allowed, model.ID) {
				filtered = append(filtered, model)
			}
		}
		return filtered
	}

	if disallowedModels != "" {
		disallowed := ParseModelSet(disallowedModels)
		if len(disallowed) == 0 {
			return models
		}
		filtered := make([]types.Model, 0)
		for _, model := range models {
			if !ModelMatches(disallowed, model.ID) {
				filtered = append(filtered, model)
			}
		}
		return filtered
	}

	return models
}
