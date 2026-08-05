package core

import (
	_ "embed"
	"encoding/json"
	"slices"
	"sync"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// communityModalitiesJSON is the fallback modalities table synced from the
// community-maintained models.dev dataset via `task modalities:sync`
// (internal/communitygen), keyed by "<provider>/<model>".
//
//go:embed community_modalities.json
var communityModalitiesJSON []byte

// communityModalities lazily parses the embedded table once. The file is
// generated and committed, so a parse failure is a build defect, not a runtime
// condition; it degrades to an empty table (modalities stay null).
var communityModalities = sync.OnceValue(func() map[string]types.ModelModalities {
	table := make(map[string]types.ModelModalities)
	_ = json.Unmarshal(communityModalitiesJSON, &table)
	return table
})

// applyCommunityModalities fills Modalities from the community table for models
// the provider listing did not resolve, so provider-published modalities always
// win. Models absent from the table keep a nil Modalities and render as explicit
// nulls when requested. The stored slices are cloned so a caller mutating one
// model's modalities can never corrupt the shared embedded table.
func applyCommunityModalities(models []types.Model) {
	table := communityModalities()
	for i := range models {
		if models[i].Modalities != nil {
			continue
		}
		for _, key := range communityLookupKeys(models[i].ID) {
			if mods, ok := table[key]; ok {
				clone := types.ModelModalities{
					Input:  slices.Clone(mods.Input),
					Output: slices.Clone(mods.Output),
				}
				models[i].Modalities = &clone
				break
			}
		}
	}
}
