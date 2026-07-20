package core

import (
	"strconv"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// applyProviderPricing scans the raw upstream models payload for a published
// per-token pricing object (e.g. OpenRouter-style `pricing` with
// prompt/completion rates) and fills Pricing (source: provider) on the
// transformed models, normalized to Gateway field names. Entries are matched
// to models by position, so it bails unless both lists have the same length.
// Models whose provider publishes no pricing are left nil; the render step
// turns them into explicit nulls when pricing is requested.
func applyProviderPricing(raw []byte, models []types.Model) {
	entries := modelEntries(raw, len(models))
	for i := range models {
		models[i].Pricing = nil
		if entries == nil {
			continue
		}
		pricing, ok := entries[i]["pricing"].(map[string]any)
		if !ok {
			continue
		}

		input := pricingRate(pricing, "input_per_token", "prompt", "input")
		output := pricingRate(pricing, "output_per_token", "completion", "output")
		if input == nil && output == nil {
			continue
		}
		models[i].Pricing = &types.ModelPricing{
			Currency:           "USD",
			InputPerToken:      input,
			OutputPerToken:     output,
			CacheReadPerToken:  pricingRate(pricing, "cache_read_per_token", "input_cache_read", "cached"),
			CacheWritePerToken: pricingRate(pricing, "cache_write_per_token", "input_cache_write", "cache_creation"),
			Source:             types.PricingSourceProvider,
		}
	}
}

// pricingRate returns the first published, positive per-token rate among the
// given keys as a decimal string. Rates arrive as decimal strings
// (OpenRouter-style) or numbers; zero and negative values mean "not
// applicable" and count as unpublished, so cache rates of "0" are omitted
// rather than rendered.
func pricingRate(pricing map[string]any, keys ...string) *string {
	for _, key := range keys {
		switch v := pricing[key].(type) {
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				return &v
			}
		case float64:
			if v > 0 {
				s := strconv.FormatFloat(v, 'f', -1, 64)
				return &s
			}
		}
	}
	return nil
}
