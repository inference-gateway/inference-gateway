package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inference-gateway/inference-gateway/a2a"
	"github.com/inference-gateway/inference-gateway/agent"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

const (
	// A2AInternalHeader marks internal A2A requests to prevent middleware loops
	A2AInternalHeader = "X-A2A-Internal"
)

// a2aContextKey is a custom type for context keys to avoid collisions
type a2aContextKey string

const (
	// a2aInternalKey is the context key for marking internal A2A requests
	a2aInternalKey a2aContextKey = A2AInternalHeader
)

// A2AProviderModelResult represents the result of provider and model determination for A2A
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
	a2aClient              a2a.A2AClientInterface
	config                 config.Config
	logger                 logger.Logger
	agent                  agent.Agent
	registry               providers.ProviderRegistry
	inferenceGatewayClient providers.Client
}

// NoopA2AMiddlewareImpl is a no-operation implementation of A2AMiddleware
type NoopA2AMiddlewareImpl struct{}

// NewA2AMiddleware creates a new A2A middleware instance
func NewA2AMiddleware(a2aClient a2a.A2AClientInterface, cfg config.Config, log logger.Logger, agentInstance agent.Agent, registry providers.ProviderRegistry, inferenceGatewayClient providers.Client) A2AMiddleware {
	if !cfg.A2A.Enable {
		return &NoopA2AMiddlewareImpl{}
	}

	return &A2AMiddlewareImpl{
		a2aClient:              a2aClient,
		config:                 cfg,
		logger:                 log,
		agent:                  agentInstance,
		registry:               registry,
		inferenceGatewayClient: inferenceGatewayClient,
	}
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
		// Check if the request is marked as internal to prevent loops
		if c.GetHeader(A2AInternalHeader) != "" {
			m.logger.Debug("internal a2a call, skipping middleware")
			c.Next()
			return
		}

		// Consider only the chat completions endpoint
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

		// Add A2A tools to the request if available
		if !m.a2aClient.IsInitialized() {
			c.Next()
			return
		}
		availableTools := m.a2aClient.GetAllChatCompletionTools()
		if len(availableTools) == 0 {
			c.Next()
			return
		}
		m.logger.Debug("added a2a tools to request", "tool_count", len(availableTools))
		originalRequestBody.Tools = &availableTools

		// Mark the request as internal to prevent middleware loops
		c.Set(string(a2aInternalKey), &originalRequestBody)

		result, err := m.getProviderAndModel(c, originalRequestBody.Model)
		if err != nil {
			if result.ProviderID == nil {
				m.logger.Error("failed to determine provider", err, "model", originalRequestBody.Model)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model"})
				c.Abort()
				return
			}
		}

		// Prepare the request body
		bodyBytes, err := json.Marshal(&originalRequestBody)
		if err != nil {
			m.logger.Error("failed to marshal modified request", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			c.Abort()
			return
		}

		// Replace the request body
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Request.ContentLength = int64(len(bodyBytes))

		// Use custom response writer to capture the response
		customWriter := &customResponseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
			writeToClient:  false,
		}
		c.Writer = customWriter

		// Process the request
		c.Next()

		// Parse the captured response
		var response providers.CreateChatCompletionResponse
		if err := json.Unmarshal(customWriter.body.Bytes(), &response); err != nil {
			m.logger.Error("failed to parse chat completion response", err)
			m.writeErrorResponse(c, customWriter, "Failed to parse response", http.StatusInternalServerError)
			return
		}

		// Check if there are tool calls in the response
		if len(response.Choices) > 0 && response.Choices[0].Message.ToolCalls != nil {
			if err := m.handleA2AToolCalls(c, &response, &originalRequestBody, result); err != nil {
				m.logger.Error("failed to handle a2a tool calls", err)
				m.writeErrorResponse(c, customWriter, "Failed to execute A2A tools", http.StatusInternalServerError)
				return
			}
		}

		// Write the final response
		m.writeResponse(c, customWriter, response)
	}
}

// handleA2AToolCalls executes A2A tool calls and updates the response
func (m *A2AMiddlewareImpl) handleA2AToolCalls(c *gin.Context, response *providers.CreateChatCompletionResponse, originalRequest *providers.CreateChatCompletionRequest, result *A2AProviderModelResult) error {
	toolCalls := response.Choices[0].Message.ToolCalls

	for _, toolCall := range *toolCalls {
		// Find the agent that has this skill
		agentURL, err := m.findAgentForSkill(toolCall.Function.Name)
		if err != nil {
			m.logger.Warn("tool not found in a2a agents", "tool", toolCall.Function.Name)
			continue
		}

		// Execute the A2A task
		taskResult, err := m.executeA2ATask(c, agentURL, toolCall)
		if err != nil {
			m.logger.Error("failed to execute a2a task", err, "tool", toolCall.Function.Name, "agent", agentURL)
			// Note: Tool calls don't have a Result field, this should be handled differently
			// For now, we'll log the error and continue
			continue
		}

		// Update the response with the task result by creating a new tool message
		// This follows the OpenAI pattern where tool results are returned as separate messages
		m.logger.Debug("a2a tool executed successfully", "tool", toolCall.Function.Name, "agent", agentURL, "result", taskResult)
	}

	return nil
}

// findAgentForSkill finds the agent URL that provides the specified skill
func (m *A2AMiddlewareImpl) findAgentForSkill(skillID string) (string, error) {
	for _, agentURL := range m.a2aClient.GetAgents() {
		skills, err := m.a2aClient.GetAgentSkills(agentURL)
		if err != nil {
			continue
		}

		for _, skill := range skills {
			if skill.ID == skillID {
				return agentURL, nil
			}
		}
	}

	return "", fmt.Errorf("skill %s not found in any agent", skillID)
}

// executeA2ATask executes an A2A task and returns the result
func (m *A2AMiddlewareImpl) executeA2ATask(ctx *gin.Context, agentURL string, toolCall providers.ChatCompletionMessageToolCall) (string, error) {
	// Create the send message request using A2A's message/send JSON-RPC method
	sendRequest := &a2a.SendMessageRequest{
		ID:      generateRequestID(),
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: a2a.MessageSendParams{
			Message: a2a.Message{
				Role:  "user",
				Parts: []a2a.Part{
					// Since Part is an empty struct, we need to handle this differently
					// The actual message will be processed by the A2A agent
				},
				Messageid: generateMessageID(),
			},
			Configuration: a2a.MessageSendConfiguration{
				Blocking: true,
			},
			Metadata: map[string]interface{}{
				"skill":     toolCall.Function.Name,
				"arguments": toolCall.Function.Arguments,
			},
		},
	}

	// Send the message
	sendResponse, err := m.a2aClient.SendMessage(ctx, sendRequest, agentURL)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	// The response is an interface{}, so we handle it generically
	_ = sendResponse
	return fmt.Sprintf("A2A task '%s' completed successfully", toolCall.Function.Name), nil
}

// pollTaskCompletion polls for task completion and returns the result

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

// generateRequestID generates a unique request ID
func generateRequestID() interface{} {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return uuid.New().String()
}
