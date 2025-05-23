package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
)

// ChatCompletionsPath is the specific API path for chat completions
const ChatCompletionsPath = "/v1/chat/completions"

// MCPMiddleware is an interface for middleware that integrates with the Model Context Protocol (MCP)
type MCPMiddleware interface {
	Middleware() gin.HandlerFunc
}

// MCPMiddlewareImpl adds Model Context Protocol capabilities to LLM requests
type MCPMiddlewareImpl struct {
	client mcp.MCPClientInterface
	logger logger.Logger
	config config.Config
}

// NoopMCPMiddlewareImpl is a no-operation implementation of MCPMiddleware
type NoopMCPMiddlewareImpl struct{}

// NewMCPMiddleware creates a new middleware instance for MCP integration
func NewMCPMiddleware(client mcp.MCPClientInterface, logger logger.Logger, cfg config.Config) (MCPMiddleware, error) {
	if !cfg.EnableMcp {
		return &NoopMCPMiddlewareImpl{}, nil
	}

	if cfg.McpServers == "" {
		logger.Debug("no MCP server URLs provided")
		return &NoopMCPMiddlewareImpl{}, nil
	}

	if client == nil {
		return &NoopMCPMiddlewareImpl{}, nil
	}

	if !client.IsInitialized() {
		if err := client.InitializeAll(context.Background()); err != nil {
			logger.Error("Failed to initialize MCP client", err)
			return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
		}
	}

	return &MCPMiddlewareImpl{
		client: client,
		logger: logger,
		config: cfg,
	}, nil
}

// Middleware returns a HTTP middleware handler for MCP integration
func (m *MCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		c.Set("use_mcp", true)

		m.logger.Debug("MCP: Processing request for tool enhancement")
		var requestBody providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			m.logger.Debug("MCP: Could not parse request body", "error", err.Error())
			c.Next()
			return
		}

		if requestBody.Stream != nil && *requestBody.Stream {
			m.logger.Debug("MCP: Streaming request detected, skipping MCP processing")
			c.Next()
			return
		}

		if len(requestBody.Messages) == 0 {
			m.logger.Debug("MCP: No messages found in request")
			c.Next()
			return
		}

		m.logger.Debug("MCP: Getting pre-converted tools from MCP client")
		chatCompletionTools := m.client.GetAllChatCompletionTools()

		if len(chatCompletionTools) > 0 {
			m.logger.Debug("MCP: Adding tools to request", "count", len(chatCompletionTools))
			requestBody.Tools = &chatCompletionTools

			jsonData, err := json.Marshal(requestBody)
			if err != nil {
				m.logger.Error("MCP: Failed to marshal modified request", err)
				c.Next()
				return
			}

			c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
			c.Request.ContentLength = int64(len(jsonData))
			c.Request.Header.Set("Content-Length", fmt.Sprint(len(jsonData)))
		} else {
			m.logger.Debug("MCP: No tools available to add to request")
		}

		w := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = w

		c.Next()

		var responseBody providers.CreateChatCompletionResponse
		bodyBytes := w.body.Bytes()
		if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
			m.logger.Debug("MCP: Could not parse response body", "error", err.Error())
			return
		}

		if len(responseBody.Choices) == 0 {
			m.logger.Debug("MCP: No choices found in response")
			return
		}
		if responseBody.Choices[0].Message.ToolCalls == nil {
			m.logger.Debug("MCP: No tool calls found in response message")
			return
		}

		// TODO - while there are tool_calls in the response continue processing, otherwise return the response back to the client, for now set timeout to 70 seconds
		timeout := time.After(10 * time.Second)
		for {
			select {
			case <-timeout:
				m.logger.Debug("MCP: Timeout reached, stopping tool call processing")
				return
			default:
				m.logger.Debug("MCP: Processing tool calls from response")
				// 1. Parse the tool calls from the response

				// 2. Call the ExecuteTool function using the MCP client and get the tool call results

				// 3. Send additional request to the LLM with the tool call results

				time.Sleep(time.Second * 5)
			}
		}

	}
}

// Middleware returns a HTTP middleware handler (no-op implementation)
func (m *NoopMCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
