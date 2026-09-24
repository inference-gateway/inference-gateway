package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mcpmocks "github.com/inference-gateway/inference-gateway/tests/mocks/mcp"

	gin "github.com/gin-gonic/gin"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	timeAlias          = "time"
	weatherAlias       = "weather"
	getTimeTool        = "get_time"
	forecastTool       = "forecast"
	nsGetTimeTool      = mcp.ToolNamePrefix + timeAlias + "_" + getTimeTool
	nsForecastTool     = mcp.ToolNamePrefix + weatherAlias + "_" + forecastTool
	upstreamFailureMsg = "upstream exploded"
)

// jsonRPCTestResponse is the decoded envelope the assertions work against; the
// handler writes the generated types.MCPJSONRPCResponse.
type jsonRPCTestResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  map[string]any  `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// newMCPEngine wires POST /mcp exactly as cmd/gateway/main.go does.
func newMCPEngine(t *testing.T, cfg config.Config, mcpClient mcp.MCPClientInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := NewRouter(cfg, logger.NewNoopLogger(), nil, nil, mcpClient, nil, nil, nil)
	r := gin.New()
	r.POST(middlewares.MCPPath, router.MCPJSONRPCHandler)
	return r
}

func postMCP(t *testing.T, engine *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, middlewares.MCPPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func mcpEnabledConfig() config.Config {
	return config.Config{MCP: &config.MCPConfig{Enabled: true, Expose: true}}
}

// TestMCPJSONRPCHandler_Gating pins the feature flag behaviour: the endpoint
// answers 403 unless both MCP_ENABLED and MCP_EXPOSE are set.
func TestMCPJSONRPCHandler_Gating(t *testing.T) {
	tests := []struct {
		name    string
		mcp     config.MCPConfig
		allowed bool
	}{
		{name: "exposed", mcp: config.MCPConfig{Enabled: true, Expose: true}, allowed: true},
		{name: "not exposed", mcp: config.MCPConfig{Enabled: true, Expose: false}},
		{name: "mcp disabled", mcp: config.MCPConfig{Enabled: false, Expose: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newMCPEngine(t, config.Config{MCP: &tt.mcp}, nil)
			w := postMCP(t, engine, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

			if tt.allowed {
				assert.Equal(t, http.StatusOK, w.Code)
				return
			}
			assert.Equal(t, http.StatusForbidden, w.Code)
			var body ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, errMsgMCPNotExposed, body.Error)
		})
	}
}

// TestMCPJSONRPCHandler_Initialize covers the handshake: a known protocol
// version is echoed back, an unknown one falls back to the gateway's default.
func TestMCPJSONRPCHandler_Initialize(t *testing.T) {
	tests := []struct {
		name     string
		params   string
		expected string
	}{
		{name: "echoes a supported version", params: `,"params":{"protocolVersion":"2025-03-26"}`, expected: "2025-03-26"},
		{name: "falls back on an unknown version", params: `,"params":{"protocolVersion":"1999-01-01"}`, expected: defaultProtocolVersion},
		{name: "falls back without params", expected: defaultProtocolVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newMCPEngine(t, mcpEnabledConfig(), nil)
			w := postMCP(t, engine, `{"jsonrpc":"2.0","id":1,"method":"initialize"`+tt.params+`}`)

			require.Equal(t, http.StatusOK, w.Code)
			var resp jsonRPCTestResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Nil(t, resp.Error)
			assert.Equal(t, "2.0", resp.Jsonrpc)
			assert.JSONEq(t, `1`, string(resp.ID))
			assert.Equal(t, tt.expected, resp.Result["protocolVersion"])
			assert.Equal(t, map[string]any{"name": mcpServerName, "version": Version}, resp.Result["serverInfo"])
			assert.Equal(t, map[string]any{"tools": map[string]any{"listChanged": false}}, resp.Result["capabilities"])
		})
	}
}

// TestMCPJSONRPCHandler_Notification asserts a request without an id is
// answered with 202 and no body, as JSON-RPC requires for notifications.
func TestMCPJSONRPCHandler_Notification(t *testing.T) {
	engine := newMCPEngine(t, mcpEnabledConfig(), nil)
	w := postMCP(t, engine, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestMCPJSONRPCHandler_ToolsList asserts tools come back namespaced, that an
// unavailable server is skipped instead of failing the call, and that the
// include/exclude lists apply.
func TestMCPJSONRPCHandler_ToolsList(t *testing.T) {
	description := "Returns the current time"

	tests := []struct {
		name        string
		excludeList string
		statuses    map[string]mcp.ServerStatus
		setup       func(*mcpmocks.MockMCPClientInterface)
		expected    []string
	}{
		{
			name:     "all servers healthy",
			statuses: map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable, weatherAlias: mcp.ServerStatusAvailable},
			expected: []string{nsGetTimeTool, nsForecastTool},
		},
		{
			name:     "unavailable server is skipped",
			statuses: map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable, weatherAlias: mcp.ServerStatusUnavailable},
			expected: []string{nsGetTimeTool},
		},
		{
			name:        "excluded tool is hidden",
			excludeList: forecastTool,
			statuses:    map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable, weatherAlias: mcp.ServerStatusAvailable},
			expected:    []string{nsGetTimeTool},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mcpClient := mcpmocks.NewMockMCPClientInterface(ctrl)
			mcpClient.EXPECT().IsInitialized().Return(true)
			mcpClient.EXPECT().GetAllServerStatuses().Return(tt.statuses)
			mcpClient.EXPECT().GetServers().Return([]string{timeAlias, weatherAlias})
			mcpClient.EXPECT().GetServerTools(timeAlias).
				Return([]mcp.Tool{{Name: getTimeTool, Description: &description}}, nil).AnyTimes()
			mcpClient.EXPECT().GetServerTools(weatherAlias).
				Return([]mcp.Tool{{Name: forecastTool, InputSchema: map[string]any{"type": "object"}}}, nil).AnyTimes()

			cfg := mcpEnabledConfig()
			cfg.MCP.ExcludeTools = tt.excludeList
			engine := newMCPEngine(t, cfg, mcpClient)
			w := postMCP(t, engine, `{"jsonrpc":"2.0","id":"abc","method":"tools/list"}`)

			require.Equal(t, http.StatusOK, w.Code)
			var resp jsonRPCTestResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Nil(t, resp.Error)
			assert.JSONEq(t, `"abc"`, string(resp.ID))

			listed, ok := resp.Result["tools"].([]any)
			require.True(t, ok)
			names := make([]string, 0, len(listed))
			for _, tool := range listed {
				entry, isObject := tool.(map[string]any)
				require.True(t, isObject)
				names = append(names, entry["name"].(string))
				assert.NotNil(t, entry["inputSchema"], "clients require an object input schema")
			}
			assert.ElementsMatch(t, tt.expected, names)
		})
	}
}

// TestMCPJSONRPCHandler_ToolsListWithoutClient asserts the endpoint answers
// with an empty list, not an error, when no MCP servers are configured.
func TestMCPJSONRPCHandler_ToolsListWithoutClient(t *testing.T) {
	engine := newMCPEngine(t, mcpEnabledConfig(), nil)
	w := postMCP(t, engine, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var resp jsonRPCTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Nil(t, resp.Error)
	assert.Empty(t, resp.Result["tools"])
}

// TestMCPJSONRPCHandler_ToolsCall asserts the call is routed to the resolved
// server under the bare tool name and that upstream failures surface as
// JSON-RPC errors rather than 500s.
func TestMCPJSONRPCHandler_ToolsCall(t *testing.T) {
	engine := func(t *testing.T, setup func(*mcpmocks.MockMCPClientInterface), excludeList string) *gin.Engine {
		t.Helper()
		ctrl := gomock.NewController(t)
		mcpClient := mcpmocks.NewMockMCPClientInterface(ctrl)
		mcpClient.EXPECT().IsInitialized().Return(true).AnyTimes()
		setup(mcpClient)
		cfg := mcpEnabledConfig()
		cfg.MCP.ExcludeTools = excludeList
		return newMCPEngine(t, cfg, mcpClient)
	}

	t.Run("dispatches to the resolved server", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil)
			m.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable})
			m.EXPECT().ExecuteTool(gomock.Any(), mcp.Request{
				Method: string(types.ToolsCall),
				Params: map[string]any{"name": getTimeTool, "arguments": map[string]any{"timezone": "UTC"}},
			}, timeAlias).Return(&mcp.CallToolResult{Content: []mcp.ContentBlock{
				map[string]any{"type": "text", "text": "12:00"},
			}}, nil)
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`","arguments":{"timezone":"UTC"}}}`)

		require.Equal(t, http.StatusOK, w.Code)
		var resp jsonRPCTestResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Nil(t, resp.Error)
		assert.Equal(t, resultTypeComplete, resp.Result["resultType"])
		content, ok := resp.Result["content"].([]any)
		require.True(t, ok)
		require.Len(t, content, 1)
		assert.Equal(t, "12:00", content[0].(map[string]any)["text"])
	})

	t.Run("unknown tool is invalid params", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(gomock.Any()).Return("", "", errors.New("no such tool"))
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mcp_time_nope"}}`)
		assertJSONRPCError(t, w, jsonRPCInvalidParams)
	})

	t.Run("excluded tool is not callable", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsForecastTool).Return(weatherAlias, forecastTool, nil)
		}, forecastTool)

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsForecastTool+`"}}`)
		assertJSONRPCError(t, w, jsonRPCInvalidParams)
	})

	t.Run("unavailable server is an internal error", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil)
			m.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusUnavailable})
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`"}}`)
		assertJSONRPCError(t, w, jsonRPCInternalError)
	})

	t.Run("upstream failure is an internal error", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil)
			m.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable})
			m.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), timeAlias).Return(nil, errors.New(upstreamFailureMsg))
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`"}}`)
		body := assertJSONRPCError(t, w, jsonRPCInternalError)
		assert.Contains(t, body.Error.Message, upstreamFailureMsg)
	})

	t.Run("missing name is invalid params", func(t *testing.T) {
		e := engine(t, func(*mcpmocks.MockMCPClientInterface) {}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`)
		assertJSONRPCError(t, w, jsonRPCInvalidParams)
	})

	t.Run("without a client it is an internal error", func(t *testing.T) {
		w := postMCP(t, newMCPEngine(t, mcpEnabledConfig(), nil), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`"}}`)
		body := assertJSONRPCError(t, w, jsonRPCInternalError)
		assert.Equal(t, errMsgMCPUnusable, body.Error.Message)
	})
}

// TestMCPJSONRPCHandler_ProtocolErrors pins the JSON-RPC error codes for
// malformed and unsupported requests. All of them travel with HTTP 200.
func TestMCPJSONRPCHandler_ProtocolErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantID   string
	}{
		{name: "malformed json", body: `{"jsonrpc":"2.0",`, wantCode: jsonRPCParseError, wantID: `null`},
		{name: "not an object", body: `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`, wantCode: jsonRPCInvalidRequest, wantID: `null`},
		{name: "missing jsonrpc version", body: `{"id":1,"method":"tools/list"}`, wantCode: jsonRPCInvalidRequest, wantID: `1`},
		{name: "wrong jsonrpc version", body: `{"jsonrpc":"1.0","id":1,"method":"tools/list"}`, wantCode: jsonRPCInvalidRequest, wantID: `1`},
		{name: "missing method", body: `{"jsonrpc":"2.0","id":1}`, wantCode: jsonRPCInvalidRequest, wantID: `1`},
		{name: "unknown method", body: `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, wantCode: jsonRPCMethodNotFound, wantID: `1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newMCPEngine(t, mcpEnabledConfig(), nil)
			w := postMCP(t, engine, tt.body)

			resp := assertJSONRPCError(t, w, tt.wantCode)
			assert.JSONEq(t, tt.wantID, string(resp.ID))
		})
	}
}

func assertJSONRPCError(t *testing.T, w *httptest.ResponseRecorder, wantCode int) jsonRPCTestResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, "protocol errors are JSON-RPC errors, not http failures")

	var resp jsonRPCTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp.Jsonrpc)
	assert.Nil(t, resp.Result)
	require.NotNil(t, resp.Error)
	assert.Equal(t, wantCode, resp.Error.Code)
	assert.NotEmpty(t, resp.Error.Message)
	return resp
}
