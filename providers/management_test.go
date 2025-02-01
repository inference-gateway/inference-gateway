package providers_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	"go.uber.org/mock/gomock"
)

func TestParseSSEDebug(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		wantType providers.EventType
	}{
		{
			name:     "message-start event",
			input:    "event: message-start\n",
			wantType: providers.EventMessageStart,
		},
		{
			name:     "content delta",
			input:    "data: {\"content\":\"hello\"}\n",
			wantType: providers.EventContentDelta,
		},
		{
			name:     "stream end",
			input:    "data: [DONE]\n",
			wantType: providers.EventStreamEnd,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := providers.ParseSSEvents([]byte(tc.input))
			if err != nil {
				t.Errorf("parseSSE() error = %v", err)
				return
			}
			t.Logf("Input: %q", tc.input)
			t.Logf("Got event type: %v", event.EventType)
			t.Logf("Got data: %q", event.Data)
		})
	}
}

func TestParseSSEWithEmbeddedMessageStart(t *testing.T) {
	input := `data: {"json": "{\"id\":\"d8c1879d-6c59-4eb7-8209-b184f81bcf15\",\"type\":\"message-start\",\"delta\":{\"message\":{\"role\":\"assistant\",\"content\":[],\"tool_plan\":\"\",\"tool_calls\":[],\"citations\":[]}}}"}`

	event, err := providers.ParseSSEvents([]byte(input))
	if err != nil {
		t.Fatalf("parseSSE() error = %v", err)
	}

	// Verify event type is MessageStart since message-start is in JSON
	if event.EventType != providers.EventMessageStart {
		t.Errorf("expected EventMessageStart, got %v", event.EventType)
	}

	// Verify data is preserved
	if !bytes.Contains(event.Data, []byte("message-start")) {
		t.Errorf("data should contain message-start marker\ngot: %s", event.Data)
	}

	t.Logf("Event type: %v", event.EventType)
	t.Logf("Event data: %s", event.Data)
}

func BenchmarkListModels(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	mockLogger := mocks.NewMockLogger(ctrl)
	mockClient := mocks.NewMockClient(ctrl)

	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Define the config map used for building the provider registry.
	configMap := map[string]*providers.Config{
		providers.OllamaID: {
			ID:       providers.OllamaID,
			Name:     providers.OllamaDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeNone,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				List: "/models",
			},
		},
		providers.GroqID: {
			ID:       providers.GroqID,
			Name:     providers.GroqDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeBearer,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				List: "/models",
			},
		},
		providers.OpenaiID: {
			ID:       providers.OpenaiID,
			Name:     providers.OpenaiDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeBearer,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				List: "/models",
			},
		},
		providers.AnthropicID: {
			ID:       providers.AnthropicID,
			Name:     providers.AnthropicDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeXheader,
			Token:    "test-token",
			ExtraHeaders: map[string][]string{
				"anthropic-version": {"2023-06-01"},
			},
			Endpoints: providers.Endpoints{
				List: "/models",
			},
		},
		providers.CloudflareID: {
			ID:       providers.CloudflareID,
			Name:     providers.CloudflareDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeBearer,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				List: "/models",
			},
		},
		providers.CohereID: {
			ID:       providers.CohereID,
			Name:     providers.CohereDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeBearer,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				List: "/models",
			},
		},
	}

	providerRegistry := providers.NewProviderRegistry(configMap, mockLogger)

	responses := map[string]struct {
		body        string
		contentType string
	}{
		providers.OllamaID: {
			body:        `{"models":[{"name":"llama2","modified_at":"2024-01-01T00:00:00Z"}]}`,
			contentType: "application/json",
		},
		providers.GroqID: {
			body:        `{"models":[{"id":"llama-70b","created":1234567890}]}`,
			contentType: "application/json",
		},
		providers.OpenaiID: {
			body:        `{"data":[{"id":"gpt-4","owned_by":"openai"}]}`,
			contentType: "application/json",
		},
		providers.AnthropicID: {
			body:        `{"data":[{"id":"claude-3-opus-20240229","display_name":"Claude 3 Opus"}]}`,
			contentType: "application/json",
		},
		providers.CloudflareID: {
			body:        `{"result":[{"id":"@cf/meta/llama-2-7b","name":"Llama 2 7B","description":"Meta's Llama 2 7B model"}]}`,
			contentType: "application/json",
		},
		providers.CohereID: {
			body:        `{"models":[{"name":"command","endpoints":["generate","chat"],"context_length":4096}]}`,
			contentType: "application/json",
		},
	}

	// Iterate over the keys of the config map.
	for providerID := range configMap {
		b.Run(providerID, func(b *testing.B) {
			mockClient.EXPECT().
				Do(gomock.Any()).
				DoAndReturn(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						Body:       io.NopCloser(strings.NewReader(responses[providerID].body)),
						Header:     http.Header{"Content-Type": []string{responses[providerID].contentType}},
						StatusCode: http.StatusOK,
					}, nil
				}).
				AnyTimes()

			provider, err := providerRegistry.BuildProvider(providerID, mockClient)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := provider.ListModels(context.Background())
				if err != nil {
					b.Fatalf("provider %s failed: %v, response: %s",
						providerID, err, responses[providerID].body)
				}
			}
		})
	}
}

func BenchmarkGenerateTokens(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	mockLogger := mocks.NewMockLogger(ctrl)
	mockClient := mocks.NewMockClient(ctrl)

	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	responses := map[string]struct {
		body        string
		contentType string
	}{
		providers.GroqID: {
			body: `{
                "id": "chatcmpl-123",
                "object": "chat.completion",
                "choices": [{
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": "Hello! How can I help you?"
                    },
                    "finish_reason": "stop"
                }]
            }`,
			contentType: "application/json",
		},
		providers.OpenaiID: {
			body: `{
                "id": "chatcmpl-456", 
                "object": "chat.completion",
                "choices": [{
                    "message": {
                        "role": "assistant",
                        "content": "Hello! How can I help you?"
                    },
                    "finish_reason": "stop"
                }]
            }`,
			contentType: "application/json",
		},
	}

	providerConfigs := map[string]*providers.Config{
		providers.GroqID: {
			ID:       providers.GroqID,
			Name:     providers.GroqDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeBearer,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				Generate: providers.GroqGenerateEndpoint,
			},
		},
		providers.OpenaiID: {
			ID:       providers.OpenaiID,
			Name:     providers.OpenaiDisplayName,
			URL:      "http://test.local",
			AuthType: providers.AuthTypeBearer,
			Token:    "test-token",
			Endpoints: providers.Endpoints{
				Generate: providers.OpenAIGenerateEndpoint,
			},
		},
	}

	messages := []providers.Message{
		{Role: "user", Content: "Hello"},
	}

	registry := providers.NewProviderRegistry(providerConfigs, mockLogger)

	for providerID := range providerConfigs {
		b.Run(providerID, func(b *testing.B) {
			mockClient.EXPECT().
				Do(gomock.Any()).
				DoAndReturn(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						Body:       io.NopCloser(strings.NewReader(responses[providerID].body)),
						Header:     http.Header{"Content-Type": []string{responses[providerID].contentType}},
						StatusCode: http.StatusOK,
					}, nil
				}).
				AnyTimes()

			provider, err := registry.BuildProvider(providerID, mockClient)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := provider.GenerateTokens(context.Background(), "test-model", messages)
				if err != nil {
					b.Fatalf("provider %s failed: %v\nResponse body: %s", providerID, err, responses[providerID].body)
				}
				if resp.Response.Content == "" {
					b.Fatalf("empty response content from %s\nResponse body: %s", providerID, responses[providerID].body)
				}
			}
		})
	}
}
