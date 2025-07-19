package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/a2a"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

const (
	A2ABypassHeader = "X-A2A-Bypass"
)

// a2aContextKey is a custom type for context keys to avoid collisions
type a2aContextKey string

const (
	a2aBypassKey a2aContextKey = A2ABypassHeader
)

// A2AProviderModelResult contains the result of provider and model determination
type A2AProviderModelResult struct {
	Provider      providers.IProvider
	ProviderModel string
	ProviderID    *providers.Provider
}

// A2AMiddleware defines the interface for A2A middleware
type A2AMiddleware interface {
	Middleware() gin.HandlerFunc
}

// A2AMiddlewareImpl implements the A2A middleware
type A2AMiddlewareImpl struct {
	registry               providers.ProviderRegistry
	config                 config.Config
	a2aClient              a2a.A2AClientInterface
	a2aAgent               a2a.Agent
	logger                 logger.Logger
	inferenceGatewayClient providers.Client
}

// NoopA2AMiddlewareImpl is a no-operation implementation of A2AMiddleware
type NoopA2AMiddlewareImpl struct{}

// NewA2AMiddleware creates a new A2A middleware instance
func NewA2AMiddleware(registry providers.ProviderRegistry, a2aClient a2a.A2AClientInterface, a2aAgent a2a.Agent, log logger.Logger, inferenceGatewayClient providers.Client, cfg config.Config) (A2AMiddleware, error) {
	if !cfg.A2A.Enable {
		return &NoopA2AMiddlewareImpl{}, nil
	}

	return &A2AMiddlewareImpl{
		a2aClient:              a2aClient,
		a2aAgent:               a2aAgent,
		config:                 cfg,
		logger:                 log,
		registry:               registry,
		inferenceGatewayClient: inferenceGatewayClient,
	}, nil
}

// Middleware returns a no-op handler for the noop implementation
func (n *NoopA2AMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware returns the A2A middleware handler
func (m *A2AMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(A2ABypassHeader) != "" {
			m.logger.Debug("internal a2a call, skipping middleware")
			c.Next()
			return
		}

		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		m.logger.Debug("a2a middleware invoked", "path", c.Request.URL.Path)
		var originalRequestBody providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&originalRequestBody); err != nil {
			m.logger.Error("failed to parse request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			c.Abort()
			return
		}

		if !m.a2aClient.IsInitialized() {
			c.Next()
			return
		}

		// Check if any agents are currently available
		agentStatuses := m.a2aClient.GetAllAgentStatuses()
		hasAvailableAgents := false
		for _, status := range agentStatuses {
			if status == a2a.AgentStatusAvailable {
				hasAvailableAgents = true
				break
			}
		}

		// If no agents are available, continue without A2A tools but log for debugging
		if !hasAvailableAgents {
			m.logger.Debug("no a2a agents currently available, skipping a2a tool injection")
			c.Next()
			return
		}

		agentQueryTool := m.createAgentQueryTool()
		m.addToolToRequest(&originalRequestBody, agentQueryTool)

		taskSubmissionTool := m.createTaskSubmissionTool()
		m.addToolToRequest(&originalRequestBody, taskSubmissionTool)

		c.Set(string(a2aBypassKey), &originalRequestBody)

		result, err := m.getProviderAndModel(c, originalRequestBody.Model)
		if err != nil {
			if result == nil || result.ProviderID == nil {
				m.logger.Error("failed to determine provider", err, "model", originalRequestBody.Model)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model"})
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

		bodyBytes, err := json.Marshal(&originalRequestBody)
		if err != nil {
			m.logger.Error("failed to marshal modified request", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Request.ContentLength = int64(len(bodyBytes))

		if originalRequestBody.Stream != nil && *originalRequestBody.Stream {
			m.logger.Debug("starting a2a streaming mode")
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Transfer-Encoding", "chunked")

			if err := m.handleA2AStreamingRequest(c, &originalRequestBody, result); err != nil {
				m.logger.Error("failed to handle a2a streaming", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "A2A streaming failed"})
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
			m.logger.Error("failed to parse chat completion response", err)
			m.writeErrorResponse(c, customWriter, "Failed to parse response", http.StatusInternalServerError)
			return
		}

		if len(response.Choices) > 0 && response.Choices[0].Message.ToolCalls != nil {
			m.a2aAgent.SetProvider(result.Provider)
			m.a2aAgent.SetModel(&result.ProviderModel)

			if err := m.a2aAgent.Run(c.Request.Context(), &originalRequestBody, &response); err != nil {
				m.logger.Error("failed to handle a2a tool calls", err)
				m.writeErrorResponse(c, customWriter, "Failed to execute A2A tools", http.StatusInternalServerError)
				return
			}
		}

		m.writeResponse(c, customWriter, response)
	}
}

// handleA2AStreamingRequest handles streaming requests with A2A agent
func (m *A2AMiddlewareImpl) handleA2AStreamingRequest(c *gin.Context, request *providers.CreateChatCompletionRequest, result *A2AProviderModelResult) error {
	m.a2aAgent.SetProvider(result.Provider)
	m.a2aAgent.SetModel(&result.ProviderModel)

	processedChunk := make(chan []byte, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(processedChunk)
		err := m.a2aAgent.RunWithStream(c.Request.Context(), processedChunk, c, request)
		if err != nil {
			m.logger.Error("a2a agent streaming failed", err)
			errCh <- err
		}
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case line, ok := <-processedChunk:
			if !ok {
				m.logger.Debug("a2a agent stream channel closed unexpectedly")
				return false
			}

			if bytes.Equal(line, []byte("data: [DONE]\n\n")) {
				m.logger.Debug("a2a agent completed all iterations, sending [DONE]")
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
			m.logger.Error("a2a agent streaming error", err)
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

// getProviderAndModel determines the provider and model from the request model string or query parameter
func (m *A2AMiddlewareImpl) getProviderAndModel(c *gin.Context, model string) (*A2AProviderModelResult, error) {
	if providerID := providers.Provider(c.Query("provider")); providerID != "" {
		provider, err := m.registry.BuildProvider(providerID, m.inferenceGatewayClient)
		if err != nil {
			return &A2AProviderModelResult{ProviderID: &providerID}, fmt.Errorf("failed to build provider: %w", err)
		}

		return &A2AProviderModelResult{
			Provider:      provider,
			ProviderModel: model,
			ProviderID:    &providerID,
		}, nil
	}

	providerPtr, providerModel := providers.DetermineProviderAndModelName(model)
	if providerPtr == nil {
		return &A2AProviderModelResult{ProviderID: nil}, fmt.Errorf("unable to determine provider for model: %s. Please specify a provider using the ?provider= query parameter or use the provider/model format", model)
	}

	provider, err := m.registry.BuildProvider(*providerPtr, m.inferenceGatewayClient)
	if err != nil {
		return &A2AProviderModelResult{ProviderID: providerPtr}, fmt.Errorf("failed to build provider: %w", err)
	}

	return &A2AProviderModelResult{
		Provider:      provider,
		ProviderModel: providerModel,
		ProviderID:    providerPtr,
	}, nil
}

// createAgentQueryTool creates a tool that allows LLM to query agent cards
func (m *A2AMiddlewareImpl) createAgentQueryTool() providers.ChatCompletionTool {
	agents := m.a2aClient.GetAgents()
	var agentsList string
	if len(agents) > 0 {
		agentsList = fmt.Sprintf(" Available agents: %s.", strings.Join(agents, ", "))
	} else {
		agentsList = " No agents are currently available."
	}

	description := fmt.Sprintf("Query an A2A agent's card to understand its capabilities and determine if it's suitable for a task.%s \n\nIf you found the agent you are looking for, just query the agent card to find out more details.", agentsList)

	return providers.ChatCompletionTool{
		Type: providers.ChatCompletionToolTypeFunction,
		Function: providers.FunctionObject{
			Name:        "query_a2a_agent_card",
			Description: &description,
			Parameters: &providers.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_url": map[string]interface{}{
						"type":        "string",
						"description": fmt.Sprintf("The URL of the A2A agent to query. Available agents: %s", strings.Join(agents, ", ")),
					},
				},
				"required": []string{"agent_url"},
			},
		},
	}
}

// createTaskSubmissionTool creates a tool that allows LLM to submit tasks to A2A agents
func (m *A2AMiddlewareImpl) createTaskSubmissionTool() providers.ChatCompletionTool {
	agents := m.a2aClient.GetAgents()
	var agentsList string
	if len(agents) > 0 {
		agentsList = fmt.Sprintf(" Available agents: %s.", strings.Join(agents, ", "))
	} else {
		agentsList = " No agents are currently available."
	}

	description := fmt.Sprintf("Submit a task to an A2A agent for execution. The agent will use its skills to complete the task.%s", agentsList)

	return providers.ChatCompletionTool{
		Type: providers.ChatCompletionToolTypeFunction,
		Function: providers.FunctionObject{
			Name:        "submit_task_to_agent",
			Description: &description,
			Parameters: &providers.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_url": map[string]interface{}{
						"type":        "string",
						"description": fmt.Sprintf("The URL of the A2A agent to submit the task to. Available agents: %s", strings.Join(agents, ", ")),
					},
					"task_description": map[string]interface{}{
						"type":        "string",
						"description": "A clear description of the task you want the agent to perform",
					},
					"additional_context": map[string]interface{}{
						"type":        "string",
						"description": "Any additional context or parameters needed for the task (optional)",
					},
				},
				"required": []string{"agent_url", "task_description"},
			},
		},
	}
}

// addToolToRequest adds a single tool to the request
func (m *A2AMiddlewareImpl) addToolToRequest(request *providers.CreateChatCompletionRequest, tool providers.ChatCompletionTool) {
	if request.Tools == nil {
		request.Tools = &[]providers.ChatCompletionTool{tool}
	} else {
		tools := append(*request.Tools, tool)
		request.Tools = &tools
	}
}

// writeErrorResponse writes an error response to the client
func (m *A2AMiddlewareImpl) writeErrorResponse(c *gin.Context, customWriter *customResponseWriter, message string, statusCode int) {
	errorResponse := ErrorResponse{Error: message}
	customWriter.statusCode = statusCode
	m.writeResponse(c, customWriter, errorResponse)
}

// writeResponse writes the response to the client
func (m *A2AMiddlewareImpl) writeResponse(c *gin.Context, customWriter *customResponseWriter, response interface{}) {
	customWriter.writeToClient = true
	customWriter.WriteHeader(customWriter.statusCode)

	responseBytes, err := json.Marshal(response)
	if err != nil {
		m.logger.Error("failed to marshal final response", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if _, err := customWriter.Write(responseBytes); err != nil {
		m.logger.Error("failed to write response", err)
	}
}
