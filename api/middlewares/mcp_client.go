package middlewares

import (
	"context"
	"fmt"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"

	"github.com/inference-gateway/inference-gateway/logger"
)

// MCPClient provides methods to interact with MCP servers
type MCPClient struct {
	ServerURLs         []string
	Clients            map[string]*mcp.Client
	Logger             logger.Logger
	ServerCapabilities map[string]map[string]interface{}
	Initialized        bool
	AuthToken          string
}

// NewMCPClient is a variable holding the function to create a new MCP client
// This allows for overriding in tests
var NewMCPClient = func(serverURLs []string, authToken string, enableSSE bool, logger logger.Logger) MCPClientInterface {
	return &MCPClient{
		ServerURLs:         serverURLs,
		Clients:            make(map[string]*mcp.Client),
		Logger:             logger,
		ServerCapabilities: make(map[string]map[string]interface{}),
		Initialized:        false,
		AuthToken:          authToken,
	}
}

// createTransport creates an HTTP transport for the MCP client
func (c *MCPClient) createTransport(serverURL string) (transport.Transport, error) {
	transportHttp := mcphttp.NewHTTPClientTransport(serverURL)

	return transportHttp, nil
}

// Initialize follows the MCP initialization handshake with all configured servers
func (c *MCPClient) Initialize(ctx context.Context) error {
	for _, serverURL := range c.ServerURLs {
		transport, err := c.createTransport(serverURL)
		if err != nil {
			c.Logger.Error("Failed to create transport", err, "server", serverURL)
			continue
		}

		client := mcp.NewClient(transport)

		initResponse, err := client.Initialize(ctx)
		if err != nil {
			c.Logger.Error("Failed to initialize MCP server", err, "server", serverURL)
			continue
		}

		c.Clients[serverURL] = client

		capabilities := map[string]interface{}{}

		capabilities["_server_url"] = serverURL

		if initResponse != nil {
			capabilities["initialized"] = true
		}

		c.ServerCapabilities[serverURL] = capabilities
	}

	if len(c.Clients) == 0 {
		return fmt.Errorf("failed to initialize any MCP servers")
	}

	c.Initialized = true
	return nil
}

// DiscoverCapabilities queries MCP servers to discover their capabilities
func (c *MCPClient) DiscoverCapabilities(ctx context.Context) ([]map[string]interface{}, error) {
	if !c.Initialized {
		if err := c.Initialize(ctx); err != nil {
			return nil, err
		}
	}

	var allCapabilities []map[string]interface{}

	for serverURL, client := range c.Clients {
		serverCapabilities := c.ServerCapabilities[serverURL]

		toolsResponse, err := client.ListTools(ctx, nil)
		if err != nil {
			c.Logger.Error("Failed to list tools", err, "server", serverURL)
			continue
		}

		toolsList := make([]interface{}, 0, len(toolsResponse.Tools))
		for _, tool := range toolsResponse.Tools {
			toolMap := map[string]interface{}{
				"name": tool.Name,
			}

			if tool.Description != nil {
				toolMap["description"] = *tool.Description
			}

			toolMap["parameters"] = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}

			toolsList = append(toolsList, toolMap)
		}

		if len(toolsList) > 0 {
			serverCapabilities["tools"] = toolsList
		}

		allCapabilities = append(allCapabilities, serverCapabilities)
	}

	if len(allCapabilities) == 0 {
		return nil, fmt.Errorf("failed to discover capabilities from any MCP server")
	}

	return allCapabilities, nil
}

// ExecuteTool invokes a tool on the appropriate MCP server using JSON-RPC
func (c *MCPClient) ExecuteTool(ctx context.Context, toolName string, params interface{}, serverURL string) (map[string]interface{}, error) {
	client, ok := c.Clients[serverURL]
	if !ok {
		return nil, fmt.Errorf("no client found for server: %s", serverURL)
	}

	response, err := client.CallTool(ctx, toolName, params)
	if err != nil {
		return nil, fmt.Errorf("tool execution error: %w", err)
	}

	result := map[string]interface{}{}

	if response != nil {
		if len(response.Content) > 0 {
			contentList := make([]interface{}, 0, len(response.Content))

			for _, content := range response.Content {
				contentMap := map[string]interface{}{}

				if content.TextContent != nil {
					contentMap["type"] = "text"
					contentMap["text"] = content.TextContent.Text
				}

				if len(contentMap) > 0 {
					contentList = append(contentList, contentMap)
				}
			}

			if len(contentList) > 0 {
				result["content"] = contentList
			}
		}
	}

	return result, nil
}

// StreamChatWithTools sends a chat request with tool capabilities and streams the response
func (c *MCPClient) StreamChatWithTools(ctx context.Context, messages []map[string]interface{}, serverURL string, callback func(chunk map[string]interface{}) error) error {
	client, ok := c.Clients[serverURL]
	if !ok {
		return fmt.Errorf("no client found for server: %s", serverURL)
	}

	mcpMessages := make([]mcp.PromptMessage, 0, len(messages))

	for _, msg := range messages {
		role, ok := msg["role"].(string)
		if !ok {
			return fmt.Errorf("message missing required 'role' field of type string")
		}

		var contentItems []*mcp.Content

		switch contentVal := msg["content"].(type) {
		case string:
			contentItems = append(contentItems, &mcp.Content{
				TextContent: &mcp.TextContent{
					Text: contentVal,
				},
			})
		case []interface{}:
			for _, item := range contentVal {
				if contentMap, ok := item.(map[string]interface{}); ok {
					if contentType, ok := contentMap["type"].(string); ok {
						switch contentType {
						case "text":
							if text, ok := contentMap["text"].(string); ok {
								contentItems = append(contentItems, &mcp.Content{
									TextContent: &mcp.TextContent{
										Text: text,
									},
								})
							}
						default:
							c.Logger.Info("Unsupported content type in message", "type", contentType)
						}
					}
				}
			}
		default:
			return fmt.Errorf("message has unsupported 'content' field type: %T", msg["content"])
		}

		if len(contentItems) == 0 {
			return fmt.Errorf("failed to process message content")
		}

		mcpMessage := mcp.PromptMessage{
			Role:    mcp.Role(role),
			Content: contentItems[0],
		}

		mcpMessages = append(mcpMessages, mcpMessage)
	}

	promptArgs := map[string]interface{}{
		"messages": mcpMessages,
	}

	response, err := client.GetPrompt(ctx, "chat", promptArgs)
	if err != nil {
		c.Logger.Info("Error with prompt request, trying alternative approach", "error", err, "serverURL", serverURL)

		toolResponse, toolErr := client.CallTool(ctx, "chat", promptArgs)
		if toolErr != nil {
			return fmt.Errorf("chat request failed: %w", toolErr)
		}

		result := map[string]interface{}{}

		if toolResponse != nil && len(toolResponse.Content) > 0 {
			contentList := make([]interface{}, 0, len(toolResponse.Content))

			for _, content := range toolResponse.Content {
				if content != nil && content.TextContent != nil {
					contentMap := map[string]interface{}{
						"type": "text",
						"text": content.TextContent.Text,
					}
					contentList = append(contentList, contentMap)
				}
			}

			if len(contentList) > 0 {
				result["content"] = contentList
			}
		}

		if len(result) > 0 {
			if err := callback(result); err != nil {
				return fmt.Errorf("callback error: %w", err)
			}
		}

		return nil
	}

	result := map[string]interface{}{}

	if response != nil && len(response.Messages) > 0 {
		contentList := make([]interface{}, 0)

		for _, message := range response.Messages {
			if message != nil && message.Content != nil && message.Content.TextContent != nil {
				contentMap := map[string]interface{}{
					"type": "text",
					"text": message.Content.TextContent.Text,
				}
				contentList = append(contentList, contentMap)
			}
		}

		if len(contentList) > 0 {
			result["content"] = contentList
		}
	}

	if len(result) > 0 {
		if err := callback(result); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}

	return nil
}

// IsInitialized returns whether the client has been successfully initialized
func (c *MCPClient) IsInitialized() bool {
	return c.Initialized
}

// GetServerCapabilities returns the server capabilities map
func (c *MCPClient) GetServerCapabilities() map[string]map[string]interface{} {
	return c.ServerCapabilities
}
