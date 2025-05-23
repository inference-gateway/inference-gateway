package middleware_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMCPMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		requestBody    interface{}
		setupMock      func(*mocks.MockMCPClientInterface)
		expectedKey    string
		expectedValue  interface{}
		expectedStatus int
	}{
		{
			name: "Skip non-chat endpoint",
			path: "/v1/models",
			requestBody: map[string]interface{}{
				"model": "model1",
			},
			setupMock: func(mock *mocks.MockMCPClientInterface) {
				mock.EXPECT().IsInitialized().Return(true).AnyTimes()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "Invalid JSON body",
			path:        middlewares.ChatCompletionsPath,
			requestBody: "invalid-json",
			setupMock: func(mock *mocks.MockMCPClientInterface) {
				mock.EXPECT().IsInitialized().Return(true).AnyTimes()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Streaming request",
			path: middlewares.ChatCompletionsPath,
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "model1",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
				Stream:   boolPtr(true),
			},
			setupMock: func(mock *mocks.MockMCPClientInterface) {
				mock.EXPECT().IsInitialized().Return(true).AnyTimes()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Empty messages",
			path: middlewares.ChatCompletionsPath,
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "model1",
				Messages: []providers.Message{},
			},
			setupMock: func(mock *mocks.MockMCPClientInterface) {
				mock.EXPECT().IsInitialized().Return(true).AnyTimes()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "No tools available",
			path: middlewares.ChatCompletionsPath,
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "model1",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMock: func(mock *mocks.MockMCPClientInterface) {
				mock.EXPECT().IsInitialized().Return(true).AnyTimes()
				mock.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{})
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Tools available",
			path: middlewares.ChatCompletionsPath,
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "model1",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMock: func(mock *mocks.MockMCPClientInterface) {
				mock.EXPECT().IsInitialized().Return(true).AnyTimes()

				description := "Test tool description"
				params := make(providers.FunctionParameters)
				params["type"] = "object"
				properties := make(map[string]interface{})
				paramProps := make(map[string]interface{})
				paramProps["type"] = "string"
				paramProps["description"] = "Test parameter"
				properties["param1"] = paramProps
				params["properties"] = properties

				tools := []providers.ChatCompletionTool{
					{
						Type: providers.ChatCompletionToolTypeFunction,
						Function: providers.FunctionObject{
							Name:        "test-tool",
							Description: &description,
							Parameters:  &params,
						},
					},
				}
				mock.EXPECT().GetAllChatCompletionTools().Return(tools)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockMCPClientInterface(ctrl)
			tt.setupMock(mockClient)

			mockLogger, err := logger.NewLogger("development")
			require.NoError(t, err)

			cfg := config.Config{
				EnableMcp:  true,
				McpServers: "http://test-server",
			}
			mcpMiddleware, err := middlewares.NewMCPMiddleware(mockClient, mockLogger, cfg)
			require.NoError(t, err)

			router := gin.New()
			router.Use(mcpMiddleware.Middleware())

			var passedTools *[]providers.ChatCompletionTool
			var useMcp bool

			router.POST(tt.path, func(c *gin.Context) {
				val, exists := c.Get("use_mcp")
				if exists {
					useMcp = val.(bool)
				}

				if c.Request.Body != nil {
					bodyBytes, _ := io.ReadAll(c.Request.Body)
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					if len(bodyBytes) > 0 {
						var reqBody providers.CreateChatCompletionRequest
						if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
							passedTools = reqBody.Tools
						}
					}
				}

				c.Status(http.StatusOK)
			})

			var reqBody []byte
			var err2 error
			if s, ok := tt.requestBody.(string); ok {
				reqBody = []byte(s)
			} else {
				reqBody, err2 = json.Marshal(tt.requestBody)
				require.NoError(t, err2)
			}

			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.path == middlewares.ChatCompletionsPath {
				assert.True(t, useMcp, "use_mcp flag should be set for chat completions path")
			}

			if tt.name == "Tools available" {
				require.NotNil(t, passedTools, "Tools should be added to the request")
				assert.Equal(t, 1, len(*passedTools), "Request should have 1 tool")
				assert.Equal(t, "test-tool", (*passedTools)[0].Function.Name, "Tool name should match")
			}
		})
	}
}

// boolPtr is a helper function to return a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}
