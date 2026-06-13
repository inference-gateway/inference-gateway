package mcp

import "strings"

// normalizeToolName lowercases the name and strips the "mcp_" prefix so that
// include/exclude matching is forgiving about the prefix and letter case.
// Lowercasing happens before the prefix is stripped so an uppercase "MCP_"
// prefix is also removed.
func normalizeToolName(name string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "mcp_")
}

// parseToolList parses a comma-separated list of tool names into a set of
// normalized names.
func parseToolList(list string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, item := range strings.Split(list, ",") {
		normalized := normalizeToolName(item)
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	return set
}

// isToolAllowed reports whether a tool with the given name should be injected,
// based on the configured include/exclude lists. The include list takes
// precedence over the exclude list, mirroring ALLOWED_MODELS/DISALLOWED_MODELS:
//   - when the include list is non-empty, only tools in it are allowed;
//   - otherwise every tool except those in the exclude list is allowed;
//   - when both lists are empty, every tool is allowed.
func isToolAllowed(toolName, includeList, excludeList string) bool {
	name := normalizeToolName(toolName)

	if include := parseToolList(includeList); len(include) > 0 {
		_, ok := include[name]
		return ok
	}

	if exclude := parseToolList(excludeList); len(exclude) > 0 {
		_, ok := exclude[name]
		return !ok
	}

	return true
}

// filterTools returns the subset of the given tools that should be injected
// into chat completion requests, honoring the configured MCP include/exclude
// lists. Tools that are filtered out are logged at debug level.
func (mc *MCPClient) filterTools(tools []Tool) []Tool {
	includeList := mc.Config.MCP.IncludeTools
	excludeList := mc.Config.MCP.ExcludeTools

	if includeList == "" && excludeList == "" {
		return tools
	}

	filtered := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if isToolAllowed(tool.Name, includeList, excludeList) {
			filtered = append(filtered, tool)
			continue
		}
		mc.Logger.Debug("mcp tool excluded from injection by include/exclude config", "tool", tool.Name)
	}
	return filtered
}
