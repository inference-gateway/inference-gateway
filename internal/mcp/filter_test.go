package mcp

import (
	"testing"

	assert "github.com/stretchr/testify/assert"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
)

func TestNormalizeToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "plain name", input: "read_file", expected: "read_file"},
		{name: "strips mcp prefix", input: "mcp_read_file", expected: "read_file"},
		{name: "lowercases", input: "Read_File", expected: "read_file"},
		{name: "trims whitespace", input: "  read_file  ", expected: "read_file"},
		{name: "prefix and case and spaces", input: " MCP_Read_File ", expected: "read_file"},
		{name: "empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeToolName(tt.input))
		})
	}
}

func TestIsToolAllowed(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		includeList string
		excludeList string
		expected    bool
	}{
		{
			name:     "no lists allows everything",
			toolName: "read_file",
			expected: true,
		},
		{
			name:        "include list allows listed tool",
			toolName:    "read_file",
			includeList: "read_file,list_directory",
			expected:    true,
		},
		{
			name:        "include list blocks unlisted tool",
			toolName:    "delete_file",
			includeList: "read_file,list_directory",
			expected:    false,
		},
		{
			name:        "exclude list blocks listed tool",
			toolName:    "delete_file",
			excludeList: "delete_file",
			expected:    false,
		},
		{
			name:        "exclude list allows unlisted tool",
			toolName:    "read_file",
			excludeList: "delete_file",
			expected:    true,
		},
		{
			name:        "include takes precedence over exclude",
			toolName:    "read_file",
			includeList: "read_file",
			excludeList: "read_file",
			expected:    true,
		},
		{
			name:        "include precedence blocks tool only in exclude",
			toolName:    "delete_file",
			includeList: "read_file",
			excludeList: "delete_file",
			expected:    false,
		},
		{
			name:        "matching tolerates mcp prefix in tool name",
			toolName:    "mcp_read_file",
			includeList: "read_file",
			expected:    true,
		},
		{
			name:        "matching tolerates mcp prefix in config",
			toolName:    "read_file",
			includeList: "mcp_read_file",
			expected:    true,
		},
		{
			name:        "matching is case insensitive",
			toolName:    "Read_File",
			excludeList: "read_file",
			expected:    false,
		},
		{
			name:        "whitespace and empty entries are ignored",
			toolName:    "read_file",
			includeList: " read_file , , list_directory ",
			expected:    true,
		},
		{
			name:        "only whitespace include list allows everything",
			toolName:    "read_file",
			includeList: " , ",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isToolAllowed(tt.toolName, tt.includeList, tt.excludeList))
		})
	}
}

func TestMCPClientFilterTools(t *testing.T) {
	allTools := []Tool{
		{Name: "read_file"},
		{Name: "list_directory"},
		{Name: "delete_file"},
	}

	toolNames := func(tools []Tool) []string {
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			names = append(names, tool.Name)
		}
		return names
	}

	tests := []struct {
		name        string
		includeList string
		excludeList string
		expected    []string
	}{
		{
			name:     "no config returns all tools",
			expected: []string{"read_file", "list_directory", "delete_file"},
		},
		{
			name:        "include list keeps only listed tools",
			includeList: "read_file,list_directory",
			expected:    []string{"read_file", "list_directory"},
		},
		{
			name:        "exclude list drops listed tools",
			excludeList: "delete_file",
			expected:    []string{"read_file", "list_directory"},
		},
		{
			name:        "include precedence over exclude",
			includeList: "read_file",
			excludeList: "read_file",
			expected:    []string{"read_file"},
		},
		{
			name:        "include list with no matches returns empty",
			includeList: "nonexistent_tool",
			expected:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MCPClient{
				Logger: logger.NewNoopLogger(),
				Config: config.Config{
					MCP: &config.MCPConfig{
						IncludeTools: tt.includeList,
						ExcludeTools: tt.excludeList,
					},
				},
			}

			filtered := mc.filterTools(allTools)
			assert.Equal(t, tt.expected, toolNames(filtered))
		})
	}
}
