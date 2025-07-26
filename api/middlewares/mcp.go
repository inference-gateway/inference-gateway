package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
)

const (
	// MCPBypassHeader marks internal MCP requests to prevent middleware loops
	MCPBypassHeader = "X-MCP-Bypass"

	// MaxMCPAgentIterations limits the number of agent loop iterations
	MaxMCPAgentIterations = 10
)

// mcpContextKey is a custom type for context keys to avoid collisions
type mcpContextKey string

const (
	// mcpBypassKey is the context key for marking to bypass MCP middleware
	mcpBypassKey mcpContextKey = MCPBypassHeader
)

// MCPProviderModelResult contains the result of provider and model determination
type MCPProviderModelResult struct {
	Provider      providers.IProvider
	ProviderModel string
	ProviderID    *providers.Provider
}

// MCPMiddleware defines the interface for MCP middleware
type MCPMiddleware interface {
	Middleware() gin.HandlerFunc
}

// MCPMiddlewareImpl implements the MCP middleware
type MCPMiddlewareImpl struct {
	registry               providers.ProviderRegistry
	inferenceGatewayClient providers.Client
	mcpClient              mcp.MCPClientInterface
	mcpAgent               mcp.Agent
	logger                 logger.Logger
	config                 config.Config
}

// NoopMCPMiddlewareImpl is a no-op implementation of MCPMiddleware
type NoopMCPMiddlewareImpl struct{}

// NewMCPMiddleware creates a new MCP middleware instance
func NewMCPMiddleware(registry providers.ProviderRegistry, inferenceGatewayClient providers.Client, mcpClient mcp.MCPClientInterface, mcpAgent mcp.Agent, log logger.Logger, cfg config.Config) (MCPMiddleware, error) {
	if mcpClient == nil {
		log.Info("mcp client is nil, using no-op middleware")
		return &NoopMCPMiddlewareImpl{}, nil
	}

	return &MCPMiddlewareImpl{
		registry:               registry,
		inferenceGatewayClient: inferenceGatewayClient,
		mcpClient:              mcpClient,
		mcpAgent:               mcpAgent,
		logger:                 log,
		config:                 cfg,
	}, nil
}

// Middleware returns the no-op middleware handler
func (n *NoopMCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware returns the MCP middleware handler
func (m *MCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(MCPBypassHeader) != "" {
			m.logger.Debug("skipping mcp middleware for internal call")
			c.Next()
			return
		}

		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		m.logger.Debug("mcp middleware invoked", "path", c.Request.URL.Path)
		var originalRequestBody providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&originalRequestBody); err != nil {
			m.logger.Error("failed to parse request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			c.Abort()
			return
		}

		if !m.mcpClient.IsInitialized() {
			c.Next()
			return
		}

		serverStatuses := m.mcpClient.GetAllServerStatuses()
		hasAvailableServers := false
		for _, status := range serverStatuses {
			if status == mcp.ServerStatusAvailable {
				hasAvailableServers = true
				break
			}
		}

		if !hasAvailableServers {
			m.logger.Debug("no mcp servers currently available, skipping mcp tool injection")
			c.Next()
			return
		}

		availableTools := m.mcpClient.GetAllChatCompletionTools()
		if len(availableTools) == 0 {
			c.Next()
			return
		}
		m.logger.Debug("added mcp tools to request", "tool_count", len(availableTools))
		originalRequestBody.Tools = &availableTools

		c.Set(string(mcpBypassKey), &originalRequestBody)

		result, err := m.getProviderAndModel(c, originalRequestBody.Model)
		if err != nil {
			if result == nil || result.ProviderID == nil {
				m.logger.Error("failed to determine provider", err, "model", originalRequestBody.Model)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported model: %s", originalRequestBody.Model)})
				c.Abort()
				return
			}

			if result.Provider == nil {
				m.logger.Error("failed to get provider", err, "provider", *result.ProviderID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not available"})
				c.Abort()
				return
			}
		}

		if originalRequestBody.Stream != nil && *originalRequestBody.Stream {
			m.logger.Debug("starting mcp streaming mode")
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Transfer-Encoding", "chunked")

			if err := m.handleMCPStreamingRequest(c, &originalRequestBody, result); err != nil {
				m.logger.Error("failed to handle mcp streaming", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP streaming failed"})
				c.Abort()
				return
			}
			c.Abort()
			return
		}

		customWriter := &customResponseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
			writeToClient:  false,
		}
		c.Writer = customWriter

		c.Next()

		var response providers.CreateChatCompletionResponse
		if err := json.Unmarshal(customWriter.body.Bytes(), &response); err != nil {
			m.logger.Error("failed to parse response body", err)
			m.writeErrorResponse(c, customWriter, "Failed to parse response", http.StatusInternalServerError)
			return
		}

		if len(response.Choices) > 0 && response.Choices[0].Message.ToolCalls != nil {
			if err := m.handleMCPToolCalls(c, &response, &originalRequestBody, result); err != nil {
				m.logger.Error("failed to handle mcp tool calls", err)
				m.writeErrorResponse(c, customWriter, "Failed to execute MCP tools", http.StatusInternalServerError)
				return
			}
		}

		m.writeResponse(c, customWriter, response)
	}
}

// getProviderAndModel determines the provider and model from the request model string or query parameter
func (m *MCPMiddlewareImpl) getProviderAndModel(c *gin.Context, model string) (*MCPProviderModelResult, error) {
	if providerID := providers.Provider(c.Query("provider")); providerID != "" {
		provider, err := m.registry.BuildProvider(providerID, m.inferenceGatewayClient)
		if err != nil {
			return &MCPProviderModelResult{ProviderID: &providerID}, fmt.Errorf("failed to build provider: %w", err)
		}

		return &MCPProviderModelResult{
			Provider:      provider,
			ProviderModel: model,
			ProviderID:    &providerID,
		}, nil
	}

	providerPtr, providerModel := providers.DetermineProviderAndModelName(model)
	if providerPtr == nil {
		return &MCPProviderModelResult{ProviderID: nil}, fmt.Errorf("unable to determine provider for model: %s. Please specify a provider using the ?provider= query parameter or use the provider/model format", model)
	}

	provider, err := m.registry.BuildProvider(*providerPtr, m.inferenceGatewayClient)
	if err != nil {
		return &MCPProviderModelResult{ProviderID: providerPtr}, fmt.Errorf("failed to build provider: %w", err)
	}

	return &MCPProviderModelResult{
		Provider:      provider,
		ProviderModel: providerModel,
		ProviderID:    providerPtr,
	}, nil
}

// handleMCPStreamingRequest handles streaming requests with MCP agent
func (m *MCPMiddlewareImpl) handleMCPStreamingRequest(c *gin.Context, request *providers.CreateChatCompletionRequest, result *MCPProviderModelResult) error {
	m.mcpAgent.SetProvider(result.Provider)
	m.mcpAgent.SetModel(&result.ProviderModel)

	processedChunk := make(chan []byte, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(processedChunk)
		err := m.mcpAgent.RunWithStream(c.Request.Context(), processedChunk, c, request)
		if err != nil {
			m.logger.Error("mcp agent streaming failed", err)
			errCh <- err
		}
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case line, ok := <-processedChunk:
			if !ok {
				m.logger.Debug("mcp agent stream channel closed unexpectedly")
				return false
			}

			if bytes.Equal(line, []byte("data: [DONE]\n\n")) {
				m.logger.Debug("mcp agent completed all iterations, sending [DONE]")
				_, err := w.Write(line)
				if err != nil {
					m.logger.Error("failed to write [DONE] to client", err)
				}
				return false
			}

			m.logger.Debug("processed chunk", "line", string(line))

			if strings.HasPrefix(string(line), "data: {") && strings.Contains(string(line), "\"error\"") {
				var errMsg struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(line[6:], &errMsg); err == nil {
					m.logger.Error("upstream provider error", fmt.Errorf("%s", errMsg.Error))
					c.Writer.WriteHeader(http.StatusServiceUnavailable)
				}
			}

			_, err := w.Write(line)
			if err != nil {
				m.logger.Error("failed to write line to client", err)
				return false
			}
			return true
		case err := <-errCh:
			m.logger.Error("mcp agent streaming error", err)
			c.Writer.WriteHeader(http.StatusServiceUnavailable)
			if _, writeErr := fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error()); writeErr != nil {
				m.logger.Error("failed to write error to stream", writeErr)
			}
			return false
		case <-c.Request.Context().Done():
			m.logger.Debug("request context done, stopping stream")
			return false
		}
	})
	return nil
}

// handleMCPToolCalls executes MCP tool calls using the injected agent
func (m *MCPMiddlewareImpl) handleMCPToolCalls(c *gin.Context, response *providers.CreateChatCompletionResponse, originalRequest *providers.CreateChatCompletionRequest, result *MCPProviderModelResult) error {
	m.mcpAgent.SetProvider(result.Provider)
	m.mcpAgent.SetModel(&result.ProviderModel)

	if err := m.mcpAgent.Run(c.Request.Context(), originalRequest, response); err != nil {
		return fmt.Errorf("mcp agent processing failed: %w", err)
	}

	m.logger.Debug("mcp agent processing completed successfully")
	return nil
}

// writeErrorResponse writes an error response to the client
func (m *MCPMiddlewareImpl) writeErrorResponse(c *gin.Context, customWriter *customResponseWriter, message string, statusCode int) {
	errorResponse := map[string]string{"error": message}
	customWriter.statusCode = statusCode
	m.writeResponse(c, customWriter, errorResponse)
}

// writeResponse writes the response to the client
func (m *MCPMiddlewareImpl) writeResponse(c *gin.Context, customWriter *customResponseWriter, response interface{}) {
	customWriter.writeToClient = true
	c.Writer = customWriter.ResponseWriter
	c.JSON(customWriter.statusCode, response)
}
