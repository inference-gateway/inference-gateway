// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// NewListModelsTransformer returns the list-models transformer for the given
// provider, falling back to the OpenAI-compatible transformer.
func NewListModelsTransformer(provider types.Provider) constants.ListModelsTransformer {
	switch provider {
	case constants.AnthropicID:
		return &ListModelsResponseAnthropic{}
	case constants.CloudflareID:
		return &ListModelsResponseCloudflare{}
	case constants.CohereID:
		return &ListModelsResponseCohere{}
	case constants.DeepseekID:
		return &ListModelsResponseDeepseek{}
	case constants.GoogleID:
		return &ListModelsResponseGoogle{}
	case constants.GroqID:
		return &ListModelsResponseGroq{}
	case constants.LlamacppID:
		return &ListModelsResponseLlamacpp{}
	case constants.MinimaxID:
		return &ListModelsResponseMinimax{}
	case constants.MistralID:
		return &ListModelsResponseMistral{}
	case constants.MoonshotID:
		return &ListModelsResponseMoonshot{}
	case constants.NvidiaID:
		return &ListModelsResponseNvidia{}
	case constants.OllamaID:
		return &ListModelsResponseOllama{}
	case constants.OllamaCloudID:
		return &ListModelsResponseOllamaCloud{}
	case constants.OpenaiID:
		return &ListModelsResponseOpenai{}
	case constants.ZaiID:
		return &ListModelsResponseZai{}
	default:
		return &ListModelsResponseOpenai{}
	}
}
