package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestProviderRegistry tests the provider registry functionality
func TestProviderRegistry(t *testing.T) {
	log, err := logger.NewLogger("test")
	assert.NoError(t, err)

	cfg := map[providers.Provider]*providers.Config{
		providers.OpenaiID: {
			ID:       providers.OpenaiID,
			Name:     providers.OpenaiDisplayName,
			URL:      providers.OpenaiDefaultBaseURL,
			Token:    "test-token",
			AuthType: providers.AuthTypeBearer,
			Endpoints: providers.Endpoints{
				Models: providers.OpenaiModelsEndpoint,
				Chat:   providers.OpenaiChatEndpoint,
			},
		},
		providers.MistralID: {
			ID:       providers.MistralID,
			Name:     providers.MistralDisplayName,
			URL:      providers.MistralDefaultBaseURL,
			Token:    "test-token",
			AuthType: providers.AuthTypeBearer,
			Endpoints: providers.Endpoints{
				Models: providers.MistralModelsEndpoint,
				Chat:   providers.MistralChatEndpoint,
			},
		},
		providers.OllamaID: {
			ID:       providers.OllamaID,
			Name:     providers.OllamaDisplayName,
			URL:      providers.OllamaDefaultBaseURL,
			AuthType: providers.AuthTypeNone,
			Endpoints: providers.Endpoints{
				Models: providers.OllamaModelsEndpoint,
				Chat:   providers.OllamaChatEndpoint,
			},
		},
	}

	registry := providers.NewProviderRegistry(cfg, log)

	providerConfigs := registry.GetProviders()
	assert.Equal(t, len(cfg), len(providerConfigs))
	assert.Equal(t, cfg[providers.OpenaiID], providerConfigs[providers.OpenaiID])
	assert.Equal(t, cfg[providers.MistralID], providerConfigs[providers.MistralID])
	assert.Equal(t, cfg[providers.OllamaID], providerConfigs[providers.OllamaID])

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := providersmocks.NewMockClient(ctrl)

	openaiProvider, err := registry.BuildProvider(providers.OpenaiID, mockClient)
	require.NoError(t, err)
	assert.Equal(t, providers.OpenaiID, *openaiProvider.GetID())
	assert.Equal(t, providers.OpenaiDisplayName, openaiProvider.GetName())
	assert.Equal(t, providers.OpenaiDefaultBaseURL, openaiProvider.GetURL())
	assert.Equal(t, "test-token", openaiProvider.GetToken())
	assert.Equal(t, providers.AuthTypeBearer, openaiProvider.GetAuthType())

	_, err = registry.BuildProvider("invalid-provider", mockClient)
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

	config := &providers.Config{
		ID:       providers.OpenaiID,
		Name:     providers.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: providers.AuthTypeBearer,
		Endpoints: providers.Endpoints{
			Chat: providers.OpenaiChatEndpoint,
		},
	}

	registry := providers.NewProviderRegistry(map[providers.Provider]*providers.Config{
		providers.OpenaiID: config,
	}, log)

	provider, err := registry.BuildProvider(providers.OpenaiID, mockClient)
	assert.NoError(t, err)

	roleUser := providers.MessageRoleUser
	req := providers.CreateChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    roleUser,
				Content: "Hello, how are you?",
			},
		},
	}

	resp, err := provider.ChatCompletions(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "test-completion-id", resp.ID)
	assert.Equal(t, "gpt-3.5-turbo", resp.Model)
	assert.Equal(t, 1, len(resp.Choices))
	assert.Equal(t, "This is a test response.", resp.Choices[0].Message.Content)
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

	config := &providers.Config{
		ID:       providers.OpenaiID,
		Name:     providers.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: providers.AuthTypeBearer,
		Endpoints: providers.Endpoints{
			Models: providers.OpenaiModelsEndpoint,
		},
	}

	registry := providers.NewProviderRegistry(map[providers.Provider]*providers.Config{
		providers.OpenaiID: config,
	}, log)

	provider, err := registry.BuildProvider(providers.OpenaiID, mockClient)
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
		providerId   providers.Provider
		name         string
		authType     string
		token        string
		extraHeaders map[string][]string
	}{
		{
			name:       "Bearer Auth",
			providerId: providers.OpenaiID,
			authType:   providers.AuthTypeBearer,
			token:      "sk-test-token",
		},
		{
			name:       "X-Header Auth",
			providerId: providers.AnthropicID,
			authType:   providers.AuthTypeXheader,
			token:      "anthropic-api-key",
			extraHeaders: map[string][]string{
				"anthropic-version": {"2023-06-01"},
			},
		},
		{
			name:       "Mistral Bearer Auth",
			providerId: providers.MistralID,
			authType:   providers.AuthTypeBearer,
			token:      "mistral-api-key",
		},
		{
			name:       "No Auth",
			providerId: providers.OllamaID,
			authType:   providers.AuthTypeNone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := map[providers.Provider]*providers.Config{
				tc.providerId: {
					ID:           tc.providerId,
					Name:         "Test Provider",
					URL:          "http://example.com",
					Token:        tc.token,
					AuthType:     tc.authType,
					ExtraHeaders: tc.extraHeaders,
				},
			}

			registry := providers.NewProviderRegistry(cfg, log)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockClient := providersmocks.NewMockClient(ctrl)

			provider, err := registry.BuildProvider(tc.providerId, mockClient)
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
	config := &providers.Config{
		ID:       providers.OpenaiID,
		Name:     providers.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: providers.AuthTypeBearer,
		Endpoints: providers.Endpoints{
			Chat: providers.OpenaiChatEndpoint,
		},
	}

	registry := providers.NewProviderRegistry(map[providers.Provider]*providers.Config{
		providers.OpenaiID: config,
	}, log)

	provider, _ := registry.BuildProvider(providers.OpenaiID, mockClient)

	roleUser := providers.MessageRoleUser
	req := providers.CreateChatCompletionRequest{
		Model: "openai/gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    roleUser,
				Content: "Hello, how are you?",
			},
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
	config := &providers.Config{
		ID:       providers.OpenaiID,
		Name:     providers.OpenaiDisplayName,
		URL:      server.URL,
		Token:    "test-token",
		AuthType: providers.AuthTypeBearer,
		Endpoints: providers.Endpoints{
			Models: providers.OpenaiModelsEndpoint,
		},
	}

	registry := providers.NewProviderRegistry(map[providers.Provider]*providers.Config{
		providers.OpenaiID: config,
	}, log)

	provider, _ := registry.BuildProvider(providers.OpenaiID, mockClient)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = provider.ListModels(context.Background())
	}
}

func BenchmarkProviderTransformations(b *testing.B) {
	ollamaData := &providers.ListModelsResponseOllama{
		Data: []providers.Model{
			{ID: "openai/gpt-3.5-turbo"},
			{ID: "openai/gpt-4"},
		},
	}

	openaiData := &providers.ListModelsResponseOpenai{
		Data: []providers.Model{
			{ID: "openai/gpt-4"},
			{ID: "openai/gpt-3.5-turbo"},
		},
	}

	mistralData := &providers.ListModelsResponseMistral{
		Object: "list",
		Data: []providers.Model{
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
