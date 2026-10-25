package transformers

import (
	"cmp"
	"math"
	"strconv"
	"time"

	modelsdev "github.com/inference-gateway/inference-gateway/providers/modelsdev"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// modelObject is the OpenAI object type of a model listing entry.
const modelObject = "model"

// pricingCurrency is the currency of every per-token rate the gateway renders.
const pricingCurrency = "USD"

// listedModel is one entry of an OpenAI-shaped models listing, plus fields some
// providers publish beyond it: an RFC3339 created_at (Anthropic), a context
// window under one of several names (Groq, Cohere, Mistral, vLLM), and
// OpenRouter-style pricing. Its tags shadow the embedded context_window and pricing.
type listedModel struct {
	types.Model
	CreatedAt           string         `json:"created_at"`
	ContextWindowTokens float64        `json:"context_window"`
	ContextLength       float64        `json:"context_length"`
	MaxContextLength    float64        `json:"max_context_length"`
	MaxModelLen         float64        `json:"max_model_len"`
	ProviderPricing     map[string]any `json:"pricing"`
}

// toModels renders an OpenAI-shaped listing as gateway models served by
// provider. A created stamp shared by every entry of a multi-model listing is a
// placeholder, not a creation date (NVIDIA stamps every model 1993-04-26), so
// it is dropped and the models.dev release date fills in.
// ponytail: identical-stamp rule only, add a near-now check if a provider stamps each entry with the request time.
func toModels(provider types.Provider, listed []listedModel) []types.Model {
	placeholder := sharedCreated(listed)
	models := make([]types.Model, len(listed))
	for i, entry := range listed {
		if entry.Created == placeholder {
			entry.Created = 0
		}
		models[i] = entry.toModel(provider)
	}
	return models
}

// sharedCreated returns the created stamp every entry carries, or 0 when the
// listing has fewer than two entries or their stamps differ.
func sharedCreated(listed []listedModel) int64 {
	if len(listed) < 2 {
		return 0
	}
	for _, entry := range listed[1:] {
		if entry.Created != listed[0].Created {
			return 0
		}
	}
	return listed[0].Created
}

// toModel renders the entry as a gateway model served by provider. Each field
// the upstream left empty falls back to the models.dev table, so
// provider-published values always win.
func (m listedModel) toModel(provider types.Provider) types.Model {
	model := m.Model
	model.ID = string(provider) + "/" + model.ID
	model.ServedBy = provider
	model.Object = cmp.Or(model.Object, modelObject)
	model.OwnedBy = cmp.Or(model.OwnedBy, string(provider))
	known := modelsdev.Lookup(model.ID)
	model.Created = cmp.Or(model.Created, unixFromRFC3339(m.CreatedAt), known.Created)
	model.ContextWindow = cmp.Or(m.providerContextWindow(), known.ContextWindow)
	model.Pricing = cmp.Or(m.providerPricing(), known.Pricing)
	model.Modalities = cmp.Or(model.Modalities, known.Modalities)
	return model
}

// providerContextWindow returns the first usable context window the provider
// published, or nil when it published none.
func (m listedModel) providerContextWindow() *types.ContextWindow {
	for _, tokens := range []float64{m.ContextWindowTokens, m.ContextLength, m.MaxContextLength, m.MaxModelLen} {
		if tokens > 0 && tokens < math.MaxInt {
			return &types.ContextWindow{Tokens: int(tokens), Source: types.ContextWindowSourceProvider}
		}
	}
	return nil
}

// providerPricing normalizes a provider-published pricing object to the
// gateway's per-token fields, or returns nil unless both input and output
// rates are published.
func (m listedModel) providerPricing() *types.Pricing {
	input := pricingRate(m.ProviderPricing, "input_per_token", "prompt", "input")
	output := pricingRate(m.ProviderPricing, "output_per_token", "completion", "output")
	if input == nil || output == nil {
		return nil
	}
	return &types.Pricing{
		Currency:           pricingCurrency,
		InputPerToken:      *input,
		OutputPerToken:     *output,
		CacheReadPerToken:  pricingRate(m.ProviderPricing, "cache_read_per_token", "input_cache_read", "cached"),
		CacheWritePerToken: pricingRate(m.ProviderPricing, "cache_write_per_token", "input_cache_write", "cache_creation"),
		Source:             types.PricingSourceProvider,
		UpdatedAt:          time.Now().UTC().Truncate(time.Second),
	}
}

// pricingRate returns the first published, positive per-token rate among the
// given keys as a decimal string. Rates arrive as decimal strings or numbers.
// Zero and negative values mean "not applicable" and count as unpublished.
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

// unixFromRFC3339 returns the timestamp in Unix seconds, or 0 when it is empty
// or malformed.
func unixFromRFC3339(timestamp string) int64 {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	return parsed.Unix()
}
