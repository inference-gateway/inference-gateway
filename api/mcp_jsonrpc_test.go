package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
	mcpmocks "github.com/inference-gateway/inference-gateway/tests/mocks/mcp"

	gin "github.com/gin-gonic/gin"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
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
	legacyVersion      = "2025-06-18"
	metaClientInfo     = "io.modelcontextprotocol/clientInfo"
	metaClientCaps     = "io.modelcontextprotocol/clientCapabilities"
)

// Guardrail fixtures: the policies POST /mcp is evaluated against and the
// tools/call body they see.
const (
	policyFileName    = "policy.rego"
	toolOutputText    = "12:00"
	argsBlockedMsg    = "tool arguments refused"
	outputBlockedMsg  = "tool output refused"
	preCallBlockedMsg = "mcp endpoint refused"
	toolsCallBody     = `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"` + nsGetTimeTool + `","arguments":{"timezone":"UTC"}}}`

	// allowPolicy allows every phase.
	allowPolicy = `package guardrails

main = {"action": "allow"}
`

	// blockToolArgsPolicy also pins the tool-phase input shape: TOOL_CALL as the
	// method and the namespaced tool name as the path.
	blockToolArgsPolicy = `package guardrails

main = {"action": "block", "message": "` + argsBlockedMsg + `"} if {
	input.method == "TOOL_CALL"
	input.phase == "tool_args"
	input.path == "` + nsGetTimeTool + `"
}
`

	// blockToolOutputPolicy matches the upstream result, which only reaches the
	// policy when tool_output passes the tool output as the request body.
	blockToolOutputPolicy = `package guardrails

main = {"action": "block", "message": "` + outputBlockedMsg + `"} if {
	input.phase == "tool_output"
	contains(input.request.body, "` + toolOutputText + `")
}
`

	// conflictingPolicy compiles but fails at evaluation time, which is what
	// GUARDRAILS_FAIL_MODE decides on.
	conflictingPolicy = `package guardrails

main = {"action": "allow"} if {
	input.phase == "tool_args"
}

main = {"action": "block"} if {
	input.phase == "tool_args"
}
`

	// blockMCPPathPolicy blocks at the pre_call phase the middleware runs.
	blockMCPPathPolicy = `package guardrails

main = {"action": "block", "message": "` + preCallBlockedMsg + `"} if {
	input.path == "` + middlewares.MCPPath + `"
	input.phase == "pre_call"
}
`
)

// jsonRPCTestResponse is the decoded envelope the assertions work against; the
// handler writes the generated types.MCPJSONRPCResponse.
type jsonRPCTestResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  map[string]any  `json:"result"`
	Error   *struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	} `json:"error"`
}

// newMCPEngine wires POST /mcp exactly as cmd/gateway/main.go does, with
// guardrails off and telemetry disabled.
func newMCPEngine(t *testing.T, cfg config.Config, mcpClient mcp.MCPClientInterface) *gin.Engine {
	t.Helper()
	return newMCPEngineWithAgent(t, cfg, mcpClient, mcp.NewAgent(logger.NewNoopLogger(), mcpClient))
}

// newMCPEngineWithAgent wires POST /mcp with the agent the caller supplies, the
// way main.go hands the router the guardrails- and telemetry-configured agent.
func newMCPEngineWithAgent(t *testing.T, cfg config.Config, mcpClient mcp.MCPClientInterface, agent *mcp.Agent) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := NewRouter(cfg, logger.NewNoopLogger(), nil, nil, mcpClient, agent, nil, nil, nil)
	r := gin.New()
	r.POST(middlewares.MCPPath, router.MCPJSONRPCHandler)
	return r
}

// newEvaluator compiles a single policy the way GUARDRAILS_POLICY_DIR is loaded
// at startup.
func newEvaluator(t *testing.T, policy string) *guardrails.Evaluator {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, policyFileName), []byte(policy), 0o600))
	evaluator, err := guardrails.NewEvaluator(context.Background(), dir)
	require.NoError(t, err)
	return evaluator
}

// postMCP sends body the way a 2026-07-28 client does: the protocol version in
// params._meta, mirrored with the method and tool name into headers. A body
// that is not a JSON object goes out untouched.
func postMCP(t *testing.T, engine *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	var msg map[string]any
	if json.Unmarshal([]byte(body), &msg) != nil {
		return postMCPRaw(t, engine, body, http.Header{})
	}

	params, _ := msg["params"].(map[string]any)
	if params == nil {
		params = make(map[string]any)
	}
	params["_meta"] = map[string]any{
		metaProtocolVersion: mcp.ProtocolVersion,
		metaClientInfo:      map[string]any{"name": "test", "version": "1.0.0"},
		metaClientCaps:      map[string]any{},
	}
	msg["params"] = params

	header := http.Header{}
	header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion)
	method, _ := msg["method"].(string)
	header.Set(mcp.HeaderMethod, method)
	if name, ok := params["name"].(string); ok {
		header.Set(mcp.HeaderName, name)
	}

	raw, err := json.Marshal(msg)
	require.NoError(t, err)
	return postMCPRaw(t, engine, string(raw), header)
}

func postMCPRaw(t *testing.T, engine *gin.Engine, body string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, middlewares.MCPPath, strings.NewReader(body))
	req.Header = header
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

// TestMCPJSONRPCHandler_Origin asserts a browser request is refused before
// anything else runs: MCP clients never send an Origin.
func TestMCPJSONRPCHandler_Origin(t *testing.T) {
	engine := newMCPEngine(t, mcpEnabledConfig(), nil)
	header := http.Header{}
	header.Set(headerOrigin, "http://evil.example")
	w := postMCPRaw(t, engine, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, header)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, errMsgMCPOrigin, body.Error)
}

// TestMCPJSONRPCHandler_Discover asserts server/discover advertises 2026-07-28
// as the only version, tools as the only capability, and names the gateway.
func TestMCPJSONRPCHandler_Discover(t *testing.T) {
	engine := newMCPEngine(t, mcpEnabledConfig(), nil)
	w := postMCP(t, engine, `{"jsonrpc":"2.0","id":1,"method":"server/discover"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var resp jsonRPCTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Nil(t, resp.Error)
	assert.JSONEq(t, `1`, string(resp.ID))
	assert.Equal(t, []any{mcp.ProtocolVersion}, resp.Result["supportedVersions"])
	assert.Equal(t, map[string]any{"tools": map[string]any{"listChanged": false}}, resp.Result["capabilities"])
	assert.Equal(t, mcp.ResultTypeComplete, resp.Result["resultType"])
	assert.Equal(t, string(mcp.DiscoverResultCacheScopePrivate), resp.Result["cacheScope"])
	assert.Contains(t, resp.Result, "ttlMs")
	assert.Equal(t, map[string]any{metaServerInfo: map[string]any{"name": mcp.GatewayInfo.Name, "version": mcp.GatewayInfo.Version}}, resp.Result["_meta"])
}

// TestMCPJSONRPCHandler_RequestMetadata pins the 2026-07-28 request metadata
// rules: headers must be present and agree with the body, and any version but
// 2026-07-28 is refused with the supported list.
func TestMCPJSONRPCHandler_RequestMetadata(t *testing.T) {
	encodedName := base64HeaderPrefix + base64.StdEncoding.EncodeToString([]byte(nsGetTimeTool)) + base64HeaderSuffix
	toolParams := map[string]any{"name": nsGetTimeTool}

	tests := []struct {
		name        string
		method      string
		params      map[string]any
		metaVersion string
		header      map[string]string
		wantStatus  int
		wantCode    int
		wantData    map[string]any
	}{
		{
			name:   "legacy initialize handshake",
			method: "initialize", params: map[string]any{"protocolVersion": legacyVersion},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "missing protocol version header", method: string(types.ToolsList), metaVersion: mcp.ProtocolVersion,
			header:     map[string]string{mcp.HeaderMethod: string(types.ToolsList)},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "unsupported version", method: string(types.ToolsList), metaVersion: legacyVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: legacyVersion, mcp.HeaderMethod: string(types.ToolsList)},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCUnsupportedVersion,
			wantData: map[string]any{"requested": legacyVersion, "supported": []any{mcp.ProtocolVersion}},
		},
		{
			name: "header disagrees with _meta", method: string(types.ToolsList), metaVersion: legacyVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsList)},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "missing _meta", method: string(types.ToolsList),
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsList)},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "method header disagrees with body", method: string(types.ToolsList), metaVersion: mcp.ProtocolVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsCall)},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "tools/call without Mcp-Name", method: string(types.ToolsCall), params: toolParams, metaVersion: mcp.ProtocolVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsCall)},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "tools/call with a different Mcp-Name", method: string(types.ToolsCall), params: toolParams, metaVersion: mcp.ProtocolVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsCall), mcp.HeaderName: nsForecastTool},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			name: "tools/call with malformed base64 Mcp-Name", method: string(types.ToolsCall), params: toolParams, metaVersion: mcp.ProtocolVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsCall), mcp.HeaderName: base64HeaderPrefix + "!!!" + base64HeaderSuffix},
			wantStatus: http.StatusBadRequest, wantCode: jsonRPCHeaderMismatch,
		},
		{
			// Passing validation lands in tools/call, which has no client here.
			name: "tools/call with base64 Mcp-Name passes validation", method: string(types.ToolsCall), params: toolParams, metaVersion: mcp.ProtocolVersion,
			header:     map[string]string{mcp.HeaderProtocolVersion: mcp.ProtocolVersion, mcp.HeaderMethod: string(types.ToolsCall), mcp.HeaderName: encodedName},
			wantStatus: http.StatusOK, wantCode: jsonRPCInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := make(map[string]any)
			for k, v := range tt.params {
				params[k] = v
			}
			if tt.metaVersion != "" {
				params["_meta"] = map[string]any{metaProtocolVersion: tt.metaVersion}
			}
			body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": tt.method, "params": params})
			require.NoError(t, err)

			header := http.Header{}
			for k, v := range tt.header {
				header.Set(k, v)
			}

			w := postMCPRaw(t, newMCPEngine(t, mcpEnabledConfig(), nil), string(body), header)
			resp := assertJSONRPCError(t, w, tt.wantStatus, tt.wantCode)
			if tt.wantData != nil {
				assert.Equal(t, tt.wantData, resp.Error.Data)
			}
		})
	}
}

// TestMCPJSONRPCHandler_DuplicateHeaders asserts a mirrored header sent twice
// is rejected even when the first copy matches the body, since an intermediary
// may act on the other copy.
func TestMCPJSONRPCHandler_DuplicateHeaders(t *testing.T) {
	for _, name := range []string{mcp.HeaderProtocolVersion, mcp.HeaderMethod, mcp.HeaderName} {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": string(types.ToolsCall),
				"params": map[string]any{"name": nsGetTimeTool, "_meta": map[string]any{metaProtocolVersion: mcp.ProtocolVersion}},
			})
			require.NoError(t, err)

			header := http.Header{}
			header.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion)
			header.Set(mcp.HeaderMethod, string(types.ToolsCall))
			header.Set(mcp.HeaderName, nsGetTimeTool)
			header.Add(name, nsForecastTool)

			w := postMCPRaw(t, newMCPEngine(t, mcpEnabledConfig(), nil), string(body), header)
			assertJSONRPCError(t, w, http.StatusBadRequest, jsonRPCHeaderMismatch)
		})
	}
}

// TestMCPJSONRPCHandler_Notification asserts a request without an id is
// answered with 202 and no body, as JSON-RPC requires for notifications.
func TestMCPJSONRPCHandler_Notification(t *testing.T) {
	engine := newMCPEngine(t, mcpEnabledConfig(), nil)
	w := postMCP(t, engine, `{"jsonrpc":"2.0","method":"notifications/cancelled"}`)

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
		mcpClient.EXPECT().GetServerTools(timeAlias).Return([]mcp.Tool{{Name: getTimeTool}}, nil).AnyTimes()
		mcpClient.EXPECT().GetServerTools(weatherAlias).Return([]mcp.Tool{{Name: forecastTool}}, nil).AnyTimes()
		setup(mcpClient)
		cfg := mcpEnabledConfig()
		cfg.MCP.ExcludeTools = excludeList
		return newMCPEngine(t, cfg, mcpClient)
	}

	t.Run("dispatches to the resolved server", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil).AnyTimes()
			m.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable})
			m.EXPECT().ExecuteTool(gomock.Any(), mcp.Request{
				Method: string(types.ToolsCall),
				Params: map[string]any{"name": getTimeTool, "arguments": map[string]any{"timezone": "UTC"}},
			}, timeAlias).Return(&mcp.CallToolResult{ResultType: mcp.ResultTypeComplete, Content: []mcp.ContentBlock{
				map[string]any{"type": "text", "text": "12:00"},
			}}, nil)
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`","arguments":{"timezone":"UTC"}}}`)

		require.Equal(t, http.StatusOK, w.Code)
		var resp jsonRPCTestResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Nil(t, resp.Error)
		assert.Equal(t, mcp.ResultTypeComplete, resp.Result["resultType"])
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
		assertJSONRPCError(t, w, http.StatusOK, jsonRPCInvalidParams)
	})

	t.Run("tool the server never listed is invalid params", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(gomock.Any()).Return(timeAlias, "nope", nil)
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mcp_time_nope"}}`)
		assertJSONRPCError(t, w, http.StatusOK, jsonRPCInvalidParams)
	})

	t.Run("excluded tool is not callable", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsForecastTool).Return(weatherAlias, forecastTool, nil)
		}, forecastTool)

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsForecastTool+`"}}`)
		assertJSONRPCError(t, w, http.StatusOK, jsonRPCInvalidParams)
	})

	t.Run("unavailable server is an internal error", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil)
			m.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusUnavailable})
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`"}}`)
		assertJSONRPCError(t, w, http.StatusOK, jsonRPCInternalError)
	})

	t.Run("upstream failure is an internal error", func(t *testing.T) {
		e := engine(t, func(m *mcpmocks.MockMCPClientInterface) {
			m.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil).AnyTimes()
			m.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable})
			m.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), timeAlias).Return(nil, errors.New(upstreamFailureMsg))
		}, "")

		w := postMCP(t, e, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`"}}`)
		body := assertJSONRPCError(t, w, http.StatusOK, jsonRPCInternalError)
		assert.Equal(t, "mcp server "+timeAlias+" failed", body.Error.Message)
		assert.NotContains(t, body.Error.Message, upstreamFailureMsg)
	})

	t.Run("without a client it is an internal error", func(t *testing.T) {
		w := postMCP(t, newMCPEngine(t, mcpEnabledConfig(), nil), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+nsGetTimeTool+`"}}`)
		body := assertJSONRPCError(t, w, http.StatusOK, jsonRPCInternalError)
		assert.Equal(t, errMsgMCPUnusable, body.Error.Message)
	})
}

// TestMCPJSONRPCHandler_ToolsCallGuardrails asserts tools/call runs tool_args
// before the upstream call and tool_output after it, GUARDRAILS_FAIL_MODE
// decides evaluation errors, and every block is a JSON-RPC error envelope.
func TestMCPJSONRPCHandler_ToolsCallGuardrails(t *testing.T) {
	tests := []struct {
		name         string
		policy       string
		failMode     string
		wantUpstream bool
		wantMessage  string
	}{
		{name: "allowed call reaches the server", policy: allowPolicy, failMode: guardrails.FailModeClosed, wantUpstream: true},
		{name: "tool_args block never reaches the server", policy: blockToolArgsPolicy, failMode: guardrails.FailModeClosed, wantMessage: argsBlockedMsg},
		{name: "tool_output block withholds the result", policy: blockToolOutputPolicy, failMode: guardrails.FailModeClosed, wantUpstream: true, wantMessage: outputBlockedMsg},
		{name: "evaluation error blocks when failing closed", policy: conflictingPolicy, failMode: guardrails.FailModeClosed, wantMessage: guardrails.MsgEvaluationFailed},
		{name: "evaluation error allows when failing open", policy: conflictingPolicy, failMode: guardrails.FailModeOpen, wantUpstream: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mcpClient := mcpmocks.NewMockMCPClientInterface(ctrl)
			mcpClient.EXPECT().IsInitialized().Return(true).AnyTimes()
			mcpClient.EXPECT().GetServerTools(timeAlias).Return([]mcp.Tool{{Name: getTimeTool}}, nil).AnyTimes()
			mcpClient.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil).AnyTimes()
			mcpClient.EXPECT().GetAllServerStatuses().
				Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable}).AnyTimes()

			upstreamCalls := 0
			if tt.wantUpstream {
				upstreamCalls = 1
			}
			mcpClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), timeAlias).
				Return(&mcp.CallToolResult{ResultType: mcp.ResultTypeComplete, Content: []mcp.ContentBlock{
					map[string]any{"type": "text", "text": toolOutputText},
				}}, nil).Times(upstreamCalls)

			agent := mcp.NewAgent(logger.NewNoopLogger(), mcpClient)
			agent.SetGuardrails(newEvaluator(t, tt.policy), tt.failMode)
			w := postMCP(t, newMCPEngineWithAgent(t, mcpEnabledConfig(), mcpClient, agent), toolsCallBody)

			if tt.wantMessage == "" {
				require.Equal(t, http.StatusOK, w.Code)
				var resp jsonRPCTestResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				require.Nil(t, resp.Error)
				assert.Equal(t, mcp.ResultTypeComplete, resp.Result["resultType"])
				return
			}

			resp := assertJSONRPCError(t, w, http.StatusForbidden, middlewares.JSONRPCGuardrailBlocked)
			assert.Equal(t, tt.wantMessage, resp.Error.Message)
			assert.JSONEq(t, `7`, string(resp.ID))
		})
	}
}

// TestMCPJSONRPCHandler_PreCallBlockEnvelope asserts a pre_call block on /mcp
// answers with a JSON-RPC error envelope echoing the request id, not the plain
// error object an MCP client cannot parse.
func TestMCPJSONRPCHandler_PreCallBlockEnvelope(t *testing.T) {
	cfg := mcpEnabledConfig()
	cfg.Guardrails = &config.GuardrailsConfig{Enabled: true, FailMode: guardrails.FailModeClosed}

	gin.SetMode(gin.TestMode)
	router := NewRouter(cfg, logger.NewNoopLogger(), nil, nil, nil, nil, nil, nil, nil)
	engine := gin.New()
	engine.Use(middlewares.NewGuardrailsMiddleware(newEvaluator(t, blockMCPPathPolicy), nil, nil, logger.NewNoopLogger(), nil, cfg).Middleware())
	engine.POST(middlewares.MCPPath, router.MCPJSONRPCHandler)

	w := postMCP(t, engine, toolsCallBody)

	resp := assertJSONRPCError(t, w, http.StatusForbidden, middlewares.JSONRPCGuardrailBlocked)
	assert.Equal(t, preCallBlockedMsg, resp.Error.Message)
	assert.JSONEq(t, `7`, string(resp.ID))
}

// TestMCPJSONRPCHandler_ToolsCallMetrics asserts a tools/call that resolves to
// an advertised tool is counted, and that a name that resolves to nothing is
// not, so client-supplied strings cannot inflate label cardinality.
func TestMCPJSONRPCHandler_ToolsCallMetrics(t *testing.T) {
	newClient := func(ctrl *gomock.Controller) *mcpmocks.MockMCPClientInterface {
		mcpClient := mcpmocks.NewMockMCPClientInterface(ctrl)
		mcpClient.EXPECT().IsInitialized().Return(true).AnyTimes()
		mcpClient.EXPECT().GetServerTools(timeAlias).Return([]mcp.Tool{{Name: getTimeTool}}, nil).AnyTimes()
		return mcpClient
	}

	t.Run("resolved tool is recorded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcpClient := newClient(ctrl)
		mcpClient.EXPECT().ResolveTool(nsGetTimeTool).Return(timeAlias, getTimeTool, nil).AnyTimes()
		mcpClient.EXPECT().GetAllServerStatuses().Return(map[string]mcp.ServerStatus{timeAlias: mcp.ServerStatusAvailable})
		mcpClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), timeAlias).
			Return(&mcp.CallToolResult{ResultType: mcp.ResultTypeComplete}, nil)

		telemetry := mocks.NewMockOpenTelemetry(ctrl)
		telemetry.EXPECT().RecordToolCall(gomock.Any(), otel.SourceGateway, otel.TeamUnknown, "", "", mcp.ToolTypeMCP, nsGetTimeTool).Times(1)

		agent := mcp.NewAgent(logger.NewNoopLogger(), mcpClient)
		agent.SetTelemetry(telemetry)
		engine := newMCPEngineWithAgent(t, mcpEnabledConfig(), mcpClient, agent)
		assert.Equal(t, http.StatusOK, postMCP(t, engine, toolsCallBody).Code)
	})

	t.Run("unknown tool is not recorded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcpClient := newClient(ctrl)
		mcpClient.EXPECT().ResolveTool(gomock.Any()).Return("", "", errors.New("no such tool"))

		telemetry := mocks.NewMockOpenTelemetry(ctrl)
		telemetry.EXPECT().RecordToolCall(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		agent := mcp.NewAgent(logger.NewNoopLogger(), mcpClient)
		agent.SetTelemetry(telemetry)
		engine := newMCPEngineWithAgent(t, mcpEnabledConfig(), mcpClient, agent)
		w := postMCP(t, engine, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mcp_time_nope"}}`)
		assertJSONRPCError(t, w, http.StatusOK, jsonRPCInvalidParams)
	})
}

// TestMCPJSONRPCHandler_ProtocolErrors pins the JSON-RPC error codes for
// malformed and unsupported requests. They travel with HTTP 200, except an
// unknown method, which 2026-07-28 pins to 404.
func TestMCPJSONRPCHandler_ProtocolErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   int
		wantID     string
	}{
		{name: "malformed json", body: `{"jsonrpc":"2.0",`, wantStatus: http.StatusOK, wantCode: jsonRPCParseError, wantID: `null`},
		{name: "not an object", body: `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`, wantStatus: http.StatusOK, wantCode: jsonRPCInvalidRequest, wantID: `null`},
		{name: "missing jsonrpc version", body: `{"id":1,"method":"tools/list"}`, wantStatus: http.StatusOK, wantCode: jsonRPCInvalidRequest, wantID: `1`},
		{name: "wrong jsonrpc version", body: `{"jsonrpc":"1.0","id":1,"method":"tools/list"}`, wantStatus: http.StatusOK, wantCode: jsonRPCInvalidRequest, wantID: `1`},
		{name: "missing method", body: `{"jsonrpc":"2.0","id":1}`, wantStatus: http.StatusOK, wantCode: jsonRPCInvalidRequest, wantID: `1`},
		{name: "unknown method", body: `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, wantStatus: http.StatusNotFound, wantCode: jsonRPCMethodNotFound, wantID: `1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newMCPEngine(t, mcpEnabledConfig(), nil)
			w := postMCP(t, engine, tt.body)

			resp := assertJSONRPCError(t, w, tt.wantStatus, tt.wantCode)
			assert.JSONEq(t, tt.wantID, string(resp.ID))
		})
	}
}

func assertJSONRPCError(t *testing.T, w *httptest.ResponseRecorder, wantStatus, wantCode int) jsonRPCTestResponse {
	t.Helper()
	require.Equal(t, wantStatus, w.Code)

	var resp jsonRPCTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2.0", resp.Jsonrpc)
	assert.Nil(t, resp.Result)
	require.NotNil(t, resp.Error)
	assert.Equal(t, wantCode, resp.Error.Code)
	assert.NotEmpty(t, resp.Error.Message)
	return resp
}
