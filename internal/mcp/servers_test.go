package mcp

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

func TestParseServers(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []ServerSpec
		errorMsg string
	}{
		{
			name:     "empty value yields no servers",
			raw:      "",
			expected: []ServerSpec{},
		},
		{
			name:     "explicit alias",
			raw:      "deepwiki=https://mcp.deepwiki.com/mcp",
			expected: []ServerSpec{{Alias: "deepwiki", URL: "https://mcp.deepwiki.com/mcp"}},
		},
		{
			name:     "alias derived from the url host",
			raw:      "http://mcp-time-server:8081/mcp",
			expected: []ServerSpec{{Alias: "mcp-time-server", URL: "http://mcp-time-server:8081/mcp"}},
		},
		{
			name:     "derived alias sanitises dots",
			raw:      "https://mcp.deepwiki.com/mcp",
			expected: []ServerSpec{{Alias: "mcp_deepwiki_com", URL: "https://mcp.deepwiki.com/mcp"}},
		},
		{
			name: "mixed explicit and derived entries with whitespace",
			raw:  " deepwiki=https://mcp.deepwiki.com/mcp , http://mcp-time-server:8081/mcp ",
			expected: []ServerSpec{
				{Alias: "deepwiki", URL: "https://mcp.deepwiki.com/mcp"},
				{Alias: "mcp-time-server", URL: "http://mcp-time-server:8081/mcp"},
			},
		},
		{
			name:     "query string with equals is not mistaken for an alias",
			raw:      "http://mcp-time-server:8081/mcp?token=abc",
			expected: []ServerSpec{{Alias: "mcp-time-server", URL: "http://mcp-time-server:8081/mcp?token=abc"}},
		},
		{
			name:     "invalid alias is rejected",
			raw:      "Deep Wiki=https://mcp.deepwiki.com/mcp",
			errorMsg: "invalid mcp server alias",
		},
		{
			name:     "reserved alias is rejected",
			raw:      "tools=https://mcp.deepwiki.com/mcp",
			errorMsg: "reserved",
		},
		{
			name:     "duplicate alias is rejected",
			raw:      "a=https://one.example.com/mcp,a=https://two.example.com/mcp",
			errorMsg: "duplicate mcp server alias",
		},
		{
			name:     "duplicate derived alias is rejected",
			raw:      "https://one.example.com/mcp,https://one.example.com/other",
			errorMsg: "duplicate mcp server alias",
		},
		{
			name:     "non http url is rejected",
			raw:      "stdio:///usr/bin/server",
			errorMsg: "invalid mcp server url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, err := ParseServers(tt.raw)

			if tt.errorMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, specs)
		})
	}
}

// newResolveClient builds an initialized client where two servers expose a
// tool with the same name.
func newResolveClient() *MCPClient {
	return &MCPClient{
		Logger:      logger.NewNoopLogger(),
		Config:      config.Config{MCP: &config.MCPConfig{}},
		initialized: true,
		serverTools: map[string][]Tool{
			"deepwiki":  {{Name: "read_file"}, {Name: "search"}},
			"time_serv": {{Name: "read_file"}, {Name: "now"}},
		},
	}
}

func TestResolveTool(t *testing.T) {
	mc := newResolveClient()

	tests := []struct {
		name          string
		namespaced    string
		expectedAlias string
		expectedTool  string
		errorMsg      string
	}{
		{
			name:          "routes a shared tool name to the first server",
			namespaced:    "mcp_deepwiki_read_file",
			expectedAlias: "deepwiki",
			expectedTool:  "read_file",
		},
		{
			name:          "routes the same tool name to the second server",
			namespaced:    "mcp_time_serv_read_file",
			expectedAlias: "time_serv",
			expectedTool:  "read_file",
		},
		{
			name:          "resolves an alias containing underscores",
			namespaced:    "mcp_time_serv_now",
			expectedAlias: "time_serv",
			expectedTool:  "now",
		},
		{
			name:          "unknown tool on a known alias still routes to that server",
			namespaced:    "mcp_deepwiki_unknown",
			expectedAlias: "deepwiki",
			expectedTool:  "unknown",
		},
		{
			name:       "unknown alias is an error",
			namespaced: "mcp_nosuch_read_file",
			errorMsg:   "does not match any mcp server alias",
		},
		{
			name:       "missing mcp prefix is an error",
			namespaced: "deepwiki_read_file",
			errorMsg:   "is not an mcp tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alias, toolName, err := mc.ResolveTool(tt.namespaced)

			if tt.errorMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedAlias, alias)
			assert.Equal(t, tt.expectedTool, toolName)
		})
	}
}

func TestResolveToolPrefersTheServerActuallyServingTheTool(t *testing.T) {
	// "time" and "time_serv" both prefix-match mcp_time_serv_now, but only
	// time_serv actually has the tool.
	mc := &MCPClient{
		Logger:      logger.NewNoopLogger(),
		Config:      config.Config{MCP: &config.MCPConfig{}},
		initialized: true,
		serverTools: map[string][]Tool{
			"time":      {{Name: "serv_other"}},
			"time_serv": {{Name: "now"}},
		},
	}

	alias, toolName, err := mc.ResolveTool("mcp_time_serv_now")
	require.NoError(t, err)
	assert.Equal(t, "time_serv", alias)
	assert.Equal(t, "now", toolName)
}

func TestConvertMCPToolsToChatCompletionToolsNamespacesNames(t *testing.T) {
	mc := &MCPClient{Logger: logger.NewNoopLogger(), Config: config.Config{MCP: &config.MCPConfig{}}}

	tools := mc.ConvertMCPToolsToChatCompletionTools("deepwiki", []Tool{{Name: "read_file"}})

	require.Len(t, tools, 1)
	assert.Equal(t, "mcp_deepwiki_read_file", tools[0].Function.Name)
}
