package core

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// communityPricingJSON is the fallback pricing table synced from the
// community-maintained models.dev dataset via `task pricing:sync`
// (internal/pricinggen), keyed by "<provider>/<model>".
//
//go:embed community_pricing.json
var communityPricingJSON []byte

// communityPricing lazily parses the embedded table once. The file is
// generated and committed, so a parse failure is a build defect, not a
// runtime condition; it degrades to an empty table (pricing stays null).
var communityPricing = sync.OnceValue(func() map[string]types.Pricing {
	table := make(map[string]types.Pricing)
	_ = json.Unmarshal(communityPricingJSON, &table)
	return table
})

// applyCommunityPricing fills Pricing from the community table for models the
// provider did not price itself, so provider-published rates always win.
// Models absent from the table (local providers, paid gates with no per-token
// rate) keep a nil Pricing and render as explicit nulls when requested.
func applyCommunityPricing(models []types.Model) {
	table := communityPricing()
	for i := range models {
		if models[i].Pricing != nil {
			continue
		}
		for _, key := range communityLookupKeys(models[i].ID) {
			if pricing, ok := table[key]; ok {
				models[i].Pricing = &pricing
				break
			}
		}
	}
}

// communityLookupKeys returns candidate table keys for a gateway model ID in
// preference order: exact, without Google's "models/" path prefix, without a
// "-latest" alias suffix, and without a trailing "-YYYYMMDD" date pin, so
// upstream listing IDs still match the dataset's canonical names. Each
// candidate containing dots also gets an underscored variant, because
// models.dev file names replace dots in model ids (e.g. NIM's
// "solar-10.7b-instruct" is committed as "solar-10_7b-instruct").
func communityLookupKeys(id string) []string {
	keys := []string{id}
	provider, model, ok := strings.Cut(id, "/")
	if !ok {
		return keys
	}
	if rest, found := strings.CutPrefix(model, "models/"); found {
		model = rest
		keys = append(keys, provider+"/"+model)
	}
	if rest, found := strings.CutSuffix(model, "-latest"); found {
		keys = append(keys, provider+"/"+rest)
	}
	if len(model) > 9 && model[len(model)-9] == '-' && isDigits(model[len(model)-8:]) {
		keys = append(keys, provider+"/"+model[:len(model)-9])
	}
	for _, key := range keys {
		if strings.Contains(key, ".") {
			keys = append(keys, strings.ReplaceAll(key, ".", "_"))
		}
	}
	return keys
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
