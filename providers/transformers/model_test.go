package transformers

import (
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	claudeHaikuCreatedAt  int64 = 1791396000
	gpt4oCreated          int64 = 1715367049
	gemini25ProReleased   int64 = 1750118400
	commandA032025Release int64 = 1741824000
	gemma312bReleased     int64 = 1741737600
	deepseekFlashReleased int64 = 1792195200

	cloudflareFinetuneCreated int64 = 1577470279

	deepseekFlashContextWindow        = 1048576
	commandA032025ContextWindow       = 288000
	glmACreated                 int64 = 1789009200
	glmBCreated                 int64 = 1785164400

	groqLlamaContextWindow = 131072
)

// TestTransformFillsCreated decodes raw upstream listings and checks created
// resolves in order: upstream created, then RFC3339 created_at, then the
// models.dev release date. Empty object and owned_by get the OpenAI defaults.
func TestTransformFillsCreated(t *testing.T) {
	tests := []struct {
		name        string
		provider    types.Provider
		body        string
		wantID      string
		wantCreated int64
		wantOwnedBy string
	}{
		{
			name:        "anthropic created_at",
			provider:    constants.AnthropicID,
			body:        `{"data":[{"type":"model","id":"claude-haiku-5-5","display_name":"Claude Haiku 5.5","created_at":"2026-10-07T18:00:00Z"}],"has_more":false}`,
			wantID:      "anthropic/claude-haiku-5-5",
			wantCreated: claudeHaikuCreatedAt,
			wantOwnedBy: string(constants.AnthropicID),
		},
		{
			name:        "upstream created wins",
			provider:    constants.OpenaiID,
			body:        `{"object":"list","data":[{"id":"gpt-4o","object":"model","created":1715367049,"owned_by":"system"}]}`,
			wantID:      "openai/gpt-4o",
			wantCreated: gpt4oCreated,
			wantOwnedBy: "system",
		},
		{
			name:        "google release date through models/ prefix",
			provider:    constants.GoogleID,
			body:        `{"data":[{"id":"models/gemini-2.5-pro"}]}`,
			wantID:      "google/models/gemini-2.5-pro",
			wantCreated: gemini25ProReleased,
			wantOwnedBy: string(constants.GoogleID),
		},
		{
			name:        "cohere release date",
			provider:    constants.CohereID,
			body:        `{"models":[{"name":"command-a-03-2025","context_length":288000}]}`,
			wantID:      "cohere/command-a-03-2025",
			wantCreated: commandA032025Release,
			wantOwnedBy: string(constants.CohereID),
		},
		{
			name:        "unknown model stays zero",
			provider:    constants.DeepseekID,
			body:        `{"object":"list","data":[{"id":"no-such-model"}]}`,
			wantID:      "deepseek/no-such-model",
			wantOwnedBy: string(constants.DeepseekID),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := transformOne(t, tt.provider, tt.body)
			assert.Equal(t, tt.wantID, model.ID)
			assert.Equal(t, tt.wantCreated, model.Created)
			assert.Equal(t, modelObject, model.Object)
			assert.Equal(t, tt.wantOwnedBy, model.OwnedBy)
			assert.Equal(t, tt.provider, model.ServedBy)
		})
	}
}

// TestTransformDropsPlaceholderCreated checks a created stamp shared by every
// entry of a listing (NVIDIA's 1993-04-26) is treated as a placeholder: known
// models get their models.dev release date, unknown ones stay 0, while a
// listing with distinct stamps keeps them.
func TestTransformDropsPlaceholderCreated(t *testing.T) {
	transformer := NewListModelsTransformer(constants.NvidiaID)
	require.NoError(t, json.Unmarshal([]byte(`{"object":"list","data":[{"id":"google/gemma-3-12b-it","created":735790403},{"id":"acme/no-such-model","created":735790403}]}`), transformer))
	placeholder := transformer.Transform().Data
	require.Len(t, placeholder, 2)
	assert.Equal(t, gemma312bReleased, placeholder[0].Created)
	assert.Zero(t, placeholder[1].Created)

	transformer = NewListModelsTransformer(constants.OllamaCloudID)
	require.NoError(t, json.Unmarshal([]byte(`{"object":"list","data":[{"id":"glm-a","created":1789009200},{"id":"glm-b","created":1785164400}]}`), transformer))
	distinct := transformer.Transform().Data
	require.Len(t, distinct, 2)
	assert.Equal(t, glmACreated, distinct[0].Created)
	assert.Equal(t, glmBCreated, distinct[1].Created)
}

// TestTransformDecodesEveryListingEnvelope checks the generated transformers
// read each envelope providers answer with: OpenAI data entries keep their id
// even beside a display name (DeepSeek), Cohere's models envelope and
// Cloudflare's result envelope name entries by name, never by Cloudflare's UUID.
func TestTransformDecodesEveryListingEnvelope(t *testing.T) {
	tests := []struct {
		name        string
		provider    types.Provider
		body        string
		wantID      string
		wantCreated int64
		wantContext *types.ContextWindow
	}{
		{
			name:     "openai data envelope with display name",
			provider: constants.DeepseekID,
			body:     `{"object":"list","data":[{"id":"deepseek-flash","object":"model","owned_by":"deepseek","name":"DeepSeek-V4.1-Flash","context_window":1048576}]}`,
			wantID:   "deepseek/deepseek-flash",
			wantContext: &types.ContextWindow{
				Tokens: deepseekFlashContextWindow,
				Source: types.ContextWindowSourceProvider,
			},
			wantCreated: deepseekFlashReleased,
		},
		{
			name:     "cohere models envelope",
			provider: constants.CohereID,
			body:     `{"models":[{"name":"command-a-03-2025","endpoints":["chat"],"context_length":288000}],"next_page_token":""}`,
			wantID:   "cohere/command-a-03-2025",
			wantContext: &types.ContextWindow{
				Tokens: commandA032025ContextWindow,
				Source: types.ContextWindowSourceProvider,
			},
			wantCreated: commandA032025Release,
		},
		{
			name:        "cloudflare result envelope",
			provider:    constants.CloudflareID,
			body:        `{"result":[{"id":"182bd5e5-6e1a-4fe4-a799-aa6d9a6ab26e","name":"cf-public-magicoder","model":"@cf/deepseek-ai/deepseek-coder-6.7b-base-awq","created_at":"2019-12-27T18:11:19.117Z","modified_at":"2019-12-27T18:11:19.117Z","public":true}],"success":true,"errors":[],"messages":[]}`,
			wantID:      "cloudflare/cf-public-magicoder",
			wantCreated: cloudflareFinetuneCreated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := transformOne(t, tt.provider, tt.body)
			assert.Equal(t, tt.wantID, model.ID)
			assert.Equal(t, tt.wantCreated, model.Created)
			if tt.wantContext != nil {
				assert.Equal(t, tt.wantContext, model.ContextWindow)
			}
			assert.Equal(t, modelObject, model.Object)
			assert.Equal(t, tt.provider, model.ServedBy)
		})
	}

	empty := NewListModelsTransformer(constants.CloudflareID)
	require.NoError(t, json.Unmarshal([]byte(`{"result":[],"success":true}`), empty))
	assert.Empty(t, empty.Transform().Data)
}

// TestTransformPrefersProviderMetadata checks provider-published context
// windows, pricing and modalities win over the models.dev table, while a model
// that publishes none falls back to models.dev with source community.
func TestTransformPrefersProviderMetadata(t *testing.T) {
	provided := transformOne(t, constants.GroqID, `{"data":[{"id":"llama-3.3-70b-versatile","context_window":131072,"pricing":{"prompt":"0.00000059","completion":"0.00000079"},"modalities":{"input":["text"],"output":["text"]}}]}`)
	assert.Equal(t, &types.ContextWindow{Tokens: groqLlamaContextWindow, Source: types.ContextWindowSourceProvider}, provided.ContextWindow)
	require.NotNil(t, provided.Pricing)
	assert.Equal(t, types.PricingSourceProvider, provided.Pricing.Source)
	assert.Equal(t, "0.00000059", provided.Pricing.InputPerToken)
	assert.Equal(t, &types.ModelModalities{Input: []types.Modality{types.ModalityText}, Output: []types.Modality{types.ModalityText}}, provided.Modalities)

	fallback := transformOne(t, constants.OpenaiID, `{"data":[{"id":"gpt-4o"}]}`)
	require.NotNil(t, fallback.ContextWindow)
	assert.Equal(t, types.ContextWindowSourceCommunity, fallback.ContextWindow.Source)
	require.NotNil(t, fallback.Pricing)
	assert.Equal(t, types.PricingSourceCommunity, fallback.Pricing.Source)
	require.NotNil(t, fallback.Modalities)
	assert.Contains(t, fallback.Modalities.Input, types.ModalityImage)

	unknown := transformOne(t, constants.OpenaiID, `{"data":[{"id":"gpt-nonexistent"}]}`)
	assert.Nil(t, unknown.ContextWindow)
	assert.Nil(t, unknown.Pricing)
	assert.Nil(t, unknown.Modalities)
}

// transformOne decodes a raw single-model listing through the provider's
// transformer and returns the transformed model.
func transformOne(t *testing.T, provider types.Provider, body string) types.Model {
	t.Helper()

	transformer := NewListModelsTransformer(provider)
	require.NoError(t, json.Unmarshal([]byte(body), transformer))
	resp := transformer.Transform()
	require.Len(t, resp.Data, 1)
	return resp.Data[0]
}
