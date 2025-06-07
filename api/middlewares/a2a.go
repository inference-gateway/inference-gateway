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
	// A2AInternalHeader marks internal A2A requests to prevent middleware loops
	A2AInternalHeader = "X-A2A-Internal"
)

// a2aContextKey is a custom type for context keys to avoid collisions
type a2aContextKey string

const (
	// a2aInternalKey is the context key for marking internal A2A requests
	a2aInternalKey a2aContextKey = A2AInternalHeader
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
		if c.GetHeader(A2AInternalHeader) != "" {
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

		agentQueryTool := m.createAgentQueryTool()
		m.addToolToRequest(&originalRequestBody, agentQueryTool)

		m.logger.Debug("added a2a query tool to request", "total_tools", 1)

		c.Set(string(a2aInternalKey), &originalRequestBody)

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

			if err := m.a2aAgent.Run(c, &originalRequestBody, &response); err != nil {
				m.logger.Error("failed to handle a2a tool calls", err)
				m.writeErrorResponse(c, customWriter, "Failed to execute A2A tools", http.StatusInternalServerError)
				return
			}
		}

		m.writeResponse(c, customWriter, response)
	}
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

	description := fmt.Sprintf("Query an A2A agent's card to understand its capabilities and determine if it's suitable for a task.%s", agentsList)

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
