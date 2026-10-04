package mcp

import (
	"fmt"

	endpoints "github.com/inference-gateway/inference-gateway/internal/platform/endpoints"
)

// ReservedAlias is the alias that would shadow the selector meta-tools
// mcp_tools_get / mcp_tools_execute, so servers may not claim it.
const ReservedAlias = "tools"

const serverKind = "mcp server"

// ServerSpec pairs a configured MCP server URL with the alias its tools are
// namespaced under.
type ServerSpec = endpoints.Spec

// ParseServers parses the MCP_SERVERS value: a comma-separated list of
// "alias=url" or bare "url" entries. A missing alias is derived from the URL
// host. Aliases must match ^[a-z0-9_-]+$, must be unique, and may not be the
// reserved alias "tools".
func ParseServers(raw string) ([]ServerSpec, error) {
	specs, err := endpoints.Parse(raw, serverKind)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		if spec.Alias == ReservedAlias {
			return nil, fmt.Errorf("mcp server alias %q is reserved (it would shadow %s/%s)", spec.Alias, SelectorToolGet, SelectorToolExecute)
		}
	}
	return specs, nil
}

// NamespacedToolName renders an MCP tool as mcp_<alias>_<tool>, the name models
// see and call.
func NamespacedToolName(alias, toolName string) string {
	return ToolNamePrefix + alias + "_" + toolName
}
