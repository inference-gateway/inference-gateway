package mcp

import (
	"strings"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Tool mode values for MCP_TOOL_MODE.
const (
	// ToolModeSelector injects only the two meta-tools and lets the model
	// discover and dispatch tools gateway-side (default).
	ToolModeSelector = "selector"
	// ToolModeDirect injects every MCP tool schema into every request.
	ToolModeDirect = "direct"
)

// Selector meta-tool names. These are gateway-defined and handled inside the
// agent loop, never dispatched to an upstream MCP server.
const (
	SelectorToolGet     = "mcp_tools_get"
	SelectorToolExecute = "mcp_tools_execute"
)

// ToolCatalogEntry describes a single MCP tool in the selector catalog. The
// input schema is only populated when full definitions are requested.
type ToolCatalogEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Server      string         `json:"server"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// SelectorToolDefinitions returns the fixed pair of meta-tools exposed in
// selector mode. Exported so tests and benchmarks reference the real shape.
func SelectorToolDefinitions() []types.ChatCompletionTool {
	getDesc := "List available MCP tools or get their full input schemas. " +
		"Call without arguments for a compact catalog (name + one-line description). " +
		"Call with names to get full schemas before executing."
	execDesc := "Execute an MCP tool by name. Look up the tool and its schema with mcp_tools_get first."

	return []types.ChatCompletionTool{
		{
			Type: types.Function,
			Function: types.FunctionObject{
				Name:        SelectorToolGet,
				Description: &getDesc,
				Parameters: &types.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Optional substring/keyword filter on tool names and descriptions",
						},
						"names": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Return full input schemas for these tools",
						},
					},
				},
			},
		},
		{
			Type: types.Function,
			Function: types.FunctionObject{
				Name:        SelectorToolExecute,
				Description: &execDesc,
				Parameters: &types.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "Tool name from mcp_tools_get"},
						"arguments": map[string]any{"type": "object", "description": "Arguments for the tool"},
					},
					"required": []string{"name"},
				},
			},
		},
	}
}

// GetSelectorTools returns the two meta-tools, or an empty slice when no
// underlying tools are available after include/exclude filtering.
func (mc *MCPClient) GetSelectorTools() []types.ChatCompletionTool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if !mc.initialized {
		return []types.ChatCompletionTool{}
	}

	// Pure existence check: we only care whether any server exposes at least one
	// tool after filtering, so the non-deterministic map iteration order is
	// irrelevant - the result is the same regardless of which server matches first.
	for _, serverTools := range mc.serverTools {
		if len(mc.filterTools(serverTools)) > 0 {
			return SelectorToolDefinitions()
		}
	}
	return []types.ChatCompletionTool{}
}

// GetToolsCatalog answers an mcp_tools_get call from the cached serverTools
// map. With names set it returns full definitions (including input schemas) for
// the matching tools; otherwise it returns the compact catalog, optionally
// filtered by a substring query. Include/exclude config is always honored.
func (mc *MCPClient) GetToolsCatalog(query string, names []string) []ToolCatalogEntry {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	wanted := make(map[string]struct{}, len(names))
	for _, n := range names {
		wanted[normalizeToolName(n)] = struct{}{}
	}
	q := strings.ToLower(strings.TrimSpace(query))

	entries := make([]ToolCatalogEntry, 0)
	for serverURL, serverTools := range mc.serverTools {
		for _, tool := range mc.filterTools(serverTools) {
			desc := ""
			if tool.Description != nil {
				desc = *tool.Description
			}

			if len(wanted) > 0 {
				if _, ok := wanted[normalizeToolName(tool.Name)]; !ok {
					continue
				}
				entries = append(entries, ToolCatalogEntry{
					Name:        tool.Name,
					Description: desc,
					Server:      serverURL,
					InputSchema: tool.InputSchema,
				})
				continue
			}

			if q != "" && !strings.Contains(strings.ToLower(tool.Name), q) && !strings.Contains(strings.ToLower(desc), q) {
				continue
			}
			entries = append(entries, ToolCatalogEntry{
				Name:        tool.Name,
				Description: desc,
				Server:      serverURL,
			})
		}
	}
	return entries
}
