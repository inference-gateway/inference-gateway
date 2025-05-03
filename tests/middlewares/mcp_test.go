package middleware_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mcpMiddleware "github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	mcp "github.com/inference-gateway/inference-gateway/mcp"
	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMCPMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		config          config.Config
		requestBody     string
		setupExpections func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		applyAssertions func(*testing.T, error)
	}{
		{
			name:        "MCP Enabled without configured MCP servers",
			config:      config.Config{McpServers: "", EnableMcp: true},
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
			setupExpections: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("no MCP server URLs provided").Times(1)
				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				return mockLogger, mockClient
			},
			applyAssertions: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:        "MCP Enabled with server but initialization fails",
			config:      config.Config{McpServers: "http://mcp-server:8080", EnableMcp: true},
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
			setupExpections: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Error("Failed to initialize MCP client", gomock.Any()).Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(errors.New("initialization error")).Times(1)
				return mockLogger, mockClient
			},
			applyAssertions: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Equal(t, "failed to initialize MCP client: initialization error", err.Error())
			},
		},
		{
			name:        "MCP Enabled with server and successful initialization",
			config:      config.Config{McpServers: "http://mcp-server:8080", EnableMcp: true},
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
			setupExpections: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				return mockLogger, mockClient
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger, client := tt.setupExpections(ctrl)
			middleware, err := mcpMiddleware.NewMCPMiddleware(client, logger, tt.config)

			if tt.applyAssertions != nil {
				tt.applyAssertions(t, err)
			}

			if err != nil {
				return
			}

			r := gin.New()
			r.Use(middleware.Middleware())
			r.POST("/v1/chat/completions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "pong"})
			})

			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(strings.NewReader(tt.requestBody)))

			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)
		})
	}
}
