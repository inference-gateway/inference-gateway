package mcp

// ToolWithServer embeds the official MCP Tool type and adds a ServerURL field
// to track which server provides this tool
type ToolWithServer struct {
	Tool
	ServerURL string
}
