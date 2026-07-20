package core

import (
	_ "embed"
	"encoding/json"
	"sync"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// communityContextWindowsJSON is the fallback context-window table synced from
// the community-maintained models.dev dataset via `task contextwindow:sync`
// (internal/pricinggen), keyed by "<provider>/<model>".
//
//go:embed community_context_windows.json
var communityContextWindowsJSON []byte

// communityContextWindow is one committed table entry: the model's context
// window in tokens and, when published, its maximum output tokens. Only the
// context window is surfaced through the API today.
type communityContextWindow struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output"`
}

// communityContextWindows lazily parses the embedded table once. The file is
// generated and committed, so a parse failure is a build defect, not a runtime
// condition; it degrades to an empty table (context windows stay null).
var communityContextWindows = sync.OnceValue(func() map[string]communityContextWindow {
	table := make(map[string]communityContextWindow)
	_ = json.Unmarshal(communityContextWindowsJSON, &table)
	return table
})

// applyCommunityContextWindows fills ContextWindow from the community table
// for models the provider listing did not resolve, so provider-published
// windows always win; a runtime lookup later still overrides both. Models
// absent from the table (local runtimes by design) keep a nil ContextWindow
// and render as explicit nulls when requested.
func applyCommunityContextWindows(models []types.Model) {
	table := communityContextWindows()
	for i := range models {
		if models[i].ContextWindow != nil {
			continue
		}
		for _, key := range communityLookupKeys(models[i].ID) {
			if entry, ok := table[key]; ok {
				models[i].ContextWindow = &types.ModelContextWindow{
					Tokens: entry.Context,
					Source: types.ContextWindowSourceCommunity,
				}
				break
			}
		}
	}
}
