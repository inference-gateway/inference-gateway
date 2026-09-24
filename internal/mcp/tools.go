package mcp

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ExecuteTool implements MCPClientInterface. The result keeps everything the
// server returned: isError, structuredContent and every content type.
func (mc *MCPClient) ExecuteTool(ctx context.Context, request Request, serverAlias string) (*CallToolResult, error) {
	mc.mu.RLock()
	initialized := mc.initialized
	_, exists := mc.serverTools[serverAlias]
	mc.mu.RUnlock()

	if !initialized {
		return nil, ErrClientNotInitialized
	}

	server, ok := mc.serverSpec(serverAlias)
	if !exists || !ok {
		return nil, ErrServerNotFound
	}

	toolName, ok := request.Params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("tool request is missing a string 'name' parameter")
	}
	arguments, _ := request.Params["arguments"].(map[string]any)

	return mc.callTool(ctx, server.URL, toolName, arguments)
}

func (mc *MCPClient) GetServers() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return nil
	}

	return slices.Sorted(maps.Keys(mc.serverTools))
}

func (mc *MCPClient) GetServerTools(serverAlias string) ([]Tool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return nil, ErrClientNotInitialized
	}

	tools := mc.serverTools[serverAlias]
	if tools == nil {
		return nil, fmt.Errorf("no tools found for server %s", serverAlias)
	}

	return tools, nil
}

// ConvertMCPToolsToChatCompletionTools converts MCP server tools to chat
// completion tools, namespacing each one as mcp_<alias>_<tool>.
func (mc *MCPClient) ConvertMCPToolsToChatCompletionTools(serverAlias string, serverTools []Tool) []types.ChatCompletionTool {
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
				Name:        NamespacedToolName(serverAlias, tool.Name),
				Description: description,
				Parameters:  (*types.FunctionParameters)(&inputSchema),
			},
		})
	}

	return tools
}

// ResolveTool splits a namespaced mcp_<alias>_<tool> name into the server alias
// and the bare tool name. Aliases may themselves contain underscores, so when
// several aliases prefix-match the one that actually serves the tool wins, and
// the longest match breaks any remaining tie.
func (mc *MCPClient) ResolveTool(namespacedName string) (string, string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return "", "", ErrClientNotInitialized
	}

	rest, ok := strings.CutPrefix(namespacedName, ToolNamePrefix)
	if !ok {
		return "", "", fmt.Errorf("tool %s is not an mcp tool (expected the %s prefix)", namespacedName, ToolNamePrefix)
	}

	bestAlias, bestTool := "", ""
	for alias, serverTools := range mc.serverTools {
		toolName, matches := strings.CutPrefix(rest, alias+"_")
		if !matches {
			continue
		}
		if slices.ContainsFunc(serverTools, func(t Tool) bool { return t.Name == toolName }) {
			return alias, toolName, nil
		}
		if len(alias) > len(bestAlias) {
			bestAlias, bestTool = alias, toolName
		}
	}

	if bestAlias == "" {
		return "", "", fmt.Errorf("tool %s does not match any mcp server alias", namespacedName)
	}
	return bestAlias, bestTool, nil
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
