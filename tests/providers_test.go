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

func TestStreamTokens_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockClient := mocks.NewMockClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	// Mock stream response
	mockResponse := `{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":" are","done":false}
` // line break is required to simulate streaming
	mockClient.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			Body:       io.NopCloser(strings.NewReader(mockResponse)),
			StatusCode: http.StatusOK,
		}, nil)

	providersRegistry := map[string]*providers.Config{
		providers.OllamaID: {
			ID:   providers.OllamaID,
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
		providers.OllamaID,
		&ml,
		&mc,
	)
	assert.NoError(t, err)

	ch, err := provider.StreamTokens(ctx, "test-model", []providers.Message{
		{Role: "user", Content: "Hello"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	// Validate response matches expected format
	resp := <-ch
	assert.Equal(t, providers.GenerateResponse{
		Provider: "Ollama",
		Response: providers.ResponseTokens{
			Content: " are",
			Model:   "phi3:3.8b",
			Role:    "assistant",
		},
	}, resp)

	cancel()

	// Verify channel closes after cancellation
	_, ok := <-ch
	assert.False(t, ok)
}

func TestStreamTokens_GroqResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockClient := mocks.NewMockClient(ctrl)

	ctx := context.Background()

	// Mock streamed response
	mockResponse := `data: {"id":"c4ac2b07-433c-41c7-af83-ff4d5b589a5b","model":"llama-3.3-70b-versatile","created":1709150338,"choices":[{"index":0,"delta":{"role":"assistant","content":" The"},"finish_reason":null}],"usage":{"prompt_tokens":57},"system_fingerprint":"fp_142b8a39df"}
` // Note: newline is required to simulate streaming chunks
	mockClient.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			Body:       io.NopCloser(strings.NewReader(mockResponse)),
			StatusCode: http.StatusOK,
		}, nil)

	providersRegistry := map[string]*providers.Config{
		providers.GroqID: {
			ID:    providers.GroqID,
			Name:  "groq",
			URL:   "http://test.local",
			Token: "test-token",
			Endpoints: providers.Endpoints{
				Generate: "/chat/completions",
				List:     "/models",
			},
			AuthType: providers.AuthTypeBearer,
		},
	}

	var ml logger.Logger = mockLogger
	var mc providers.Client = mockClient
	provider, err := providers.NewProvider(
		providersRegistry,
		providers.GroqID,
		&ml,
		&mc,
	)
	assert.NoError(t, err)

	ch, err := provider.StreamTokens(ctx, "llama-3.3-70b-versatile", []providers.Message{
		{Role: "user", Content: "Why is the sky blue?"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	// Validate response matches expected format
	resp := <-ch
	assert.Equal(t, providers.GenerateResponse{
		Provider: "Groq",
		Response: providers.ResponseTokens{
			Content: " The",
			Model:   "llama-3.3-70b-versatile",
			Role:    "assistant",
		},
	}, resp)

	// Verify channel closes normally
	_, ok := <-ch
	assert.False(t, ok)
}
