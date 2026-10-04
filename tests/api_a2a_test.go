package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"

	gin "github.com/gin-gonic/gin"

	types "github.com/inference-gateway/adk/types"

	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	a2a "github.com/inference-gateway/inference-gateway/internal/a2a"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
)

const (
	a2aAgentAlias     = "mock"
	a2aUpstreamTask   = "task-1"
	a2aPrefixedTask   = a2aAgentAlias + ":" + a2aUpstreamTask
	a2aContextID      = "ctx-1"
	a2aSkillID        = "echo"
	a2aGatewayVersion = "test"
	a2aCallerAuth     = "Bearer caller-token"
	a2aStreamEvents   = 2
	a2aTestTimeout    = 5 * time.Second
	a2aMaxBody        = 1 << 20
	a2aBlockedMsg     = "a2a refused"

	a2aBlockPreCallPolicy = `package guardrails

main = {"action": "block", "message": "` + a2aBlockedMsg + `"} if {
	input.path == "` + middlewares.A2APath + `"
	input.phase == "pre_call"
}
`
	a2aBlockPostCallPolicy = `package guardrails

main = {"action": "block", "message": "` + a2aBlockedMsg + `"} if {
	input.path == "` + middlewares.A2APath + `"
	input.phase == "post_call"
}
`
)

// newA2AStubAgent is a minimal A2A agent speaking the JSON-RPC binding the ADK
// client expects: a public card, unary methods answering with a Task and
// SubscribeToTask answering with an SSE stream. It asserts the gateway strips
// the task id prefix and never forwards the caller's credentials.
func newA2AStubAgent(t *testing.T) *httptest.Server {
	t.Helper()

	streaming := true
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-card.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(types.AgentCard{
			Name:               "Mock Agent",
			Capabilities:       types.AgentCapabilities{Streaming: &streaming},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{{ID: a2aSkillID, Name: "Echo", Description: "echoes", Tags: []string{"test"}}},
		}))
	})
	mux.HandleFunc("POST /a2a", func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"), "the caller's credentials must not reach the agent")
		var req types.JSONRPCRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			return
		}
		params := types.Struct{}
		if req.Params != nil {
			params = *req.Params
		}
		assert.NotContains(t, params, "tenant", "the alias tenant is the gateway's, not the agent's")

		task := types.Task{ID: a2aUpstreamTask, ContextID: ptrString(a2aContextID), Status: types.TaskStatus{State: "TASK_STATE_COMPLETED"}}
		switch req.Method {
		case types.A2AMethodSendMessage:
			writeA2AResult(t, w, req, types.SendMessageResponse{Task: &task})
		case types.A2AMethodGetTask:
			assert.Equal(t, a2aUpstreamTask, params["id"], "the gateway strips the alias prefix")
			writeA2AResult(t, w, req, task)
		case types.A2AMethodSubscribeToTask:
			assert.Equal(t, a2aUpstreamTask, params["id"])
			writeA2AStream(t, w, req, a2aStreamEvents)
		default:
			t.Errorf("unexpected upstream method %q", req.Method)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func writeA2AResult(t *testing.T, w http.ResponseWriter, req types.JSONRPCRequest, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	assert.NoError(t, json.NewEncoder(w).Encode(types.JSONRPCSuccessResponse{JSONRPC: "2.0", ID: *req.ID, Result: result}))
}

func writeA2AStream(t *testing.T, w http.ResponseWriter, req types.JSONRPCRequest, count int) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	for i := range count {
		event := types.StreamResponse{StatusUpdate: &types.TaskStatusUpdateEvent{
			TaskID:    a2aUpstreamTask,
			ContextID: a2aContextID,
			Status:    types.TaskStatus{State: types.TaskState(fmt.Sprintf("TASK_STATE_%d", i))},
		}}
		payload, err := json.Marshal(types.JSONRPCSuccessResponse{JSONRPC: "2.0", ID: *req.ID, Result: event})
		assert.NoError(t, err)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func ptrString(s string) *string { return &s }

type a2aTestEnv struct {
	cfg      config.Config
	registry *a2a.Registry
	handler  *api.A2AHandler
}

func newA2AEnv(t *testing.T, agentURL string, telemetry *mocks.MockOpenTelemetry) a2aTestEnv {
	t.Helper()
	cfg := config.Config{
		A2A:    &config.A2AConfig{Enabled: true, ClientTimeout: a2aTestTimeout, StreamIdleTimeout: a2aTestTimeout},
		Server: &config.ServerConfig{MaxRequestBodySize: a2aMaxBody},
	}
	specs, err := a2a.ParseAgents(a2aAgentAlias + "=" + agentURL)
	require.NoError(t, err)
	registry := a2a.NewRegistry(*cfg.A2A, logger.NewNoopLogger(), a2a.DialAgents(specs))
	registry.Refresh(context.Background())

	var otelImpl otel.OpenTelemetry
	if telemetry != nil {
		otelImpl = telemetry
	}
	handler := api.NewA2AHandler(cfg, logger.NewNoopLogger(), registry, otelImpl, a2aGatewayVersion)
	return a2aTestEnv{cfg: cfg, registry: registry, handler: handler}
}

func (env a2aTestEnv) engine(extra ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(extra...)
	r.GET(middlewares.A2AAgentCardPath, env.handler.AgentCard)
	r.GET(middlewares.A2AProtectedResourcePath, env.handler.ProtectedResourceMetadata)
	r.POST(middlewares.A2APath, env.handler.JSONRPC)
	r.GET(middlewares.A2AAgentsPath, env.handler.Agents)
	return r
}

func postA2A(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, middlewares.A2APath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", a2aCallerAuth)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

type a2aEnvelope struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  map[string]any  `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeA2A(t *testing.T, w *httptest.ResponseRecorder) a2aEnvelope {
	t.Helper()
	var env a2aEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), w.Body.String())
	assert.Equal(t, "2.0", env.Jsonrpc)
	return env
}

func TestA2AAgentCardMergesRegisteredAgents(t *testing.T) {
	env := newA2AEnv(t, newA2AStubAgent(t).URL, nil)

	req := httptest.NewRequest(http.MethodGet, middlewares.A2AAgentCardPath, nil)
	req.Host = "gateway.example.com"
	req.Header.Set(middlewares.ForwardedProtoHeader, "https")
	w := httptest.NewRecorder()
	env.engine().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var card types.AgentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &card))
	assert.Equal(t, a2aGatewayVersion, card.Version)
	require.Len(t, card.Skills, 1)
	assert.Equal(t, a2a.SkillID(a2aAgentAlias, a2aSkillID), card.Skills[0].ID)
	assert.True(t, *card.Capabilities.Streaming)
	assert.False(t, *card.Capabilities.PushNotifications)
	require.Len(t, card.SupportedInterfaces, 2)
	assert.Equal(t, "https://gateway.example.com/a2a", card.SupportedInterfaces[0].URL)
	assert.Equal(t, a2aAgentAlias, *card.SupportedInterfaces[1].Tenant)
}

func TestA2AAgentsListsTheRegistry(t *testing.T) {
	srv := newA2AStubAgent(t)
	env := newA2AEnv(t, srv.URL, nil)

	w := httptest.NewRecorder()
	env.engine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, middlewares.A2AAgentsPath, nil))

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Agents []a2a.AgentStatus `json:"agents"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Agents, 1)
	assert.Equal(t, a2aAgentAlias, body.Agents[0].Alias)
	assert.Equal(t, srv.URL, body.Agents[0].URL)
	assert.True(t, body.Agents[0].Reachable)
	assert.NotNil(t, body.Agents[0].LastSeen)
}

func TestA2AUnreachableAgentIsListedNotFatal(t *testing.T) {
	env := newA2AEnv(t, "http://127.0.0.1:1", nil)

	w := httptest.NewRecorder()
	env.engine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, middlewares.A2AAgentsPath, nil))

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Agents []a2a.AgentStatus `json:"agents"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Agents, 1)
	assert.False(t, body.Agents[0].Reachable)

	w = postA2A(env.engine(), `{"jsonrpc":"2.0","id":1,"method":"GetTask","params":{"id":"`+a2aPrefixedTask+`"}}`)
	resp := decodeA2A(t, w)
	require.NotNil(t, resp.Error)
	assert.Equal(t, a2a.CodeInternalError, resp.Error.Code)
}

func TestA2AJSONRPCRelaysToTheNamedAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	telemetry := mocks.NewMockOpenTelemetry(ctrl)
	telemetry.EXPECT().RecordA2ARequest(gomock.Any(), a2aAgentAlias, string(types.A2AMethodSendMessage), "ok", gomock.Any())
	telemetry.EXPECT().RecordA2ARequest(gomock.Any(), a2aAgentAlias, string(types.A2AMethodGetTask), "ok", gomock.Any())
	telemetry.EXPECT().RecordA2ARequest(gomock.Any(), "", string(types.A2AMethodGetTask), fmt.Sprint(a2a.CodeInvalidParams), gomock.Any())
	env := newA2AEnv(t, newA2AStubAgent(t).URL, telemetry)
	engine := env.engine()

	w := postA2A(engine, `{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"messageId":"m1","role":"ROLE_USER","parts":[{"text":"hi"}],"metadata":{"agent":"`+a2aAgentAlias+`"}}}}`)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeA2A(t, w)
	require.Nil(t, resp.Error, w.Body.String())
	assert.JSONEq(t, `1`, string(resp.ID), "the client's id is echoed, not the ADK's")
	task := resp.Result["task"].(map[string]any)
	assert.Equal(t, a2aPrefixedTask, task["id"])

	w = postA2A(engine, `{"jsonrpc":"2.0","id":"abc","method":"GetTask","params":{"id":"`+a2aPrefixedTask+`"}}`)
	resp = decodeA2A(t, w)
	require.Nil(t, resp.Error, w.Body.String())
	assert.JSONEq(t, `"abc"`, string(resp.ID))
	assert.Equal(t, a2aPrefixedTask, resp.Result["id"])
	assert.Equal(t, a2aContextID, resp.Result["contextId"])

	w = postA2A(engine, `{"jsonrpc":"2.0","id":2,"method":"GetTask","params":{"id":"`+a2aUpstreamTask+`"}}`)
	resp = decodeA2A(t, w)
	require.NotNil(t, resp.Error)
	assert.Equal(t, a2a.CodeInvalidParams, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, a2aAgentAlias)
}

func TestA2AJSONRPCEnvelopeErrors(t *testing.T) {
	env := newA2AEnv(t, newA2AStubAgent(t).URL, nil)
	engine := env.engine()

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{name: "invalid json is a parse error", body: `{not json`, wantCode: a2a.CodeParseError},
		{name: "wrong version is an invalid request", body: `{"jsonrpc":"1.0","id":1,"method":"GetTask"}`, wantCode: a2a.CodeInvalidRequest},
		{name: "missing method is an invalid request", body: `{"jsonrpc":"2.0","id":1}`, wantCode: a2a.CodeInvalidRequest},
		{name: "unknown method is method not found", body: `{"jsonrpc":"2.0","id":1,"method":"tasks/get"}`, wantCode: a2a.CodeMethodNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := postA2A(engine, tt.body)
			require.Equal(t, http.StatusOK, w.Code)
			resp := decodeA2A(t, w)
			require.NotNil(t, resp.Error)
			assert.Equal(t, tt.wantCode, resp.Error.Code)
		})
	}

	t.Run("a notification is accepted and not relayed", func(t *testing.T) {
		w := postA2A(engine, `{"jsonrpc":"2.0","method":"GetTask","params":{"id":"`+a2aPrefixedTask+`"}}`)
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Empty(t, w.Body.String())
	})
}

func TestA2ASubscribeToTaskRelaysTheStream(t *testing.T) {
	env := newA2AEnv(t, newA2AStubAgent(t).URL, nil)

	w := postA2A(env.engine(), `{"jsonrpc":"2.0","id":7,"method":"SubscribeToTask","params":{"id":"`+a2aPrefixedTask+`"}}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	events := make([]a2aEnvelope, 0)
	scanner := bufio.NewScanner(w.Body)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var event a2aEnvelope
		require.NoError(t, json.Unmarshal([]byte(data), &event))
		events = append(events, event)
	}
	require.Len(t, events, a2aStreamEvents)
	for _, event := range events {
		assert.JSONEq(t, `7`, string(event.ID))
		update := event.Result["statusUpdate"].(map[string]any)
		assert.Equal(t, a2aPrefixedTask, update["taskId"])
	}
}

func TestA2AGuardrailsAnswerWithJSONRPCEnvelope(t *testing.T) {
	tests := []struct {
		name   string
		policy string
	}{
		{name: "pre_call block", policy: a2aBlockPreCallPolicy},
		{name: "post_call block", policy: a2aBlockPostCallPolicy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newA2AEnv(t, newA2AStubAgent(t).URL, nil)
			env.cfg.Guardrails = &config.GuardrailsConfig{Enabled: true, FailMode: guardrails.FailModeClosed}
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.rego"), []byte(tt.policy), 0o600))
			evaluator, err := guardrails.NewEvaluator(context.Background(), dir)
			require.NoError(t, err)
			guard := middlewares.NewGuardrailsMiddleware(evaluator, nil, nil, logger.NewNoopLogger(), nil, env.cfg)

			w := postA2A(env.engine(guard.Middleware()), `{"jsonrpc":"2.0","id":9,"method":"GetTask","params":{"id":"`+a2aPrefixedTask+`"}}`)

			require.Equal(t, http.StatusForbidden, w.Code)
			resp := decodeA2A(t, w)
			require.NotNil(t, resp.Error)
			assert.Equal(t, middlewares.JSONRPCGuardrailBlocked, resp.Error.Code)
			assert.Equal(t, a2aBlockedMsg, resp.Error.Message)
			assert.JSONEq(t, `9`, string(resp.ID))
		})
	}
}

func TestA2AProtectedResourceMetadata(t *testing.T) {
	env := newA2AEnv(t, newA2AStubAgent(t).URL, nil)

	w := httptest.NewRecorder()
	env.engine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, middlewares.A2AProtectedResourcePath, nil))
	assert.Equal(t, http.StatusNotFound, w.Code, "nothing to serve without an authorization server")

	env.cfg.Auth = &config.AuthConfig{Enabled: true, OidcIssuer: "https://idp.example.com/realms/gateway"}
	env.handler = api.NewA2AHandler(env.cfg, logger.NewNoopLogger(), env.registry, nil, a2aGatewayVersion)
	req := httptest.NewRequest(http.MethodGet, middlewares.A2AProtectedResourcePath, nil)
	req.Host = "gateway.example.com"
	w = httptest.NewRecorder()
	env.engine().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.Equal(t, "http://gateway.example.com/a2a", doc["resource"])
	assert.Equal(t, []any{"https://idp.example.com/realms/gateway"}, doc["authorization_servers"])
}
