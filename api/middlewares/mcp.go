package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
		// Only process requests to the chat completions endpoint
		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		c.Set("use_mcp", true)

		m.logger.Debug("MCP Processing request for MCP enhancement")
		var requestBody providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			m.logger.Debug("Could not parse request body for MCP enhancement", "error", err.Error())
			c.Next()
			return
		}

		m.logger.Debug("MCP Checking if request is a streaming request")
		wantsSSE := strings.Contains(c.GetHeader("Accept"), "text/event-stream") && requestBody.Stream != nil && *requestBody.Stream
		m.logger.Debug("MCP Request is a streaming request:", wantsSSE)

		m.logger.Debug("MCP Checking if request contains messages")
		if len(requestBody.Messages) == 0 {
			m.logger.Debug("MCP No messages found in request, continuing without MCP enhancement")
			c.Next()
			return
		}

		m.logger.Debug("MCP Discovering MCP capabilities")
		ctx := c.Request.Context()
		capabilities, err := m.client.DiscoverCapabilities(ctx)
		if err != nil {
			m.logger.Error("Failed to discover MCP capabilities", err)
			c.Next()
			return
		}

		m.logger.Debug("MCP Extracting tools from capabilities")
		var toolsToAdd []mcp.MCPToolDefinition
		for _, capabilitySet := range capabilities {
			if tools, ok := capabilitySet["tools"].([]interface{}); ok {
				for _, tool := range tools {
					if toolMap, ok := tool.(map[string]interface{}); ok {
						toolDef := mcp.MCPToolDefinition{
							Parameters: mcp.MCPToolParameters{
								Type:       "object",
								Properties: make(map[string]interface{}),
							},
						}

						if name, ok := toolMap["name"].(string); ok {
							toolDef.Name = name
						}

						if desc, ok := toolMap["description"].(string); ok {
							toolDef.Description = desc
						}

						if params, ok := toolMap["parameters"].(map[string]interface{}); ok {
							if paramType, typeOk := params["type"].(string); typeOk {
								toolDef.Parameters.Type = paramType
							}
							if props, propsOk := params["properties"].(map[string]interface{}); propsOk {
								toolDef.Parameters.Properties = props
							}
						}

						toolsToAdd = append(toolsToAdd, toolDef)
					}
				}
			}
		}

		if len(toolsToAdd) > 0 {
			// Create ChatCompletionTool slice from MCP tools
			tools := make([]providers.ChatCompletionTool, 0, len(toolsToAdd))
			for _, tool := range toolsToAdd {
				chatTool := providers.ChatCompletionTool{
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.FunctionObject{
						Name: tool.Name,
						Parameters: &providers.FunctionParameters{
							"type":       tool.Parameters.Type,
							"properties": tool.Parameters.Properties,
						},
					},
				}

				if tool.Description != "" {
					chatTool.Function.Description = &tool.Description
				}

				tools = append(tools, chatTool)
			}

			// Store the original request for later use
			originalRequestBody := requestBody

			// Update the request with the tools
			requestBody.Tools = &tools
			c.Set("original_request", originalRequestBody)

			// Convert to JSON and recreate the body reader
			c.Request.Body = createReadCloser(requestBody)
		}

		c.Next()

		if wantsSSE {
			m.processStreamResponse(c)
		} else {
			m.processNonStreamResponse(c)
		}
	}
}

// createReadCloser creates a ReadCloser from a struct to rebuild the request body
func createReadCloser(body interface{}) io.ReadCloser {
	jsonBody, _ := json.Marshal(body)
	return io.NopCloser(bytes.NewReader(jsonBody))
}

// processStreamResponse handles streaming responses and intercepts tool calls
func (m *MCPMiddlewareImpl) processStreamResponse(_ *gin.Context) {
	// Implementation for streaming responses would be more complex
	// It would need to intercept the SSE stream, check for tool calls,
	// and handle them appropriately
	// This would typically involve modifying the response writer
	m.logger.Debug("MCP streaming response processing not fully implemented yet")
}

// processNonStreamResponse handles non-streaming responses and intercepts tool calls
func (m *MCPMiddlewareImpl) processNonStreamResponse(c *gin.Context) {
	// Check if we have a response body to process
	responseData, exists := c.Get("response_data")
	if !exists {
		return
	}

	response, ok := responseData.(map[string]interface{})
	if !ok {
		m.logger.Debug("Could not parse response data for MCP processing")
		return
	}

	// Check for tool calls in the response
	choices, hasChoices := response["choices"].([]interface{})
	if !hasChoices || len(choices) == 0 {
		return
	}

	// Process each choice for tool calls
	toolResponses := make([]interface{}, 0)
	toolCallFound := false

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		message, hasMessage := choiceMap["message"].(map[string]interface{})
		if !hasMessage {
			continue
		}

		toolCalls, hasToolCalls := message["tool_calls"].([]interface{})
		if !hasToolCalls || len(toolCalls) == 0 {
			continue
		}

		// We have tool calls, process them
		toolCallFound = true
		for _, toolCall := range toolCalls {
			var mcpToolCall mcp.MCPToolCall

			toolCallMap, ok := toolCall.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract ID
			if id, ok := toolCallMap["id"].(string); ok {
				mcpToolCall.ID = id
			} else {
				continue
			}

			// Extract Type
			if typeVal, ok := toolCallMap["type"].(string); ok {
				mcpToolCall.Type = typeVal
			} else {
				mcpToolCall.Type = "function"
			}

			// Extract Function details
			functionMap, hasFunction := toolCallMap["function"].(map[string]interface{})
			if !hasFunction {
				continue
			}

			if name, ok := functionMap["name"].(string); ok {
				mcpToolCall.Function.Name = name
			} else {
				continue
			}

			if args, ok := functionMap["arguments"].(string); ok {
				mcpToolCall.Function.Arguments = args
			} else {
				continue
			}

			// Parse arguments
			var params interface{}
			if err := json.Unmarshal([]byte(mcpToolCall.Function.Arguments), &params); err != nil {
				m.logger.Error("Failed to parse tool call arguments", err)
				continue
			}

			// Find appropriate MCP server for this tool
			var serverURL string
			for url, capabilities := range m.client.GetServerCapabilities() {
				if tools, ok := capabilities["tools"].([]interface{}); ok {
					for _, tool := range tools {
						if toolMap, ok := tool.(map[string]interface{}); ok {
							if name, ok := toolMap["name"].(string); ok && name == mcpToolCall.Function.Name {
								serverURL = url
								break
							}
						}
					}
				}
				if serverURL != "" {
					break
				}
			}

			if serverURL == "" {
				m.logger.Error("Failed to execute tool", fmt.Errorf("no MCP server found for tool: %s", mcpToolCall.Function.Name))
				continue
			}

			// Execute the tool
			result, err := m.client.ExecuteTool(c.Request.Context(), mcpToolCall.Function.Name, params, serverURL)
			if err != nil {
				m.logger.Error("Failed to execute tool", err, "tool", mcpToolCall.Function.Name)
				continue
			}

			// Process the tool result to extract content
			var contentStr string

			if contentArray, ok := result["content"].([]interface{}); ok && len(contentArray) > 0 {
				// Handle case where content is an array of objects
				contents := make([]string, 0, len(contentArray))
				for _, item := range contentArray {
					if contentItem, ok := item.(map[string]interface{}); ok {
						if text, ok := contentItem["text"].(string); ok {
							contents = append(contents, text)
						} else if itemType, typeOk := contentItem["type"].(string); typeOk && itemType == "text" {
							if text, ok := contentItem["text"].(string); ok {
								contents = append(contents, text)
							}
						}
					}
				}
				contentStr = strings.Join(contents, "\n")
			} else {
				// Fall back to string conversion of the entire result
				contentStr = fmt.Sprintf("%v", result)
			}

			// Add tool result to the response
			toolResponseMap := map[string]interface{}{
				"role":         "tool",
				"tool_call_id": mcpToolCall.ID,
				"content":      contentStr,
			}

			// Append tool response to toolResponses
			toolResponses = append(toolResponses, toolResponseMap)
		}
	}

	// Only add messages if we have tool responses and if tool calls were found
	if toolCallFound {
		response["messages"] = toolResponses
	}

	// Update the response in the context
	c.Set("response_data", response)
}

// Middleware returns a HTTP middleware handler (no-op implementation)
func (m *NoopMCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
