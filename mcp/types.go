package mcp

// MCPToolDefinition represents a tool that can be used by the LLM
type MCPToolDefinition struct {
	Name        string
	Description string
	Parameters  MCPToolParameters
	ServerURL   string // The URL of the MCP server that provides this tool
}

// MCPToolParameters defines the parameters for an MCP tool
type MCPToolParameters struct {
	Type       string
	Properties map[string]interface{}
}

// MCPToolCall represents a tool call requested by the LLM
type MCPToolCall struct {
	ID       string
	Type     string
	Function MCPToolCallFunction
}

// MCPToolCallFunction contains the details of the function to call
type MCPToolCallFunction struct {
	Name      string
	Arguments string
}
