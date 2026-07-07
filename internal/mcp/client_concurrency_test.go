package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

func newMCPStubServer(t *testing.T, initDelay time.Duration, initCount *atomic.Int32) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		require.NoError(t, json.Unmarshal(body, &req))

		if req.ID == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		var result any
		switch req.Method {
		case "initialize":
			if initCount != nil {
				initCount.Add(1)
			}
			time.Sleep(initDelay)
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "stub", "version": "1.0.0"},
			}
		case "tools/list":
			result = map[string]any{
				"tools": []map[string]any{
					{"name": "echo", "description": "echo", "inputSchema": map[string]any{"type": "object"}},
				},
			}
		case "tools/call":
			result = map[string]any{
				"content": []map[string]any{{"type": "text", "text": "ok"}},
			}
		default:
			result = map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}))
	}))

	t.Cleanup(srv.Close)
	return srv
}

func newStubMCPConfig() config.Config {
	return config.Config{
		MCP: &config.MCPConfig{
			DialTimeout:           2 * time.Second,
			TlsHandshakeTimeout:   2 * time.Second,
			ResponseHeaderTimeout: 2 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ClientTimeout:         5 * time.Second,
			RequestTimeout:        5 * time.Second,
			MaxRetries:            0,
			RetryInterval:         10 * time.Millisecond,
			InitialBackoff:        10 * time.Millisecond,
			EnableReconnect:       true,
			ReconnectInterval:     1 * time.Hour,
		},
	}
}

func TestMCPClientConcurrentReadersDuringReconnection(t *testing.T) {
	srv := newMCPStubServer(t, 0, nil)

	mc := NewMCPClient([]string{srv.URL}, logger.NewNoopLogger(), newStubMCPConfig()).(*MCPClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, mc.InitializeAll(ctx))

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				mc.GetServers()
				mc.GetAllChatCompletionTools()
				mc.GetAllServerStatuses()
				mc.IsInitialized()
				_, _ = mc.GetServerTools(srv.URL)
				_, _ = mc.GetServerForTool("echo")
				_, _ = mc.ExecuteTool(ctx, Request{
					Method: "tools/call",
					Params: map[string]any{"name": "echo", "arguments": map[string]any{}},
				}, srv.URL)
			}
		})
	}

	for range 20 {
		mc.attemptServerReconnection(ctx, srv.URL)
	}

	close(stop)
	wg.Wait()

	tools := mc.GetAllChatCompletionTools()
	require.Len(t, tools, 1)
	assert.Equal(t, "mcp_echo", tools[0].Function.Name)
}

func TestAttemptServerReconnectionSingleFlight(t *testing.T) {
	var initCount atomic.Int32
	srv := newMCPStubServer(t, 300*time.Millisecond, &initCount)

	mc := NewMCPClient([]string{srv.URL}, logger.NewNoopLogger(), newStubMCPConfig()).(*MCPClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			mc.attemptServerReconnection(ctx, srv.URL)
		})
	}
	wg.Wait()

	assert.Equal(t, int32(1), initCount.Load())
}

func TestRunWithStreamReturnsWhenConsumerAbandons(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := providersmocks.NewMockIProvider(ctrl)

	streamCh := make(chan []byte)
	done := make(chan struct{})
	defer close(done)

	go func() {
		chunk := []byte(`data: {"choices":[{"delta":{"content":"x"}}]}` + "\n")
		for {
			select {
			case streamCh <- chunk:
			case <-done:
				return
			}
		}
	}()

	provider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return((<-chan []byte)(streamCh), nil).AnyTimes()

	model := "openai/gpt-4o"
	agent := &agentImpl{
		logger:   logger.NewNoopLogger(),
		provider: provider,
		model:    &model,
	}

	ctx, cancel := context.WithCancel(context.Background())
	middlewareCh := make(chan []byte, 2)

	errCh := make(chan error, 1)
	go func() {
		errCh <- agent.RunWithStream(ctx, middlewareCh, &types.CreateChatCompletionRequest{})
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("RunWithStream did not return after the consumer stopped draining")
	}
}
