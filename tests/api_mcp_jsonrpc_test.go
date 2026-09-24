package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"

	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	jsonRPCServerAlias = "stub"
	jsonRPCToolName    = "echo"
	jsonRPCToolOutput  = "ok"
	jsonRPCInboundAuth = "Bearer caller-token"

	// mcpRequestMeta is the params._meta every 2026-07-28 request carries.
	mcpRequestMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"` + mcp.ProtocolVersion + `","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}`
)

// newJSONRPCStubServer is a minimal stateless 2026-07-28 upstream MCP server.
// It checks every request carries the request metadata the gateway must send
// and none of the caller's credentials.
func newJSONRPCStubServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Meta mcp.RequestMetaObject `json:"_meta"`
				Name string                `json:"name"`
			} `json:"params"`
		}
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			return
		}

		assert.Empty(t, r.Header.Get("Authorization"), "the caller's credentials must not reach upstream servers")
		assert.Equal(t, mcp.ProtocolVersion, r.Header.Get(mcp.HeaderProtocolVersion))
		assert.Equal(t, mcp.ProtocolVersion, req.Params.Meta.IoModelcontextprotocolProtocolVersion)
		assert.Equal(t, req.Method, r.Header.Get(mcp.HeaderMethod))

		var result any
		switch req.Method {
		case string(mcp.ToolsList):
			result = map[string]any{
				"resultType": mcp.ResultTypeComplete,
				"tools": []map[string]any{
					{"name": jsonRPCToolName, "description": "echoes back", "inputSchema": map[string]any{"type": "object"}},
				},
			}
		case string(mcp.ToolsCall):
			// The gateway must strip the mcp_<alias>_ namespace before it gets here.
			assert.Equal(t, jsonRPCToolName, req.Params.Name)
			assert.Equal(t, jsonRPCToolName, r.Header.Get(mcp.HeaderName))
			result = map[string]any{
				"resultType": mcp.ResultTypeComplete,
				"content":    []map[string]any{{"type": "text", "text": jsonRPCToolOutput}},
			}
		default:
			t.Errorf("unexpected upstream method %q", req.Method)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}))
	}))

	t.Cleanup(srv.Close)
	return srv
}

// TestMCPJSONRPCEndpointEndToEnd drives POST /mcp against a real MCP client
// talking to a real (stub) MCP server, so the namespacing and the dispatch back
// to the upstream server are exercised end to end rather than mocked.
func TestMCPJSONRPCEndpointEndToEnd(t *testing.T) {
	srv := newJSONRPCStubServer(t)

	cfg := config.Config{MCP: &config.MCPConfig{
		Enabled:               true,
		Expose:                true,
		DialTimeout:           2 * time.Second,
		TlsHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
		ExpectContinueTimeout: time.Second,
		ClientTimeout:         5 * time.Second,
		RequestTimeout:        5 * time.Second,
		RetryInterval:         10 * time.Millisecond,
		InitialBackoff:        10 * time.Millisecond,
	}}

	mcpClient := mcp.NewMCPClient([]mcp.ServerSpec{{Alias: jsonRPCServerAlias, URL: srv.URL}}, logger.NewNoopLogger(), cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, mcpClient.InitializeAll(ctx))

	router := api.NewRouter(cfg, logger.NewNoopLogger(), nil, nil, mcpClient, mcp.NewAgent(logger.NewNoopLogger(), mcpClient), nil, nil, nil)
	engine := gin.New()
	engine.POST(middlewares.MCPPath, router.MCPJSONRPCHandler)

	post := func(method types.MCPJSONRPCRequestMethod, toolName, body string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodPost, middlewares.MCPPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", jsonRPCInboundAuth)
		req.Header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion)
		req.Header.Set(mcp.HeaderMethod, string(method))
		if toolName != "" {
			req.Header.Set(mcp.HeaderName, toolName)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Body.Len() == 0 {
			return w.Code, nil
		}
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return w.Code, resp
	}

	code, resp := post(types.ServerDiscover, "", `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+mcpRequestMeta+`}}`)
	require.Equal(t, http.StatusOK, code)
	result, ok := resp["result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{mcp.ProtocolVersion}, result["supportedVersions"])

	namespaced := mcp.NamespacedToolName(jsonRPCServerAlias, jsonRPCToolName)
	code, resp = post(types.ToolsList, "", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{`+mcpRequestMeta+`}}`)
	require.Equal(t, http.StatusOK, code)
	result, ok = resp["result"].(map[string]any)
	require.True(t, ok)
	tools, ok := result["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	assert.Equal(t, namespaced, tools[0].(map[string]any)["name"])

	code, resp = post(types.ToolsCall, namespaced, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"`+namespaced+`","arguments":{"text":"hi"},`+mcpRequestMeta+`}}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, resp["error"])
	result, ok = resp["result"].(map[string]any)
	require.True(t, ok)
	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	assert.Equal(t, jsonRPCToolOutput, content[0].(map[string]any)["text"])
}
