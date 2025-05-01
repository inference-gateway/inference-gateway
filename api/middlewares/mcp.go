package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	return &MCPMiddleware{
		MCPClient:     client,
		Logger:        logger,
		Enabled:       true,
		ToolServerMap: make(map[string]string),
	}, nil
}

// Middleware returns a HTTP middleware handler for MCP integration
func (m *MCPMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Enabled {
			c.Next()
			return
		}

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

		blw := &bodyLogWriter{ResponseWriter: c.Writer, body: bytes.NewBufferString(""), middleware: m}
		c.Writer = blw

		c.Next()

		if blw.body.Len() > 0 {
			contentType := blw.Header().Get("Content-Type")
			if strings.Contains(contentType, "application/json") {
				body := blw.body.Bytes()

				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err == nil {
					processedResponse, err := m.processToolCalls(response)
					if err == nil {
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
func (m *MCPMiddleware) ProcessToolCalls(response map[string]interface{}) (map[string]interface{}, error) {
	return m.processToolCalls(response)
}

// processToolCalls handles any tool calls in the response through MCP
func (m *MCPMiddleware) processToolCalls(response map[string]interface{}) (map[string]interface{}, error) {
	// Extract choices from the response
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return response, nil
	}

	// Get the first choice (we only handle the first one for now)
	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return response, nil
	}

	// Get the message
	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return response, nil
	}

	// Check for tool calls
	toolCalls, ok := message["tool_calls"].([]interface{})
	if !ok || len(toolCalls) == 0 {
		return response, nil
	}

	// Process each tool call
	for i, tc := range toolCalls {
		toolCall, ok := tc.(map[string]interface{})
		if !ok {
			continue
		}

		// Get the function
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

		// Get the server URL for this tool
		serverURL, ok := m.ToolServerMap[name]
		if !ok {
			m.Logger.Error("No server URL found for tool", nil, "tool", name)
			toolCalls[i].(map[string]interface{})["function"].(map[string]interface{})["response"] = fmt.Sprintf("Error: Tool '%s' not found in any MCP server", name)
			continue
		}

		// Execute the tool through MCP
		result, err := m.MCPClient.ExecuteTool(context.Background(), name, arguments, serverURL)
		if err != nil {
			m.Logger.Error("Failed to execute MCP tool", err, "tool", name)

			// Add an error result
			toolCalls[i].(map[string]interface{})["function"].(map[string]interface{})["response"] = fmt.Sprintf("Error executing tool: %v", err)
		} else {
			// Add the successful result
			resultBytes, _ := json.Marshal(result)
			toolCalls[i].(map[string]interface{})["function"].(map[string]interface{})["response"] = string(resultBytes)
		}
	}

	return response, nil
}

// EnhanceRequest modifies a request to include MCP capabilities
func (m *MCPMiddleware) EnhanceRequest(r *http.Request) error {
	if !m.Enabled {
		return nil
	}

	// Discover MCP server capabilities
	allCapabilities, err := m.MCPClient.DiscoverCapabilities(r.Context())
	if err != nil {
		m.Logger.Error("Failed to discover MCP capabilities", err)
		return err
	}

	// Extract and combine tools from all servers
	tools, err := m.extractToolsFromAllCapabilities(allCapabilities)
	if err != nil {
		m.Logger.Error("Failed to extract tools from capabilities", err)
		return err
	}

	// Read and modify the request body
	var requestBody map[string]interface{}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		return err
	}
	r.Body.Close()

	// Add the tools to the request
	requestBody["tools"] = tools

	// Create a new request body with the modified content
	modifiedBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	// Replace the request body
	r.Body = io.NopCloser(bytes.NewReader(modifiedBody))
	r.ContentLength = int64(len(modifiedBody))

	// Ensure proper Content-Type
	r.Header.Set("Content-Type", "application/json")

	return nil
}

// extractToolsFromAllCapabilities collects tools from all MCP servers
func (m *MCPMiddleware) extractToolsFromAllCapabilities(allCapabilities []map[string]interface{}) ([]map[string]interface{}, error) {
	tools := []map[string]interface{}{}

	// Clear the tool-server map before populating it
	m.ToolServerMap = make(map[string]string)

	for _, capabilities := range allCapabilities {
		serverURL, ok := capabilities["_server_url"].(string)
		if !ok {
			m.Logger.Error("Missing server URL in capabilities", nil)
			continue
		}

		toolsList, ok := capabilities["tools"].([]interface{})
		if !ok {
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

			// Store the tool -> server mapping
			m.ToolServerMap[name] = serverURL

			// Convert to the format expected by LLMs
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
