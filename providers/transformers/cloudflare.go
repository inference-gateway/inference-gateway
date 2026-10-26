package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ListModelsResponseCloudflare decodes Cloudflare's {"result":[...]} listing
// besides the OpenAI shape mocks serve. Workers AI has no OpenAI-compatible
// GET /v1/models (every GET under /ai/v1 answers 7001), and its result entries
// are named by name with an opaque UUID id, so they are read like Cohere's
// models entries (see .openapi-ignore).
type ListModelsResponseCloudflare struct {
	modelListing
	Result []listedModel `json:"result"`
}

func (l *ListModelsResponseCloudflare) Transform() types.ListModelsResponse {
	provider := constants.CloudflareID
	l.Models = l.Result
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     l.models(provider),
	}
}
