package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// MCPBypassHeader marks internal MCP requests to prevent middleware loops
	MCPBypassHeader = "X-MCP-Bypass"
)

// mcpContextKey carries the MCP middleware's parsed request on the request
// context, replacing gin's per-request value store.
type mcpContextKey struct{}

// WithMCPRequest marks the request as MCP-originating and stores req (a
// *types.CreateChatCompletionRequest) for the chat completions handler.
func WithMCPRequest(ctx context.Context, req any) context.Context {
	return context.WithValue(ctx, mcpContextKey{}, req)
}

// MCPRequestFromContext returns the value stored by WithMCPRequest, nil when absent.
func MCPRequestFromContext(ctx context.Context) any {
	return ctx.Value(mcpContextKey{})
}

// MCPProviderModelResult contains the result of provider and model determination
type MCPProviderModelResult struct {
	Provider      core.IProvider
	ProviderModel string
	ProviderID    *types.Provider
}

// MCPMiddleware defines the interface for MCP middleware
type MCPMiddleware interface {
	Middleware() func(http.Handler) http.Handler
}

// MCPMiddlewareImpl implements the MCP middleware
type MCPMiddlewareImpl struct {
	registry               registry.ProviderRegistry
	inferenceGatewayClient client.Client
	mcpClient              mcp.MCPClientInterface
	mcpAgent               *mcp.Agent
	logger                 logger.Logger
	config                 config.Config
}

// NoopMCPMiddlewareImpl is a no-op implementation of MCPMiddleware
type NoopMCPMiddlewareImpl struct{}

// NewMCPMiddleware creates a new MCP middleware instance
func NewMCPMiddleware(providerRegistry registry.ProviderRegistry, inferenceGatewayClient client.Client, mcpClient mcp.MCPClientInterface, mcpAgent *mcp.Agent, log logger.Logger, cfg config.Config) (MCPMiddleware, error) {
	if mcpClient == nil {
		log.Info("mcp client is nil, using no-op middleware")
		return &NoopMCPMiddlewareImpl{}, nil
	}

	return &MCPMiddlewareImpl{
		registry:               providerRegistry,
		inferenceGatewayClient: inferenceGatewayClient,
		mcpClient:              mcpClient,
		mcpAgent:               mcpAgent,
		logger:                 log,
		config:                 cfg,
	}, nil
}

// Middleware returns the no-op middleware handler
func (n *NoopMCPMiddlewareImpl) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// Middleware returns the MCP middleware handler
func (m *MCPMiddlewareImpl) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(MCPBypassHeader) != "" {
				m.logger.Debug("skipping mcp middleware for internal call")
				next.ServeHTTP(w, r)
				return
			}

			if r.URL.Path != ChatCompletionsPath {
				next.ServeHTTP(w, r)
				return
			}

			m.logger.Debug("mcp middleware invoked", "path", r.URL.Path)
			var originalRequestBody types.CreateChatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&originalRequestBody); err != nil {
				m.logger.Error("failed to parse request body", err)
				WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
				return
			}
			r = r.WithContext(WithMCPRequest(r.Context(), &originalRequestBody))

			if !m.mcpClient.IsInitialized() {
				next.ServeHTTP(w, r)
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
				next.ServeHTTP(w, r)
				return
			}

			var availableTools []types.ChatCompletionTool
			if m.config.MCP.ToolMode == mcp.ToolModeDirect {
				availableTools = m.mcpClient.GetAllChatCompletionTools()
			} else {
				availableTools = m.mcpClient.GetSelectorTools()
			}
			if len(availableTools) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			if originalRequestBody.Tools != nil {
				availableTools = append(*originalRequestBody.Tools, availableTools...)
			}
			m.logger.Debug("added mcp tools to request", "tool_count", len(availableTools), "tool_mode", m.config.MCP.ToolMode)
			originalRequestBody.Tools = &availableTools

			result, err := m.getProviderAndModel(r, originalRequestBody.Model)
			if err != nil {
				if result == nil || result.ProviderID == nil {
					m.logger.Error("failed to determine provider", err, "model", originalRequestBody.Model)
					WriteJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Unsupported model: %s", originalRequestBody.Model)})
					return
				}

				if result.Provider == nil {
					m.logger.Error("failed to get provider", err, "provider", *result.ProviderID)
					WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Provider not available"})
					return
				}
			}

			if originalRequestBody.Stream != nil && *originalRequestBody.Stream {
				m.logger.Debug("starting mcp streaming mode")
				SetSSEHeaders(w)

				if err := m.handleMCPStreamingRequest(w, r, &originalRequestBody, result); err != nil {
					m.logger.Error("failed to handle mcp streaming", err)
					WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "MCP streaming failed"})
					return
				}
				return
			}

			customWriter := capture(w)

			next.ServeHTTP(customWriter, r)

			if customWriter.statusCode >= http.StatusBadRequest {
				customWriter.replayTo(w)
				return
			}

			var response types.CreateChatCompletionResponse
			if err := json.Unmarshal(customWriter.body.Bytes(), &response); err != nil {
				m.logger.Error("failed to parse response body", err)
				m.writeErrorResponse(w, customWriter, "Failed to parse response", http.StatusInternalServerError)
				return
			}

			if len(response.Choices) > 0 && response.Choices[0].Message.ToolCalls != nil {
				if err := m.handleMCPToolCalls(r, &response, &originalRequestBody, result); err != nil {
					m.logger.Error("failed to handle mcp tool calls", err)
					m.writeErrorResponse(w, customWriter, "Failed to execute MCP tools", http.StatusInternalServerError)
					return
				}
			}

			m.writeResponse(w, customWriter, response)
		})
	}
}

// getProviderAndModel determines the provider and model from the request model string or query parameter
func (m *MCPMiddlewareImpl) getProviderAndModel(r *http.Request, model string) (*MCPProviderModelResult, error) {
	providerID, providerModel, ok := routing.ResolveProvider(r.URL.Query().Get("provider"), model)
	if !ok {
		return &MCPProviderModelResult{ProviderID: nil}, fmt.Errorf("unable to determine provider for model: %s. Please specify a provider using the ?provider= query parameter or use the provider/model format", model)
	}

	provider, err := m.registry.BuildProvider(providerID, m.inferenceGatewayClient)
	if err != nil {
		return &MCPProviderModelResult{ProviderID: &providerID}, fmt.Errorf("failed to build provider: %w", err)
	}

	return &MCPProviderModelResult{
		Provider:      provider,
		ProviderModel: providerModel,
		ProviderID:    &providerID,
	}, nil
}

// handleMCPStreamingRequest handles streaming requests with MCP agent
func (m *MCPMiddlewareImpl) handleMCPStreamingRequest(w http.ResponseWriter, r *http.Request, request *types.CreateChatCompletionRequest, result *MCPProviderModelResult) error {
	processedChunk := make(chan []byte, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(processedChunk)
		err := m.mcpAgent.RunWithStream(r.Context(), result.Provider, result.ProviderModel, processedChunk, request)
		if err != nil {
			m.logger.Error("mcp agent streaming failed", err)
			errCh <- err
		}
	}()

	StreamResponse(w, func(out io.Writer) bool {
		select {
		case line, ok := <-processedChunk:
			if !ok {
				m.logger.Debug("mcp agent stream channel closed unexpectedly")
				return false
			}

			ResetWriteDeadline(w, m.config.Server.WriteTimeout)

			if bytes.Equal(line, []byte(types.SSEDoneEvent)) {
				m.logger.Debug("mcp agent completed all iterations, sending [DONE]")
				_, err := out.Write(line)
				if err != nil {
					m.logger.Error("failed to write [DONE] to client", err)
				}
				return false
			}

			m.logger.Debug("processed chunk", "line", string(line))

			data, hasData := bytes.CutPrefix(line, []byte(types.SSEDataPrefix))
			if hasData && bytes.HasPrefix(data, []byte("{")) && bytes.Contains(data, []byte("\"error\"")) {
				var errMsg struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(data, &errMsg); err == nil {
					m.logger.Error("upstream provider error", fmt.Errorf("%s", errMsg.Error))
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			}

			_, err := out.Write(line)
			if err != nil {
				m.logger.Error("failed to write line to client", err)
				return false
			}
			return true
		case err := <-errCh:
			m.logger.Error("mcp agent streaming error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, writeErr := out.Write(types.SSEErrorEvent(err.Error())); writeErr != nil {
				m.logger.Error("failed to write error to stream", writeErr)
			}
			return false
		case <-r.Context().Done():
			m.logger.Debug("request context done, stopping stream")
			return false
		}
	})
	return nil
}

// handleMCPToolCalls executes MCP tool calls using the injected agent
func (m *MCPMiddlewareImpl) handleMCPToolCalls(r *http.Request, response *types.CreateChatCompletionResponse, originalRequest *types.CreateChatCompletionRequest, result *MCPProviderModelResult) error {
	if err := m.mcpAgent.Run(r.Context(), result.Provider, result.ProviderModel, originalRequest, response); err != nil {
		return fmt.Errorf("mcp agent processing failed: %w", err)
	}

	m.logger.Debug("mcp agent processing completed successfully")
	return nil
}

// writeErrorResponse writes an error response to the client
func (m *MCPMiddlewareImpl) writeErrorResponse(w http.ResponseWriter, customWriter *customResponseWriter, message string, statusCode int) {
	errorResponse := map[string]string{"error": message}
	customWriter.statusCode = statusCode
	m.writeResponse(w, customWriter, errorResponse)
}

// writeResponse writes the response to the client
func (m *MCPMiddlewareImpl) writeResponse(w http.ResponseWriter, customWriter *customResponseWriter, response any) {
	WriteJSON(w, customWriter.statusCode, response)
}
