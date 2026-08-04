// Code generated from OpenAPI schema. DO NOT EDIT.
package registry

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// The registry of all providers
var Registry = map[types.Provider]*ProviderConfig{
	constants.AnthropicID: {
		ID:             constants.AnthropicID,
		Name:           constants.AnthropicDisplayName,
		URL:            constants.AnthropicDefaultBaseURL,
		AuthType:       constants.AuthTypeXheader,
		SupportsVision: false,
		ExtraHeaders: map[string][]string{
			"anthropic-version": {"2023-06-01"},
		},
		Endpoints: types.Endpoints{
			Models: constants.AnthropicModelsEndpoint,
			Chat:   constants.AnthropicChatEndpoint,
		},
	},
	constants.CloudflareID: {
		ID:             constants.CloudflareID,
		Name:           constants.CloudflareDisplayName,
		URL:            constants.CloudflareDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.CloudflareModelsEndpoint,
			Chat:   constants.CloudflareChatEndpoint,
		},
	},
	constants.CohereID: {
		ID:             constants.CohereID,
		Name:           constants.CohereDisplayName,
		URL:            constants.CohereDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.CohereModelsEndpoint,
			Chat:   constants.CohereChatEndpoint,
		},
	},
	constants.DeepseekID: {
		ID:             constants.DeepseekID,
		Name:           constants.DeepseekDisplayName,
		URL:            constants.DeepseekDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.DeepseekModelsEndpoint,
			Chat:   constants.DeepseekChatEndpoint,
		},
	},
	constants.GoogleID: {
		ID:             constants.GoogleID,
		Name:           constants.GoogleDisplayName,
		URL:            constants.GoogleDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.GoogleModelsEndpoint,
			Chat:   constants.GoogleChatEndpoint,
		},
	},
	constants.GroqID: {
		ID:             constants.GroqID,
		Name:           constants.GroqDisplayName,
		URL:            constants.GroqDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.GroqModelsEndpoint,
			Chat:   constants.GroqChatEndpoint,
		},
	},
	constants.LlamacppID: {
		ID:             constants.LlamacppID,
		Name:           constants.LlamacppDisplayName,
		URL:            constants.LlamacppDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.LlamacppModelsEndpoint,
			Chat:   constants.LlamacppChatEndpoint,
		},
	},
	constants.MinimaxID: {
		ID:             constants.MinimaxID,
		Name:           constants.MinimaxDisplayName,
		URL:            constants.MinimaxDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.MinimaxModelsEndpoint,
			Chat:   constants.MinimaxChatEndpoint,
		},
	},
	constants.MistralID: {
		ID:             constants.MistralID,
		Name:           constants.MistralDisplayName,
		URL:            constants.MistralDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.MistralModelsEndpoint,
			Chat:   constants.MistralChatEndpoint,
		},
	},
	constants.MoonshotID: {
		ID:             constants.MoonshotID,
		Name:           constants.MoonshotDisplayName,
		URL:            constants.MoonshotDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.MoonshotModelsEndpoint,
			Chat:   constants.MoonshotChatEndpoint,
		},
	},
	constants.NvidiaID: {
		ID:             constants.NvidiaID,
		Name:           constants.NvidiaDisplayName,
		URL:            constants.NvidiaDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.NvidiaModelsEndpoint,
			Chat:   constants.NvidiaChatEndpoint,
		},
	},
	constants.OllamaID: {
		ID:             constants.OllamaID,
		Name:           constants.OllamaDisplayName,
		URL:            constants.OllamaDefaultBaseURL,
		AuthType:       constants.AuthTypeNone,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.OllamaModelsEndpoint,
			Chat:   constants.OllamaChatEndpoint,
		},
	},
	constants.OllamaCloudID: {
		ID:             constants.OllamaCloudID,
		Name:           constants.OllamaCloudDisplayName,
		URL:            constants.OllamaCloudDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.OllamaCloudModelsEndpoint,
			Chat:   constants.OllamaCloudChatEndpoint,
		},
	},
	constants.OpenaiID: {
		ID:             constants.OpenaiID,
		Name:           constants.OpenaiDisplayName,
		URL:            constants.OpenaiDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models:           constants.OpenaiModelsEndpoint,
			Chat:             constants.OpenaiChatEndpoint,
			Responses:        ptr(constants.OpenaiResponsesEndpoint),
			Images:           ptr(constants.OpenaiImagesEndpoint),
			ImagesEdits:      ptr(constants.OpenaiImagesEditsEndpoint),
			ImagesVariations: ptr(constants.OpenaiImagesVariationsEndpoint),
		},
	},
	constants.ZaiID: {
		ID:             constants.ZaiID,
		Name:           constants.ZaiDisplayName,
		URL:            constants.ZaiDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.ZaiModelsEndpoint,
			Chat:   constants.ZaiChatEndpoint,
		},
	},
}
