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

		m.logger.Debug("MCP: Discovering available tools")
		capabilities := m.client.GetServerCapabilities()

		var toolsToAdd []mcp.MCPToolDefinition
		for serverURL, capabilitySet := range capabilities {
			toolsData := capabilitySet.Tools
			var toolsArray []interface{}

			if tools, ok := toolsData["tools"].([]interface{}); ok {
				toolsArray = tools
			} else if toolsBytes, err := json.Marshal(toolsData["tools"]); err == nil {
				if err = json.Unmarshal(toolsBytes, &toolsArray); err != nil {
					m.logger.Debug("MCP: Could not unmarshal tools", "error", err.Error())
				}
			}

			if len(toolsArray) == 0 {
				continue
			}

			for _, tool := range toolsArray {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					toolDef := mcp.MCPToolDefinition{
						Parameters: mcp.MCPToolParameters{
							Type:       "object",
							Properties: make(map[string]interface{}),
						},
						ServerURL: serverURL,
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

		if len(toolsToAdd) > 0 {
			tools := make([]providers.ChatCompletionTool, 0, len(toolsToAdd))
			toolDefinitions := make(map[string]mcp.MCPToolDefinition)

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
				toolDefinitions[tool.Name] = tool
			}

			m.logger.Debug("MCP: Adding tools to request", "toolCount", len(tools))

			c.Set("mcp_tool_definitions", toolDefinitions)
			c.Set("original_request", requestBody)

			requestBody.Tools = &tools

			c.Request.Body = createReadCloser(requestBody)
		}

		c.Next()

		m.processToolCalls(c)
	}
}

// processToolCalls handles response and intercepts tool calls
func (m *MCPMiddlewareImpl) processToolCalls(c *gin.Context) {
	responseData, exists := c.Get("response_data")
	if !exists {
		m.logger.Debug("MCP: No response data found")
		return
	}

	response, ok := responseData.(map[string]interface{})
	if !ok {
		m.logger.Debug("MCP: Could not parse response data")
		return
	}

	choices, hasChoices := response["choices"].([]interface{})
	if !hasChoices || len(choices) == 0 {
		m.logger.Debug("MCP: No choices found in response")
		return
	}

	toolDefs, hasDefs := c.Get("mcp_tool_definitions")
	if !hasDefs {
		m.logger.Debug("MCP: No tool definitions found")
		return
	}

	toolDefinitions, ok := toolDefs.(map[string]mcp.MCPToolDefinition)
	if !ok {
		m.logger.Debug("MCP: Could not parse tool definitions")
		return
	}

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

		toolCallFound = true
		for _, toolCall := range toolCalls {
			var mcpToolCall mcp.MCPToolCall

			toolCallMap, ok := toolCall.(map[string]interface{})
			if !ok {
				continue
			}

			if id, ok := toolCallMap["id"].(string); ok {
				mcpToolCall.ID = id
			} else {
				continue
			}

			if typeVal, ok := toolCallMap["type"].(string); ok {
				mcpToolCall.Type = typeVal
			} else {
				mcpToolCall.Type = "function"
			}

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

			var params interface{}
			if err := json.Unmarshal([]byte(mcpToolCall.Function.Arguments), &params); err != nil {
				m.logger.Error("MCP: Failed to parse tool call arguments", err)
				continue
			}

			toolDef, found := toolDefinitions[mcpToolCall.Function.Name]
			if !found {
				m.logger.Error("MCP: No server URL found for tool", nil, "tool", mcpToolCall.Function.Name)
				continue
			}

			m.logger.Debug("MCP: Executing tool call", "tool", mcpToolCall.Function.Name, "server", toolDef.ServerURL)
			mcpRequest := mcp.Request{
				Method: "tools/call",
				Params: map[string]interface{}{
					"name":      mcpToolCall.Function.Name,
					"arguments": params,
				},
			}

			result, err := m.client.ExecuteTool(c.Request.Context(), mcpRequest, toolDef.ServerURL)
			if err != nil {
				m.logger.Error("MCP: Failed to execute tool", err, "tool", mcpToolCall.Function.Name)
				continue
			}

			var contentStr string
			contents := make([]string, 0)

			contentArray := result.Content
			if len(contentArray) > 0 {
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

				if len(contents) > 0 {
					contentStr = strings.Join(contents, "\n")
				} else {
					contentBytes, _ := json.Marshal(contentArray)
					contentStr = string(contentBytes)
				}
			} else {
				contentBytes, _ := json.Marshal(result)
				contentStr = string(contentBytes)
			}

			toolResponseMap := map[string]interface{}{
				"role":         "tool",
				"tool_call_id": mcpToolCall.ID,
				"content":      contentStr,
			}

			toolResponses = append(toolResponses, toolResponseMap)
		}
	}

	if toolCallFound && len(toolResponses) > 0 {
		m.logger.Debug("MCP: Adding tool responses to response", "count", len(toolResponses))
		response["messages"] = toolResponses
	}

	c.Set("response_data", response)
}

// Middleware returns a HTTP middleware handler (no-op implementation)
func (m *NoopMCPMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// createReadCloser creates a ReadCloser from a struct to rebuild the request body
func createReadCloser(body interface{}) io.ReadCloser {
	jsonBody, _ := json.Marshal(body)
	return io.NopCloser(bytes.NewReader(jsonBody))
}
