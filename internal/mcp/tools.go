package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ExecuteTool implements MCPClientInterface.
func (mc *MCPClient) ExecuteTool(ctx context.Context, request Request, serverURL string) (*CallToolResult, error) {
	if !mc.Initialized {
		return nil, ErrClientNotInitialized
	}

	client, exists := mc.Clients[serverURL]
	if !exists {
		return nil, ErrServerNotFound
	}

	toolName := request.Params["name"].(string)
	toolArgs := request.Params["arguments"]

	result, err := client.CallTool(ctx, toolName, toolArgs)
	if err != nil {
		return nil, err
	}

	response := CallToolResult{
		Content: make([]ContentBlock, len(result.Content)),
	}

	for i, content := range result.Content {
		contentBytes, err := json.Marshal(content)
		if err != nil {
			mc.Logger.Error("Failed to marshal content", err)
			continue
		}

		var contentMap map[string]any
		if err = json.Unmarshal(contentBytes, &contentMap); err != nil {
			mc.Logger.Error("Failed to unmarshal content", err)
			continue
		}

		response.Content[i] = contentMap
	}

	return &response, nil
}

// GetServerCapabilities implements MCPClientInterface.
func (mc *MCPClient) GetServerCapabilities() map[string]ServerCapabilities {
	return mc.ServerCapabilities
}

func (mc *MCPClient) GetServers() []string {
	if !mc.Initialized {
		return nil
	}

	servers := make([]string, 0, len(mc.Clients))
	for serverURL := range mc.Clients {
		servers = append(servers, serverURL)
	}
	return servers
}

func (mc *MCPClient) GetServerTools(serverURL string) ([]Tool, error) {
	if !mc.Initialized {
		return nil, ErrClientNotInitialized
	}

	tools := mc.ServerTools[serverURL]
	if tools == nil {
		return nil, fmt.Errorf("no tools found for server %s", serverURL)
	}

	return tools, nil
}

// ConvertMCPToolsToChatCompletionTools converts MCP server tools to chat completion tools
func (mc *MCPClient) ConvertMCPToolsToChatCompletionTools(serverTools []Tool) []types.ChatCompletionTool {
	tools := make([]types.ChatCompletionTool, 0)
	for _, tool := range serverTools {
		description := tool.Description

		inputSchema := tool.InputSchema

		if inputSchema == nil {
			inputSchema = make(map[string]any)
		}

		tools = append(tools, types.ChatCompletionTool{
			Type: "function",
			Function: types.FunctionObject{
				Name:        "mcp_" + tool.Name,
				Description: description,
				Parameters:  (*types.FunctionParameters)(&inputSchema),
			},
		})
	}

	return tools
}

// GetServerForTool returns the server URL that provides the specified tool
func (mc *MCPClient) GetServerForTool(toolName string) (string, error) {
	if !mc.Initialized {
		return "", fmt.Errorf("mcp client not initialized")
	}

	for serverURL, serverTools := range mc.ServerTools {
		for _, tool := range serverTools {
			if tool.Name == toolName {
				return serverURL, nil
			}
		}
	}

	return "", fmt.Errorf("tool %s not found on any server", toolName)
}

// GetAllChatCompletionTools returns all pre-converted chat completion tools from all servers
func (mc *MCPClient) GetAllChatCompletionTools() []types.ChatCompletionTool {
	if !mc.Initialized {
		return []types.ChatCompletionTool{}
	}
	return mc.ChatCompletionTools
}

// IsInitialized implements MCPClientInterface.
func (mc *MCPClient) IsInitialized() bool {
	return mc.Initialized
}
