package middlewares

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
)

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

	if err := client.InitializeAll(context.Background()); err != nil {
		logger.Error("Failed to initialize MCP client", err)
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return &MCPMiddlewareImpl{
		client: client,
		logger: logger,
		config: cfg,
	}, nil
}

// Middleware returns a HTTP middleware handler for MCP integration
func (m *MCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	// It captures the request and response, modifies them as needed, and handles tool calls
	// It also sets the context to indicate that MCP is being used
	// If MCP is not enabled, it simply calls the next handler in the chain
	// If there is no tool call, it proceeds with the original response
	// If it didn't find any tool calls, it returns the original response
	// It informs the client if there is a tool call in progress via notification or SSE
	return func(c *gin.Context) {

		wantsSSE := strings.Contains(c.GetHeader("Accept"), "text/event-stream")

		// processs request

		c.Next()

		// process response
		if wantsSSE {
			// process streaming response
		} else {
			// process non-streaming response
		}
	}
}

// NoopMCPMiddlewareImpl is a no-operation implementation of MCPMiddleware
func (m *NoopMCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
