package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ExecuteTool implements MCPClientInterface.
func (mc *MCPClient) ExecuteTool(ctx context.Context, request Request, serverURL string) (*CallToolResult, error) {
	mc.mu.RLock()
	initialized := mc.initialized
	client, exists := mc.clients[serverURL]
	mc.mu.RUnlock()

	if !initialized {
		return nil, ErrClientNotInitialized
	}

	if !exists {
		return nil, ErrServerNotFound
	}

	toolName, ok := request.Params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("tool request is missing a string 'name' parameter")
	}
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

func (mc *MCPClient) GetServers() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return nil
	}

	servers := make([]string, 0, len(mc.clients))
	for serverURL := range mc.clients {
		servers = append(servers, serverURL)
	}
	return servers
}

func (mc *MCPClient) GetServerTools(serverURL string) ([]Tool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return nil, ErrClientNotInitialized
	}

	tools := mc.serverTools[serverURL]
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
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return "", fmt.Errorf("mcp client not initialized")
	}

	for serverURL, serverTools := range mc.serverTools {
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
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return []types.ChatCompletionTool{}
	}
	return mc.chatCompletionTools
}

// IsInitialized implements MCPClientInterface.
func (mc *MCPClient) IsInitialized() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.initialized
}
