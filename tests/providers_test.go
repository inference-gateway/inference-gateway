package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	transformers "github.com/inference-gateway/inference-gateway/providers/transformers"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestProviderRegistry tests the provider registry functionality
func TestProviderRegistry(t *testing.T) {
	log, err := logger.NewLogger("test")
	assert.NoError(t, err)

	cfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:       constants.OpenaiID,
			Name:     constants.OpenaiDisplayName,
			URL:      constants.OpenaiDefaultBaseURL,
			Token:    "test-token",
			AuthType: constants.AuthTypeBearer,
			Endpoints: types.Endpoints{
				Models: constants.OpenaiModelsEndpoint,
				Chat:   constants.OpenaiChatEndpoint,
			},
		},
		constants.MistralID: {
			ID:       constants.MistralID,
			Name:     constants.MistralDisplayName,
			URL:      constants.MistralDefaultBaseURL,
			Token:    "test-token",
			AuthType: constants.AuthTypeBearer,
			Endpoints: types.Endpoints{
				Models: constants.MistralModelsEndpoint,
				Chat:   constants.MistralChatEndpoint,
			},
		},
		constants.OllamaID: {
			ID:       constants.OllamaID,
			Name:     constants.OllamaDisplayName,
			URL:      constants.OllamaDefaultBaseURL,
			AuthType: constants.AuthTypeNone,
			Endpoints: types.Endpoints{
				Models: constants.OllamaModelsEndpoint,
				Chat:   constants.OllamaChatEndpoint,
			},
		},
	}

	providerRegistry := registry.NewProviderRegistry(cfg, log)

	providerConfigs := providerRegistry.GetProviders()
	assert.Equal(t, len(cfg), len(providerConfigs))
	assert.Equal(t, cfg[constants.OpenaiID], providerConfigs[constants.OpenaiID])
	assert.Equal(t, cfg[constants.MistralID], providerConfigs[constants.MistralID])
	assert.Equal(t, cfg[constants.OllamaID], providerConfigs[constants.OllamaID])

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := providersmocks.NewMockClient(ctrl)

	openaiProvider, err := providerRegistry.BuildProvider(constants.OpenaiID, mockClient)
	require.NoError(t, err)
	assert.Equal(t, constants.OpenaiID, *openaiProvider.GetID())
	assert.Equal(t, constants.OpenaiDisplayName, openaiProvider.GetName())
	assert.Equal(t, constants.OpenaiDefaultBaseURL, openaiProvider.GetURL())
	assert.Equal(t, "test-token", openaiProvider.GetToken())
	assert.Equal(t, constants.AuthTypeBearer, openaiProvider.GetAuthType())

	_, err = providerRegistry.BuildProvider("invalid-provider", mockClient)
	assert.Error(t, err)
}

// TestProviderChatCompletions tests chat completions functionality
func TestProviderChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/proxy/openai/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
            "id": "test-completion-id",
            "object": "chat.completion",
            "created": 1677858242,
            "model": "gpt-3.5-turbo",
            "choices": [
                {
                    "message": {
                        "role": "assistant",
                        "content": "This is a test response."
                    },
                    "finish_reason": "stop",
                    "index": 0
                }
            ]
        }`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := providersmocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "POST", req.Method)
			assert.Contains(t, req.URL.Path, "/chat/completions")
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

			return http.DefaultClient.Post(server.URL+"/proxy/openai/chat/completions", "application/json", nil)
		})

	log, err := logger.NewLogger("test")
	assert.NoError(t, err)

	config := &registry.ProviderConfig{
		ID:       constants.OpenaiID,
		Name:     constants.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: constants.AuthTypeBearer,
		Endpoints: types.Endpoints{
			Chat: constants.OpenaiChatEndpoint,
		},
	}

	providerRegistry := registry.NewProviderRegistry(map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: config,
	}, log)

	provider, err := providerRegistry.BuildProvider(constants.OpenaiID, mockClient)
	assert.NoError(t, err)

	msg := types.NewTextMessage(t, types.User, "Hello, how are you?")
	require.NoError(t, err)

	req := types.CreateChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.Message{
			msg,
		},
	}

	resp, err := provider.ChatCompletions(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "test-completion-id", resp.ID)
	assert.Equal(t, "gpt-3.5-turbo", resp.Model)
	assert.Equal(t, 1, len(resp.Choices))
	content, err := resp.Choices[0].Message.Content.AsMessageContent0()
	assert.NoError(t, err)
	assert.Equal(t, "This is a test response.", content)
	assert.Equal(t, "stop", string(resp.Choices[0].FinishReason))
}

// TestProviderListModels tests listing models functionality
func TestProviderListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/proxy/openai/models", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
            "object": "list",
            "data": [
                {
                    "id": "gpt-3.5-turbo",
                    "object": "model",
                    "created": 1677610602,
                    "owned_by": "openai"
                },
                {
                    "id": "gpt-4",
                    "object": "model",
                    "created": 1677649963,
                    "owned_by": "openai"
                }
            ]
        }`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := providersmocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "GET", req.Method)
			assert.Contains(t, req.URL.String(), "/models")
			return http.DefaultClient.Get(server.URL + "/proxy/openai/models")
		})

	log, err := logger.NewLogger("test")
	assert.NoError(t, err)

	config := &registry.ProviderConfig{
		ID:       constants.OpenaiID,
		Name:     constants.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: constants.AuthTypeBearer,
		Endpoints: types.Endpoints{
			Models: constants.OpenaiModelsEndpoint,
		},
	}

	providerRegistry := registry.NewProviderRegistry(map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: config,
	}, log)

	provider, err := providerRegistry.BuildProvider(constants.OpenaiID, mockClient)
	assert.NoError(t, err)

	resp, err := provider.ListModels(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resp.Data))
	assert.Equal(t, "openai/gpt-3.5-turbo", resp.Data[0].ID)
	assert.Equal(t, "openai/gpt-4", resp.Data[1].ID)
}

// TestDifferentAuthTypes tests that different auth types are properly handled
func TestDifferentAuthTypes(t *testing.T) {
	log, err := logger.NewLogger("test")
	assert.NoError(t, err)

	testCases := []struct {
		providerId   types.Provider
		name         string
		authType     string
		token        string
		extraHeaders map[string][]string
	}{
		{
			name:       "Bearer Auth",
			providerId: constants.OpenaiID,
			authType:   constants.AuthTypeBearer,
			token:      "sk-test-token",
		},
		{
			name:       "X-Header Auth",
			providerId: constants.AnthropicID,
			authType:   constants.AuthTypeXheader,
			token:      "anthropic-api-key",
			extraHeaders: map[string][]string{
				"anthropic-version": {"2023-06-01"},
			},
		},
		{
			name:       "Mistral Bearer Auth",
			providerId: constants.MistralID,
			authType:   constants.AuthTypeBearer,
			token:      "mistral-api-key",
		},
		{
			name:       "No Auth",
			providerId: constants.OllamaID,
			authType:   constants.AuthTypeNone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := map[types.Provider]*registry.ProviderConfig{
				tc.providerId: {
					ID:           tc.providerId,
					Name:         "Test Provider",
					URL:          "http://example.com",
					Token:        tc.token,
					AuthType:     tc.authType,
					ExtraHeaders: tc.extraHeaders,
				},
			}

			providerRegistry := registry.NewProviderRegistry(cfg, log)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockClient := providersmocks.NewMockClient(ctrl)

			provider, err := providerRegistry.BuildProvider(tc.providerId, mockClient)
			require.NoError(t, err)

			assert.Equal(t, tc.authType, provider.GetAuthType())
			assert.Equal(t, tc.token, provider.GetToken())
			if tc.extraHeaders != nil {
				assert.Equal(t, tc.extraHeaders, provider.GetExtraHeaders())
			}
		})
	}
}

func BenchmarkChatCompletions(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
            "id": "test-completion-id",
            "object": "chat.completion",
            "created": 1677858242,
            "model": "gpt-3.5-turbo",
            "choices": [
                {
                    "message": {
                        "role": "assistant",
                        "content": "This is a test response for benchmarking."
                    },
                    "finish_reason": "stop",
                    "index": 0
                }
            ]
        }`))
	}))
	defer server.Close()

	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	mockClient := providersmocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Post(server.URL+"/proxy/openai/chat/completions", "application/json", nil)
		}).
		AnyTimes()

	log, _ := logger.NewLogger("test")
	config := &registry.ProviderConfig{
		ID:       constants.OpenaiID,
		Name:     constants.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: constants.AuthTypeBearer,
		Endpoints: types.Endpoints{
			Chat: constants.OpenaiChatEndpoint,
		},
	}

	providerRegistry := registry.NewProviderRegistry(map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: config,
	}, log)

	provider, _ := providerRegistry.BuildProvider(constants.OpenaiID, mockClient)

	msg := types.NewTextMessage(b, types.User, "Hello, how are you?")

	req := types.CreateChatCompletionRequest{
		Model: "openai/gpt-3.5-turbo",
		Messages: []types.Message{
			msg,
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = provider.ChatCompletions(context.Background(), req)
	}
}

func BenchmarkListModels(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"object": "list",
            "data": [
                {
                    "id": "openai/gpt-3.5-turbo",
                    "object": "model",
                    "created": 1677610602,
                    "owned_by": "openai"
                },
                {
                    "id": "openai/gpt-4",
                    "object": "model",
                    "created": 1677649963,
                    "owned_by": "openai"
                }
            ]
        }`))
	}))
	defer server.Close()

	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	mockClient := providersmocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Get(server.URL + "/proxy/openai/models")
		}).
		AnyTimes()

	log, _ := logger.NewLogger("test")
	config := &registry.ProviderConfig{
		ID:       constants.OpenaiID,
		Name:     constants.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: constants.AuthTypeBearer,
		Endpoints: types.Endpoints{
			Models: constants.OpenaiModelsEndpoint,
		},
	}

	providerRegistry := registry.NewProviderRegistry(map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: config,
	}, log)

	provider, _ := providerRegistry.BuildProvider(constants.OpenaiID, mockClient)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = provider.ListModels(context.Background())
	}
}

func BenchmarkProviderTransformations(b *testing.B) {
	ollamaData := &transformers.ListModelsResponseOllama{
		Data: []types.Model{
			{ID: "openai/gpt-3.5-turbo"},
			{ID: "openai/gpt-4"},
		},
	}

	openaiData := &transformers.ListModelsResponseOpenai{
		Data: []types.Model{
			{ID: "openai/gpt-4"},
			{ID: "openai/gpt-3.5-turbo"},
		},
	}

	mistralData := &transformers.ListModelsResponseMistral{
		Object: "list",
		Data: []types.Model{
			{ID: "mistral-large-latest"},
			{ID: "mistral-small-latest"},
		},
	}

	b.Run("OllamaTransform", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ollamaData.Transform()
		}
	})

	b.Run("OpenAITransform", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = openaiData.Transform()
		}
	})

	b.Run("MistralTransform", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mistralData.Transform()
		}
	})
}
