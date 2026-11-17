package mcp

import "strings"

// normalizeToolName lowercases the name and strips the mcp_ prefix so that
// include/exclude matching is forgiving about the prefix and letter case.
// Lowercasing happens before the prefix is stripped so an uppercase MCP_
// prefix is also removed.
func normalizeToolName(name string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), ToolNamePrefix)
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

// IsToolAllowed reports whether a tool of the given alias is injected into chat
// completions and listed on POST /mcp. An entry matches the bare tool name, so it
// applies to every server, or the namespaced one (with or without mcp_). Like
// ALLOWED_MODELS/DISALLOWED_MODELS, a non-empty include list allows only its tools,
// otherwise every tool outside the exclude list is allowed.
func IsToolAllowed(serverAlias, toolName, includeList, excludeList string) bool {
	bare := normalizeToolName(toolName)
	namespaced := normalizeToolName(serverAlias + "_" + toolName)

	listed := func(set map[string]struct{}) bool {
		_, bareOK := set[bare]
		_, namespacedOK := set[namespaced]
		return bareOK || namespacedOK
	}

	if include := parseToolList(includeList); len(include) > 0 {
		return listed(include)
	}

	if exclude := parseToolList(excludeList); len(exclude) > 0 {
		return !listed(exclude)
	}

	return true
}

// filterTools returns the subset of the given tools that should be injected
// into chat completion requests, honoring the configured MCP include/exclude
// lists. Tools that are filtered out are logged at debug level.
func (mc *MCPClient) filterTools(serverAlias string, tools []Tool) []Tool {
	includeList := mc.Config.MCP.IncludeTools
	excludeList := mc.Config.MCP.ExcludeTools

	if includeList == "" && excludeList == "" {
		return tools
	}

	filtered := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if IsToolAllowed(serverAlias, tool.Name, includeList, excludeList) {
			filtered = append(filtered, tool)
			continue
		}
		mc.Logger.Debug("mcp tool excluded from injection by include/exclude config", "tool", tool.Name, "server", serverAlias)
	}
	return filtered
}
