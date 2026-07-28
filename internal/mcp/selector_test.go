package mcp

import (
	"testing"

	assert "github.com/stretchr/testify/assert"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
)

func strPtr(s string) *string { return &s }

func newCatalogClient(include, exclude string) *MCPClient {
	return &MCPClient{
		Logger:      logger.NewNoopLogger(),
		Config:      config.Config{MCP: &config.MCPConfig{IncludeTools: include, ExcludeTools: exclude}},
		initialized: true,
		serverTools: map[string][]Tool{
			"http://server-a": {
				{Name: "read_file", Description: strPtr("Read a file from disk"), InputSchema: map[string]any{"type": "object"}},
				{Name: "list_directory", Description: strPtr("List directory contents"), InputSchema: map[string]any{"type": "object"}},
			},
			"http://server-b": {
				{Name: "search_web", Description: strPtr("Search the web"), InputSchema: map[string]any{"type": "object", "required": []any{"q"}}},
			},
		},
	}
}

func catalogNames(entries []ToolCatalogEntry) map[string]ToolCatalogEntry {
	byName := make(map[string]ToolCatalogEntry, len(entries))
	for _, e := range entries {
		byName[e.Name] = e
	}
	return byName
}

func TestGetToolsCatalog_Compact(t *testing.T) {
	mc := newCatalogClient("", "")

	entries := mc.GetToolsCatalog("", nil)
	assert.Len(t, entries, 3)
	for _, e := range entries {
		assert.Nil(t, e.InputSchema, "compact catalog must not include schemas")
		assert.NotEmpty(t, e.Server)
		assert.NotEmpty(t, e.Description)
	}
}

func TestGetToolsCatalog_QueryFilter(t *testing.T) {
	mc := newCatalogClient("", "")

	byName := catalogNames(mc.GetToolsCatalog("directory", nil))
	assert.Len(t, byName, 1)
	assert.Contains(t, byName, "list_directory")

	byName = catalogNames(mc.GetToolsCatalog("web", nil))
	assert.Len(t, byName, 1)
	assert.Contains(t, byName, "search_web")
}

func TestGetToolsCatalog_NamesReturnsSchemas(t *testing.T) {
	mc := newCatalogClient("", "")

	entries := mc.GetToolsCatalog("", []string{"search_web", "mcp_read_file"})
	byName := catalogNames(entries)
	assert.Len(t, byName, 2)
	assert.NotNil(t, byName["search_web"].InputSchema)
	assert.NotNil(t, byName["read_file"].InputSchema, "mcp_ prefix in request should still match")
}

func TestGetToolsCatalog_HonorsExcludeFilter(t *testing.T) {
	mc := newCatalogClient("", "read_file")

	byName := catalogNames(mc.GetToolsCatalog("", nil))
	assert.NotContains(t, byName, "read_file")
	assert.Contains(t, byName, "list_directory")
	assert.Contains(t, byName, "search_web")
}

func TestGetSelectorTools(t *testing.T) {
	mc := newCatalogClient("", "")

	tools := mc.GetSelectorTools()
	assert.Len(t, tools, 2)
	assert.Equal(t, SelectorToolGet, tools[0].Function.Name)
	assert.Equal(t, SelectorToolExecute, tools[1].Function.Name)
}

func TestGetSelectorTools_EmptyWhenAllFiltered(t *testing.T) {
	mc := newCatalogClient("nonexistent_tool", "")

	assert.Empty(t, mc.GetSelectorTools())
}
