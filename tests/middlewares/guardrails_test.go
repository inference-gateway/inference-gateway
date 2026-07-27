package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	types "github.com/inference-gateway/inference-gateway/providers/types"

	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
)

func TestNewGuardrailsMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		expectNoop bool
	}{
		{
			name:       "Disabled returns Noop",
			enabled:    false,
			expectNoop: true,
		},
		{
			name:       "Enabled returns real middleware",
			enabled:    true,
			expectNoop: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			cfg := config.Config{
				Guardrails: &config.GuardrailsConfig{
					Enabled: tt.enabled,
				},
			}

			if !tt.enabled {
				mockLogger.EXPECT().Info("guardrails disabled, using no-op middleware")
			}

			mw := middlewares.NewGuardrailsMiddleware(nil, nil, nil, mockLogger, nil, cfg)
			assert.NotNil(t, mw)
			handler := mw.Middleware()
			assert.NotNil(t, handler)
		})
	}
}

func TestGuardrailsMiddleware_Noop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info("guardrails disabled, using no-op middleware")

	cfg := config.Config{
		Guardrails: &config.GuardrailsConfig{
			Enabled: false,
		},
	}

	mw := middlewares.NewGuardrailsMiddleware(nil, nil, nil, mockLogger, nil, cfg)

	router := gin.New()
	router.Use(mw.Middleware())

	handlerCalled := false
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Noop middleware should call next handler")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGuardrailsMiddleware_BlockRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	cfg := config.Config{
		Guardrails: &config.GuardrailsConfig{
			Enabled:   true,
			PolicyDir: "",
			FailMode:  "closed",
		},
		Server: &config.ServerConfig{
			MaxRequestBodySize: 10485760,
		},
	}

	evaluator, err := guardrails.NewEvaluator(context.Background(), "")
	assert.NoError(t, err)

	mw := middlewares.NewGuardrailsMiddleware(evaluator, nil, nil, mockLogger, nil, cfg)

	router := gin.New()
	router.Use(mw.Middleware())

	handlerCalled := false
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGuardrailsMiddleware_Detectors(t *testing.T) {
	detectors := guardrails.DefaultDetectors()

	tests := []struct {
		name     string
		input    string
		expected int // number of matches
	}{
		{
			name:     "detect API key",
			input:    `api_key=sk-12345678901234567890`,
			expected: 1,
		},
		{
			name:     "detect bearer token",
			input:    `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`,
			expected: 1,
		},
		{
			name:     "detect email",
			input:    `user@example.com`,
			expected: 1,
		},
		{
			name:     "no match for safe text",
			input:    `Hello, how are you?`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := guardrails.ScanDetectors(tt.input, detectors)
			assert.Len(t, results, tt.expected)
		})
	}
}

func TestGuardrailsMiddleware_RedactSensitive(t *testing.T) {
	detectors := guardrails.DefaultDetectors()

	input := `api_key=sk-12345678901234567890 and email is user@example.com`
	redacted := guardrails.RedactSensitive(input, detectors)

	assert.NotContains(t, redacted, "sk-12345678901234567890", "API key should be redacted")
	assert.Contains(t, redacted, "[REDACTED]", "Should contain redacted markers")
	assert.Contains(t, redacted, "user@example.com", "Email should not be redacted (redact=false)")
}

func TestGuardrailsMiddleware_LuhnCheck(t *testing.T) {
	detectors := guardrails.DefaultDetectors()

	validCard := "4111111111111111"
	results := guardrails.ScanDetectors(validCard, detectors)
	assert.Len(t, results, 1, "Valid credit card should be detected")

	invalidCard := "1234567890123456"
	results = guardrails.ScanDetectors(invalidCard, detectors)
	assert.Len(t, results, 0, "Invalid Luhn number should not be detected as credit card")
}

func TestGuardrailsMiddleware_ExternalClient(t *testing.T) {
	client := guardrails.NewExternalClient("", 0)
	dec, err := client.Check(context.Background(), &guardrails.Input{
		Method: "POST",
		Path:   "/v1/chat/completions",
		Phase:  "pre_call",
	})
	assert.NoError(t, err)
	assert.Equal(t, guardrails.ActionAllow, dec.Action)
}

func TestGuardrailsMiddleware_PolicyCompile(t *testing.T) {
	evaluator, err := guardrails.NewEvaluator(context.Background(), "../../examples/guardrails/policies")
	assert.NoError(t, err, "example policies should compile without error")
	assert.NotNil(t, evaluator)

	dec, err := evaluator.Eval(context.Background(), &guardrails.Input{
		Method: "POST",
		Path:   "/v1/chat/completions",
		Phase:  "pre_call",
		Request: &guardrails.Req{
			Body:  `{"model":"test","messages":[]}`,
			Model: "test",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, guardrails.ActionAllow, dec.Action)
}

func TestGuardrailsMiddleware_NonStreamingPostCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	cfg := config.Config{
		Guardrails: &config.GuardrailsConfig{
			Enabled:   true,
			PolicyDir: "",
			FailMode:  "closed",
		},
		Server: &config.ServerConfig{
			MaxRequestBodySize: 10485760,
		},
	}

	evaluator, err := guardrails.NewEvaluator(context.Background(), "")
	assert.NoError(t, err)

	mw := middlewares.NewGuardrailsMiddleware(evaluator, nil, nil, mockLogger, nil, cfg)

	router := gin.New()
	router.Use(mw.Middleware())

	requestData := types.CreateChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.Message{
			types.NewTextMessage(t, types.User, "Hello"),
		},
	}
	requestBody, _ := json.Marshal(requestData)

	router.POST("/v1/chat/completions", func(c *gin.Context) {
		response := types.CreateChatCompletionResponse{
			ID:    "test-id",
			Model: "gpt-4",
			Choices: []types.ChatCompletionChoice{
				{
					Message:      types.NewTextMessage(t, types.Assistant, "Hello! How can I help you?"),
					FinishReason: types.Stop,
				},
			},
		}
		c.JSON(http.StatusOK, response)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp types.CreateChatCompletionResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	content, err := resp.Choices[0].Message.Content.AsMessageContent0()
	assert.NoError(t, err)
	assert.Equal(t, "Hello! How can I help you?", content)
}
