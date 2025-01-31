package tests

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestStreamTokens(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		mockResponse string
		messages     []providers.Message
		expectedResp providers.GenerateResponse
		testCancel   bool
		expectError  bool
	}{
		// {
		// 	name:         "Ollama successful response",
		// 	provider:     providers.OllamaID,
		// 	mockResponse: `{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":" are","done":false}\n`,
		// 	messages: []providers.Message{
		// 		{Role: "user", Content: "Hello"},
		// 	},
		// 	expectedResp: providers.GenerateResponse{
		// 		Provider: "Ollama",
		// 		Response: providers.ResponseTokens{
		// 			Content: " are",
		// 			Model:   "phi3:3.8b",
		// 			Role:    "assistant",
		// 		},
		// 	},
		// 	testCancel:  false,
		// 	expectError: false,
		// },
		// {
		// 	name:         "Context cancellation",
		// 	provider:     providers.OllamaID,
		// 	mockResponse: `{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":" are","done":false}\n`,
		// 	messages: []providers.Message{
		// 		{Role: "user", Content: "Hello"},
		// 	},
		// 	testCancel:  true,
		// 	expectError: false,
		// },
		// {
		// 	name:         "Ollama error response",
		// 	provider:     providers.OllamaID,
		// 	mockResponse: `{"error":"Invalid request","message":"Invalid request"}`,
		// 	messages: []providers.Message{
		// 		{Role: "user", Content: "Hello"},
		// 	},
		// 	testCancel:  false,
		// 	expectError: true,
		// },
		{
			name:     "Groq successful response",
			provider: providers.GroqID,
			mockResponse: `data: {"id":"test-id","object":"text","created":1644000000,"model":"test-model","choices":[{"index":0,"message":{"role":"user","content":"Hello"},"delta":{"role":"assistant","content":" are"},"logprobs":null,"finish_reason":"length"}],"usage":{"total_tokens":1,"total_characters":1},"system_fingerprint":"test-fingerprint","x_groq":{"id":"test-id"}}

data: [DONE]

`,
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
			},
			expectedResp: providers.GenerateResponse{
				Provider: providers.GroqDisplayName,
				Response: providers.ResponseTokens{
					Content: " are",
					Model:   "test-model",
					Role:    "assistant",
				},
			},
			testCancel:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockClient := mocks.NewMockClient(ctrl)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockClient.EXPECT().
				Do(gomock.Any()).
				Return(&http.Response{
					Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					StatusCode: http.StatusOK,
				}, nil)

			providersRegistry := map[string]*providers.Config{
				tt.provider: {
					ID:   tt.provider,
					Name: "ollama",
					URL:  "http://test.local",
					Endpoints: providers.Endpoints{
						Generate: "/api/generate",
						List:     "/api/tags",
					},
					AuthType: providers.AuthTypeNone,
				},
			}

			var ml logger.Logger = mockLogger
			var mc providers.Client = mockClient
			provider, err := providers.NewProvider(
				providersRegistry,
				tt.provider,
				&ml,
				&mc,
			)
			assert.NoError(t, err)

			ch, err := provider.StreamTokens(ctx, "test-model", tt.messages)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, ch)

			if !tt.testCancel {
				resp := <-ch
				assert.Equal(t, tt.expectedResp, resp)
			} else {
				cancel()
				_, ok := <-ch
				assert.False(t, ok)
			}
		})
	}
}
