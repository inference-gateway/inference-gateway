package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	otelapi "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"
	trace "go.opentelemetry.io/otel/sdk/trace"

	logger "github.com/inference-gateway/inference-gateway/logger"
)

const (
	rpcTestTool   = "echo"
	rpcTestCursor = "page-2"
)

// rpcTestRequest is what the stub decodes from every outbound request.
type rpcTestRequest struct {
	ID     any    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Meta      RequestMetaObject `json:"_meta"`
		Cursor    *string           `json:"cursor"`
		Name      string            `json:"name"`
		Arguments *map[string]any   `json:"arguments"`
	} `json:"params"`
}

// newRPCTestClient points an MCPClient at a stub whose handler gets the decoded
// request and writes the reply.
func newRPCTestClient(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, req rpcTestRequest)) (*MCPClient, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcTestRequest
		if assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			handle(w, r, req)
		}
	}))
	t.Cleanup(srv.Close)
	return NewMCPClient(nil, logger.NewNoopLogger(), newStubMCPConfig()).(*MCPClient), srv.URL
}

func writeRPCResult(t *testing.T, w http.ResponseWriter, id any, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}))
}

// TestRPCRequestMetadata asserts every request is a stateless 2026-07-28
// request: version in _meta and headers, the gateway as clientInfo, the trace
// context propagated, and no credentials. A call without arguments still sends
// an empty arguments object.
func TestRPCRequestMetadata(t *testing.T) {
	otelapi.SetTracerProvider(trace.NewTracerProvider())
	otelapi.SetTextMapPropagator(propagation.TraceContext{})

	mc, url := newRPCTestClient(t, func(w http.ResponseWriter, r *http.Request, req rpcTestRequest) {
		assert.Equal(t, ProtocolVersion, r.Header.Get(HeaderProtocolVersion))
		assert.Equal(t, string(ToolsCall), r.Header.Get(HeaderMethod))
		assert.Equal(t, rpcTestTool, r.Header.Get(HeaderName))
		assert.NotEmpty(t, r.Header.Get("traceparent"), "mcp requests must carry the trace context")
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Equal(t, ProtocolVersion, req.Params.Meta.IoModelcontextprotocolProtocolVersion)
		assert.Equal(t, &GatewayInfo, req.Params.Meta.IoModelcontextprotocolClientInfo)
		assert.NotNil(t, req.Params.Arguments, "arguments must be sent even when empty")
		writeRPCResult(t, w, req.ID, map[string]any{"resultType": ResultTypeComplete, "content": []any{}})
	})

	ctx, span := otelapi.Tracer("test").Start(context.Background(), "root")
	defer span.End()
	_, err := mc.callTool(ctx, url, rpcTestTool, nil)
	require.NoError(t, err)
}

// TestRPCListToolsPaginates asserts every page is fetched and each tool gets an
// object input schema even when the server sends none.
func TestRPCListToolsPaginates(t *testing.T) {
	mc, url := newRPCTestClient(t, func(w http.ResponseWriter, r *http.Request, req rpcTestRequest) {
		if req.Params.Cursor == nil {
			writeRPCResult(t, w, req.ID, map[string]any{"tools": []any{map[string]any{"name": "first"}}, "nextCursor": rpcTestCursor})
			return
		}
		assert.Equal(t, rpcTestCursor, *req.Params.Cursor)
		writeRPCResult(t, w, req.ID, map[string]any{"tools": []any{map[string]any{"name": "second", "inputSchema": map[string]any{"type": "object"}}}})
	})

	tools, err := mc.listTools(context.Background(), url)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "first", tools[0].Name)
	assert.Equal(t, "second", tools[1].Name)
	assert.NotNil(t, tools[0].InputSchema)
}

// TestRPCCallTool pins how tools/call results come back: everything the server
// sent is kept, a missing resultType reads as complete, and input_required or a
// JSON-RPC error is an error.
func TestRPCCallTool(t *testing.T) {
	tests := []struct {
		name    string
		reply   func(t *testing.T, w http.ResponseWriter, id any)
		wantErr string
		check   func(t *testing.T, result *CallToolResult)
	}{
		{
			name: "isError and non-text content are kept",
			reply: func(t *testing.T, w http.ResponseWriter, id any) {
				writeRPCResult(t, w, id, map[string]any{
					"resultType": ResultTypeComplete,
					"isError":    true,
					"content":    []any{map[string]any{"type": "image", "data": "aGk=", "mimeType": "image/png"}},
				})
			},
			check: func(t *testing.T, result *CallToolResult) {
				require.NotNil(t, result.IsError)
				assert.True(t, *result.IsError)
				require.Len(t, result.Content, 1)
				assert.Equal(t, "image", result.Content[0].(map[string]any)["type"])
			},
		},
		{
			name: "missing resultType reads as complete",
			reply: func(t *testing.T, w http.ResponseWriter, id any) {
				writeRPCResult(t, w, id, map[string]any{"content": []any{}})
			},
			check: func(t *testing.T, result *CallToolResult) {
				assert.Equal(t, ResultTypeComplete, result.ResultType)
			},
		},
		{
			name: "sse response after a notification",
			reply: func(t *testing.T, w http.ResponseWriter, id any) {
				w.Header().Set("Content-Type", contentTypeSSE)
				_, err := fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n"+
					"data: {\"jsonrpc\":\"2.0\",\"id\":%v,\n"+
					"data: \"result\":{\"resultType\":\"complete\",\"content\":[{\"type\":\"text\",\"text\":\"ok\"}]}}\n\n", id)
				assert.NoError(t, err)
			},
			check: func(t *testing.T, result *CallToolResult) {
				require.Len(t, result.Content, 1)
				assert.Equal(t, "ok", result.Content[0].(map[string]any)["text"])
			},
		},
		{
			name: "input_required is an error",
			reply: func(t *testing.T, w http.ResponseWriter, id any) {
				writeRPCResult(t, w, id, map[string]any{"resultType": resultTypeInputRequired})
			},
			wantErr: ErrInputRequired.Error(),
		},
		{
			name: "json-rpc error with a 400 is an error",
			reply: func(t *testing.T, w http.ResponseWriter, id any) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32022, "message": "unsupported protocol version"},
				}))
			},
			wantErr: "json-rpc error -32022: unsupported protocol version",
		},
		{
			name: "non json-rpc reply is an error",
			reply: func(t *testing.T, w http.ResponseWriter, id any) {
				http.Error(w, "no session", http.StatusBadRequest)
			},
			wantErr: "http 400 with no json-rpc response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, url := newRPCTestClient(t, func(w http.ResponseWriter, r *http.Request, req rpcTestRequest) {
				tt.reply(t, w, req.ID)
			})

			result, err := mc.callTool(context.Background(), url, rpcTestTool, map[string]any{"text": "hi"})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, result)
		})
	}
}
