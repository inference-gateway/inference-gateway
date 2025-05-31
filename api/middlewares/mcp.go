package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/agent"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
)

const (
	// ChatCompletionsPath is the endpoint path for chat completions
	ChatCompletionsPath = "/v1/chat/completions"

	// MCPInternalHeader marks internal MCP requests to prevent middleware loops
	MCPInternalHeader = "X-MCP-Internal"
)

// contextKey is a custom type for context keys to avoid collisions
type mcpContextKey string

const (
	// mcpInternalKey is the context key for marking internal MCP requests
	mcpInternalKey mcpContextKey = MCPInternalHeader
)

// customResponseWriter captures the response body but doesn't write it
// to the client until we're ready, allowing us to intercept tool calls
type customResponseWriter struct {
	gin.ResponseWriter
	body          *bytes.Buffer
	statusCode    int
	writeToClient bool
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteHeader captures the status code but doesn't write it to the client
// unless writeToClient is true
func (w *customResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	if w.writeToClient {
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the response body but doesn't write it to the client
// unless writeToClient is true
func (w *customResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	if w.writeToClient {
		return w.ResponseWriter.Write(b)
	}
	return len(b), nil
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
	logger                 logger.Logger
	cfg                    config.Config
}

// NoopMCPMiddlewareImpl is a no-op implementation of MCPMiddleware
type NoopMCPMiddlewareImpl struct{}

// NewMCPMiddleware creates a new MCP middleware instance
func NewMCPMiddleware(registry providers.ProviderRegistry, inferenceGatewayClient providers.Client, mcpClient mcp.MCPClientInterface, logger logger.Logger, cfg config.Config) (MCPMiddleware, error) {
	if mcpClient == nil {
		logger.Info("MCP client is nil, using no-op middleware")
		return &NoopMCPMiddlewareImpl{}, nil
	}

	return &MCPMiddlewareImpl{
		registry:               registry,
		inferenceGatewayClient: inferenceGatewayClient,
		mcpClient:              mcpClient,
		logger:                 logger,
		cfg:                    cfg,
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
		// Check if the request is marked as internal to prevent loops
		if c.GetHeader(MCPInternalHeader) != "" {
			m.logger.Debug("MCP Middleware: Not an internal MCP call")
			c.Next()
			return
		}

		// Consider only the chat completions endpoint
		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		m.logger.Debug("MCP Middleware: MCP middleware invoked", "path", c.Request.URL.Path)
		var originalRequestBody providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&originalRequestBody); err != nil {
			m.logger.Error("MCP Middleware: Failed to parse request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			c.Abort()
			return
		}

		// Add MCP tools to the request if available
		if !m.mcpClient.IsInitialized() {
			c.Next()
			return
		}
		availableTools := m.mcpClient.GetAllChatCompletionTools()
		if len(availableTools) == 0 {
			c.Next()
			return
		}
		m.logger.Debug("MCP Middleware: Added MCP tools to request", "toolCount", len(availableTools))
		originalRequestBody.Tools = &availableTools

		// Mark the request as internal to prevent middleware loops
		c.Set(string(mcpInternalKey), &originalRequestBody)

		result, err := m.getProviderAndModel(c, originalRequestBody.Model)
		if err != nil {
			if result.ProviderID == nil {
				m.logger.Error("MCP Middleware: Failed to determine provider", err, "model", originalRequestBody.Model)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported model: %s", originalRequestBody.Model)})
			} else {
				m.logger.Error("MCP Middleware: Failed to get provider", err, "provider", *result.ProviderID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not available"})
			}
			c.Abort()
			return
		}

		m.logger.Debug("Using provider", "provider", result.ProviderID, "model", result.ProviderModel)

		agent := agent.NewAgent(m.logger, m.mcpClient, result.Provider, result.ProviderModel)

		// Streaming response handling
		if originalRequestBody.Stream != nil && *originalRequestBody.Stream {
			m.logger.Debug("Starting agent streaming mode")
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Transfer-Encoding", "chunked")

			processedChunk := make(chan []byte, 100)
			errCh := make(chan error, 1)

			// Start agent streaming in a goroutine
			go func() {
				defer close(processedChunk)
				err := agent.RunWithStream(c.Request.Context(), processedChunk, c, &originalRequestBody)
				if err != nil {
					m.logger.Error("MCP Middleware: Agent streaming failed", err)
					errCh <- err
				}
			}()

			// Stream response to client
			c.Stream(func(w io.Writer) bool {
				select {
				case line, ok := <-processedChunk:
					if !ok {
						m.logger.Debug("MCP Middleware: Agent stream channel closed unexpectedly")
						return false
					}

					if bytes.Equal(line, []byte("data: [DONE]\n\n")) {
						m.logger.Debug("MCP Middleware: Agent completed all iterations, sending [DONE]")
						_, err := w.Write(line)
						if err != nil {
							m.logger.Error("MCP Middleware: Failed to write [DONE] to client", err)
						}
						return false
					}

					m.logger.Debug("MCP Middleware: Received line from agent", "line", string(line))

					if strings.HasPrefix(string(line), "data: {") && strings.Contains(string(line), "\"error\"") {
						var errMsg struct {
							Error string `json:"error"`
						}
						if err := json.Unmarshal(line[6:], &errMsg); err == nil {
							m.logger.Error("MCP Middleware: Upstream provider error", fmt.Errorf(errMsg.Error))
							c.Writer.WriteHeader(http.StatusServiceUnavailable)
						}
					}

					_, err := w.Write(line)
					if err != nil {
						m.logger.Error("MCP Middleware: Failed to write line to client", err)
						return false
					}
					return true
				case err := <-errCh:
					m.logger.Error("MCP Middleware: Agent streaming error", err)
					c.Writer.WriteHeader(http.StatusServiceUnavailable)
					if _, writeErr := fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error()); writeErr != nil {
						m.logger.Error("MCP Middleware: Failed to write error to stream", writeErr)
					}
					return false
				case <-c.Request.Context().Done():
					m.logger.Debug("Request context done, stopping stream")
					return false
				}
			})
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

		// For non-streaming requests, we need to handle the response after the provider call
		// and iterate through the agent's tool calls if any until we get a final response
		c.Next()

		// non-streaming response handling
		m.logger.Debug("MCP Middleware: Non-streaming response, waiting for provider response")

		var response providers.CreateChatCompletionResponse
		err = json.Unmarshal(customWriter.body.Bytes(), &response)
		if err != nil {
			m.logger.Error("MCP Middleware: Failed to parse response body", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}
		m.logger.Debug("MCP Middleware: Parsed response from provider", "response", response)

		// Run the agent to process tool calls and get the final response
		err = agent.Run(c.Request.Context(), &originalRequestBody, &response)
		if err != nil {
			m.logger.Error("MCP Middleware: Agent failed", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Agent processing failed"})
			return
		}

		m.logger.Debug("MCP Middleware: Received final response from agent", "response", response)
		responseBytes, err := json.Marshal(response)
		if err != nil {
			m.logger.Error("MCP Middleware: Failed to marshal final response", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal response"})
			return
		}

		customWriter.writeToClient = true
		customWriter.ResponseWriter.Header().Set("Content-Type", "application/json")
		customWriter.ResponseWriter.WriteHeader(http.StatusOK)
		_, err = customWriter.ResponseWriter.Write(responseBytes)
		if err != nil {
			m.logger.Error("MCP Middleware: Failed to write response", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write response"})
			return
		}
	}
}

// ProviderModelResult contains the result of provider and model determination
type ProviderModelResult struct {
	Provider      providers.IProvider
	ProviderModel string
	ProviderID    *providers.Provider
}

// getProviderAndModel determines the provider and model from the request model string or query parameter
func (m *MCPMiddlewareImpl) getProviderAndModel(c *gin.Context, model string) (*ProviderModelResult, error) {
	if providerID := providers.Provider(c.Query("provider")); providerID != "" {
		provider, err := m.registry.BuildProvider(providerID, m.inferenceGatewayClient)
		if err != nil {
			return &ProviderModelResult{ProviderID: &providerID}, fmt.Errorf("failed to build provider: %w", err)
		}

		return &ProviderModelResult{
			Provider:      provider,
			ProviderModel: model,
			ProviderID:    &providerID,
		}, nil
	}

	providerPtr, providerModel := providers.DetermineProviderAndModelName(model)
	if providerPtr == nil {
		return &ProviderModelResult{ProviderID: nil}, fmt.Errorf("unable to determine provider for model: %s. Please specify a provider using the ?provider= query parameter or use the provider/model format", model)
	}

	provider, err := m.registry.BuildProvider(*providerPtr, m.inferenceGatewayClient)
	if err != nil {
		return &ProviderModelResult{ProviderID: providerPtr}, fmt.Errorf("failed to build provider: %w", err)
	}

	return &ProviderModelResult{
		Provider:      provider,
		ProviderModel: providerModel,
		ProviderID:    providerPtr,
	}, nil
}
