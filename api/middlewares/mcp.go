package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/logger"
)

// Key for MCP usage in request context
type mcpContextKey string

const usingMCPKey mcpContextKey = "using_mcp"

// MCPMiddleware adds Model Context Protocol capabilities to LLM requests
type MCPMiddleware struct {
	MCPClient     MCPClientInterface
	Logger        logger.Logger
	Enabled       bool
	ToolServerMap map[string]string
}

// NewMCPMiddleware creates a new middleware instance for MCP integration
func NewMCPMiddleware(serverURLs []string, logger logger.Logger) (*MCPMiddleware, error) {
	if len(serverURLs) == 0 {
		return nil, fmt.Errorf("no MCP server URLs provided")
	}

	client := NewMCPClient(serverURLs, "", true, logger)

	if err := client.Initialize(context.Background()); err != nil {
		logger.Error("Failed to initialize MCP client", err)
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return &MCPMiddleware{
		MCPClient:     client,
		Logger:        logger,
		Enabled:       true,
		ToolServerMap: make(map[string]string),
	}, nil
}

// Middleware returns a HTTP middleware handler for MCP integration
// It captures the request and response, modifies them as needed, and handles tool calls
// It also sets the context to indicate that MCP is being used
// If MCP is not enabled, it simply calls the next handler in the chain
// If there is no tool call, it proceeds with the original response
// If it didn't find any tool calls, it returns the original response
// It informs the client if there is a tool call in progress via notification or SSE
func (m *MCPMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Enabled {
			c.Next()
			return
		}

		wantsSSE := strings.Contains(c.GetHeader("Accept"), "text/event-stream")

		err := m.EnhanceRequest(c.Request)
		if err != nil {
			m.Logger.Error("Failed to enhance request with MCP capabilities", err)
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to process MCP capabilities",
			})
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), usingMCPKey, true)
		c.Request = c.Request.WithContext(ctx)

		blw := &bodyLogWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
			middleware:     m,
			wantsSSE:       wantsSSE,
			context:        c,
		}
		c.Writer = blw

		c.Next()

		if blw.body.Len() > 0 {
			contentType := blw.Header().Get("Content-Type")
			if strings.Contains(contentType, "application/json") {
				body := blw.body.Bytes()

				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err == nil {
					processedResponse, err := m.ProcessToolCalls(response, c)
					if err == nil {
						if !reflect.DeepEqual(response, processedResponse) && wantsSSE {
							c.Header("Content-Type", "text/event-stream")
							c.Header("Cache-Control", "no-cache")
							c.Header("Connection", "keep-alive")
							c.Header("Transfer-Encoding", "chunked")

							c.SSEvent("tool_call_complete", map[string]interface{}{
								"status": "complete",
							})
						}

						modifiedBody, err := json.Marshal(processedResponse)
						if err == nil {
							c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
							_, writeErr := c.Writer.Write(modifiedBody)
							if writeErr != nil {
								m.Logger.Error("Failed to write response body", writeErr)
							}
							return
						}
					}
				}
			}
		}
	}
}

// bodyLogWriter is a custom gin.ResponseWriter that captures the response body
type bodyLogWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	middleware *MCPMiddleware
	wantsSSE   bool
	context    *gin.Context
}

// Write captures the response being written
func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString captures the string being written
func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// ProcessToolCalls is the public version of processToolCalls for testing purposes
func (m *MCPMiddleware) ProcessToolCalls(response map[string]interface{}, sseContext ...*gin.Context) (map[string]interface{}, error) {
	return m.processToolCalls(response, sseContext...)
}

// ExtractToolsFromAllCapabilities is the public version of extractToolsFromAllCapabilities for testing purposes
func (m *MCPMiddleware) ExtractToolsFromAllCapabilities(allCapabilities []map[string]interface{}) ([]map[string]interface{}, error) {
	return m.extractToolsFromAllCapabilities(allCapabilities)
}

// processToolCalls handles any tool calls in the response through MCP
func (m *MCPMiddleware) processToolCalls(response map[string]interface{}, sseContext ...*gin.Context) (map[string]interface{}, error) {
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return response, nil
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return response, nil
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return response, nil
	}

	toolCalls, ok := message["tool_calls"].([]interface{})
	if !ok || len(toolCalls) == 0 {
		return response, nil
	}

	var ctx *gin.Context
	if len(sseContext) > 0 && sseContext[0] != nil {
		ctx = sseContext[0]
	}

	var messageContent string
	if content, ok := message["content"].(string); ok {
		messageContent = content
	}

	totalTools := len(toolCalls)
	for i, tc := range toolCalls {
		toolCall, ok := tc.(map[string]interface{})
		if !ok {
			continue
		}

		function, ok := toolCall["function"].(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := function["name"].(string)
		if !ok {
			continue
		}

		arguments, ok := function["arguments"].(string)
		if !ok {
			continue
		}

		if ctx != nil {
			ctx.SSEvent("tool_call_progress", map[string]interface{}{
				"status":    "in_progress",
				"tool_name": name,
				"progress":  float64(i) / float64(totalTools),
				"message":   fmt.Sprintf("Executing tool %s", name),
			})
			ctx.Writer.Flush()
		}

		serverURL, ok := m.ToolServerMap[name]
		if !ok {
			m.Logger.Error("No server URL found for tool", nil, "tool", name)
			errorMsg := fmt.Sprintf("Error: Tool '%s' not found in any MCP server", name)

			if messageContent != "" {
				messageContent += "\n\n"
			}
			messageContent += errorMsg
			continue
		}

		result, err := m.MCPClient.ExecuteTool(context.Background(), name, arguments, serverURL)
		if err != nil {
			m.Logger.Error("Failed to execute MCP tool", err, "tool", name)
			errorMsg := fmt.Sprintf("Error executing tool: %v", err)

			if messageContent != "" {
				messageContent += "\n\n"
			}
			messageContent += errorMsg

			if ctx != nil {
				ctx.SSEvent("tool_call_error", map[string]interface{}{
					"status":    "error",
					"tool_name": name,
					"message":   fmt.Sprintf("Error executing tool %s: %v", name, err),
				})
				ctx.Writer.Flush()
			}
		} else {
			var toolResult string
			if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
				for _, item := range content {
					if contentMap, ok := item.(map[string]interface{}); ok {
						if text, ok := contentMap["text"].(string); ok {
							if toolResult != "" {
								toolResult += "\n"
							}
							toolResult += text
						}
					}
				}
			}

			if messageContent != "" {
				messageContent += "\n\n"
			}
			messageContent += fmt.Sprintf("Tool '%s' result: %s", name, toolResult)

			if ctx != nil {
				ctx.SSEvent("tool_call_success", map[string]interface{}{
					"status":    "success",
					"tool_name": name,
					"progress":  float64(i+1) / float64(totalTools),
				})
				ctx.Writer.Flush()
			}
		}
	}

	message["content"] = messageContent

	if ctx != nil {
		ctx.SSEvent("tool_calls_complete", map[string]interface{}{
			"status":  "complete",
			"message": "All tool calls completed",
		})
		ctx.Writer.Flush()
	}

	return response, nil
}

// EnhanceRequest modifies a request to include MCP capabilities
func (m *MCPMiddleware) EnhanceRequest(r *http.Request) error {
	if !m.Enabled {
		return nil
	}

	allCapabilities, err := m.MCPClient.DiscoverCapabilities(r.Context())
	if err != nil {
		m.Logger.Error("Failed to discover MCP capabilities", err)
		return err
	}

	tools, err := m.extractToolsFromAllCapabilities(allCapabilities)
	if err != nil {
		m.Logger.Error("Failed to extract tools from capabilities", err)
		return err
	}

	var requestBody map[string]interface{}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		return err
	}
	r.Body.Close()

	requestBody["tools"] = tools

	modifiedBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	r.Body = io.NopCloser(bytes.NewReader(modifiedBody))
	r.ContentLength = int64(len(modifiedBody))

	r.Header.Set("Content-Type", "application/json")

	return nil
}

// extractToolsFromAllCapabilities collects tools from all MCP servers
func (m *MCPMiddleware) extractToolsFromAllCapabilities(allCapabilities []map[string]interface{}) ([]map[string]interface{}, error) {
	tools := []map[string]interface{}{}

	m.ToolServerMap = make(map[string]string)

	for _, capabilities := range allCapabilities {
		serverURL, ok := capabilities["_server_url"].(string)
		if !ok {
			m.Logger.Error("Missing server URL in capabilities", nil)
			continue
		}

		var toolsList []interface{}

		if tools, ok := capabilities["tools"].([]interface{}); ok {
			toolsList = tools
		} else if resources, ok := capabilities["resources"].(map[string]interface{}); ok {
			if tools, ok := resources["tools"].([]interface{}); ok {
				toolsList = tools
			}
		}

		if len(toolsList) == 0 {
			m.Logger.Error("No tools found in MCP capabilities", nil, "server", serverURL)
			continue
		}

		for _, toolInterface := range toolsList {
			tool, ok := toolInterface.(map[string]interface{})
			if !ok {
				continue
			}

			name, ok := tool["name"].(string)
			if !ok {
				continue
			}

			m.ToolServerMap[name] = serverURL

			formattedTool := map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        name,
					"description": tool["description"],
					"parameters":  tool["parameters"],
				},
			}
			tools = append(tools, formattedTool)
		}
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found in any MCP server")
	}

	return tools, nil
}
