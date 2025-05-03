package mcp

import (
	"context"
	"fmt"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"

	"github.com/inference-gateway/inference-gateway/logger"
)

// MCPToolDefinition represents a tool definition discovered from an MCP server
type MCPToolDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Parameters  MCPToolParameters `json:"parameters"`
}

// MCPToolParameters defines the parameter schema for an MCP tool
type MCPToolParameters struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// MCPToolContent represents content returned by a tool execution
type MCPToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult represents the result of a tool execution
type MCPToolResult struct {
	Content []MCPToolContent `json:"content"`
}

// MCPToolResponse represents a tool response to be added to messages
type MCPToolResponse struct {
	Role       string `json:"role"`
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// MCPToolFunction represents the function part of a tool call
type MCPToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// MCPToolCall represents a tool call from the LLM
type MCPToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function MCPToolFunction `json:"function"`
}

// MCPClientInterface defines the interface for MCP client implementations
//
//go:generate mockgen -source=client.go -destination=../tests/mocks/mcp_client.go -package=mocks
type MCPClientInterface interface {
	// InitializeAll establishes connection with MCP servers and performs handshake
	InitializeAll(ctx context.Context) error

	// IsInitialized returns whether the client has been successfully initialized
	IsInitialized() bool

	// DiscoverCapabilities retrieves capabilities from all configured MCP servers
	DiscoverCapabilities(ctx context.Context) ([]map[string]interface{}, error)

	// ExecuteTool invokes a tool on the appropriate MCP server
	ExecuteTool(ctx context.Context, toolName string, params interface{}, serverURL string) (map[string]interface{}, error)

	// StreamChatWithTools sends a chat request with tool capabilities and streams the response
	StreamChatWithTools(ctx context.Context, messages []map[string]interface{}, serverURL string, callback func(chunk map[string]interface{}) error) error

	// GetServerCapabilities returns the server capabilities map
	GetServerCapabilities() map[string]map[string]interface{}
}

// MCPClient provides methods to interact with MCP servers
type MCPClient struct {
	ServerURLs         []string
	Clients            map[string]*mcp.Client
	Logger             logger.Logger
	ServerCapabilities map[string]map[string]interface{}
	Initialized        bool
}

// NewMCPClient is a variable holding the function to create a new MCP client
// This allows for overriding in tests
func NewMCPClient(serverURLs []string, logger logger.Logger) MCPClientInterface {
	return &MCPClient{
		ServerURLs:         serverURLs,
		Clients:            make(map[string]*mcp.Client),
		Logger:             logger,
		ServerCapabilities: make(map[string]map[string]interface{}),
		Initialized:        false,
	}
}

// createTransport creates an HTTP transport for the MCP client
func (c *MCPClient) createTransport(serverURL string) (transport.Transport, error) {
	transportHttp := mcphttp.NewHTTPClientTransport(serverURL)

	return transportHttp, nil
}

// InitializeAll follows the MCP initialization handshake with all configured servers
// and provides detailed debug information about server capabilities
func (c *MCPClient) InitializeAll(ctx context.Context) error {
	c.Logger.Info("Starting initialization with MCP servers", "count", len(c.ServerURLs))

	for _, serverURL := range c.ServerURLs {
		c.Logger.Debug("Attempting to initialize MCP server", "server", serverURL)

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
		c.Logger.Debug("Successfully connected to MCP server", "server", serverURL)

		capabilities := map[string]interface{}{}
		capabilities["_server_url"] = serverURL

		if initResponse != nil {
			capabilities["initialized"] = true

			c.Logger.Debug("MCP server initialization response",
				"server", serverURL,
				"version", initResponse.ServerInfo.Version,
				"name", initResponse.ServerInfo.Name)

			if initResponse.ServerInfo.Version != "" {
				capabilities["version"] = initResponse.ServerInfo.Version
				c.Logger.Info("MCP server version", "server", serverURL, "version", initResponse.ServerInfo.Version)
			}

			if initResponse.ServerInfo.Name != "" {
				capabilities["name"] = initResponse.ServerInfo.Name
				c.Logger.Info("MCP server name", "server", serverURL, "name", initResponse.ServerInfo.Name)
			}

			if len(initResponse.Meta) > 0 {
				c.Logger.Debug("MCP server metadata received", "server", serverURL, "count", len(initResponse.Meta))
				capabilities["metadata"] = initResponse.Meta

				for key, value := range initResponse.Meta {
					c.Logger.Debug("MCP server metadata entry", "server", serverURL, "key", key, "value", value)
				}
			}
		}

		c.ServerCapabilities[serverURL] = capabilities
		c.Logger.Info("Server capabilities registered", "server", serverURL)
	}

	if len(c.Clients) == 0 {
		c.Logger.Error("Failed to initialize any MCP servers", fmt.Errorf("no servers initialized"), "attempted", len(c.ServerURLs))
		return fmt.Errorf("failed to initialize any MCP servers")
	}

	c.Logger.Info("MCP client initialization complete", "successful_connections", len(c.Clients))
	c.Initialized = true
	return nil
}

// DiscoverCapabilities queries MCP servers to discover their capabilities
func (c *MCPClient) DiscoverCapabilities(ctx context.Context) ([]map[string]interface{}, error) {
	if !c.Initialized {
		return nil, fmt.Errorf("MCP client not initialized properly")
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
