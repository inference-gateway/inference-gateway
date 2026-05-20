package tests

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
)

// TestMCPClientTransportModes tests the transport mode functionality
func TestMCPClientTransportModes(t *testing.T) {
	cfg := config.Config{
		MCP: &config.MCPConfig{
			DialTimeout:           5 * time.Second,
			TlsHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ClientTimeout:         30 * time.Second,
			RequestTimeout:        10 * time.Second,
		},
	}

	testLogger, err := logger.NewLogger("test")
	require.NoError(t, err)

	mcpClient := mcp.NewMCPClient([]string{}, testLogger, cfg)

	t.Run("Transport mode client creation", func(t *testing.T) {
		serverURL := "http://example.com/mcp"

		client1 := mcpClient.(*mcp.MCPClient).NewClientWithTransport(serverURL, mcp.TransportModeStreamableHTTP)
		assert.NotNil(t, client1)

		client2 := mcpClient.(*mcp.MCPClient).NewClientWithTransport(serverURL, mcp.TransportModeSSE)
		assert.NotNil(t, client2)

		client3 := mcpClient.(*mcp.MCPClient).NewClientWithTransport(serverURL, mcp.TransportModeHTTP)
		assert.NotNil(t, client3)
	})
}

// TestInitializeAllWithUnreachableServersAndReconnect verifies that when all
// configured MCP servers are unreachable at startup, InitializeAll returns nil
// (instead of ErrNoClientsInitialized) as long as EnableReconnect is true. The
// background reconnection loop is expected to keep retrying so the gateway can
// continue serving requests without crash-looping.
// Regression test for: https://github.com/inference-gateway/inference-gateway/issues/304
func TestInitializeAllWithUnreachableServersAndReconnect(t *testing.T) {
	unreachableURL := reserveUnreachableURL(t)

	t.Run("EnableReconnect=true returns nil (non-fatal)", func(t *testing.T) {
		cfg := config.Config{
			MCP: &config.MCPConfig{
				DialTimeout:           100 * time.Millisecond,
				TlsHandshakeTimeout:   100 * time.Millisecond,
				ResponseHeaderTimeout: 100 * time.Millisecond,
				ExpectContinueTimeout: 100 * time.Millisecond,
				ClientTimeout:         200 * time.Millisecond,
				RequestTimeout:        500 * time.Millisecond,
				MaxRetries:            1,
				RetryInterval:         10 * time.Millisecond,
				InitialBackoff:        10 * time.Millisecond,
				EnableReconnect:       true,
				ReconnectInterval:     30 * time.Second,
			},
		}

		testLogger, err := logger.NewLogger("test")
		require.NoError(t, err)

		mcpClient := mcp.NewMCPClient([]string{unreachableURL}, testLogger, cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		initErr := mcpClient.InitializeAll(ctx)
		assert.NoError(t, initErr,
			"InitializeAll must not return a fatal error when reconnect is enabled — background reconnection takes over")
		assert.True(t, mcpClient.IsInitialized(),
			"client should report as initialized so the gateway pipeline can continue")
	})

	t.Run("StopBackgroundReconnection cancels the loop cleanly", func(t *testing.T) {
		cfg := config.Config{
			MCP: &config.MCPConfig{
				DialTimeout:           100 * time.Millisecond,
				TlsHandshakeTimeout:   100 * time.Millisecond,
				ResponseHeaderTimeout: 100 * time.Millisecond,
				ExpectContinueTimeout: 100 * time.Millisecond,
				ClientTimeout:         200 * time.Millisecond,
				RequestTimeout:        500 * time.Millisecond,
				MaxRetries:            1,
				RetryInterval:         10 * time.Millisecond,
				InitialBackoff:        10 * time.Millisecond,
				EnableReconnect:       true,
				ReconnectInterval:     1 * time.Hour,
			},
		}

		testLogger, err := logger.NewLogger("test")
		require.NoError(t, err)

		mcpClient := mcp.NewMCPClient([]string{unreachableURL}, testLogger, cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		require.NoError(t, mcpClient.InitializeAll(ctx))

		stopped := make(chan struct{})
		go func() {
			mcpClient.StopBackgroundReconnection()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("StopBackgroundReconnection blocked — reconnect goroutine is not respecting cancellation")
		}

		mcpClient.StopBackgroundReconnection()
	})

	t.Run("EnableReconnect=false still returns ErrNoClientsInitialized", func(t *testing.T) {
		cfg := config.Config{
			MCP: &config.MCPConfig{
				DialTimeout:           100 * time.Millisecond,
				TlsHandshakeTimeout:   100 * time.Millisecond,
				ResponseHeaderTimeout: 100 * time.Millisecond,
				ExpectContinueTimeout: 100 * time.Millisecond,
				ClientTimeout:         200 * time.Millisecond,
				RequestTimeout:        500 * time.Millisecond,
				MaxRetries:            1,
				RetryInterval:         10 * time.Millisecond,
				InitialBackoff:        10 * time.Millisecond,
				EnableReconnect:       false,
			},
		}

		testLogger, err := logger.NewLogger("test")
		require.NoError(t, err)

		mcpClient := mcp.NewMCPClient([]string{unreachableURL}, testLogger, cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		initErr := mcpClient.InitializeAll(ctx)
		require.Error(t, initErr,
			"InitializeAll must still surface a fatal error when reconnect is disabled")
		assert.ErrorIs(t, initErr, mcp.ErrNoClientsInitialized)
	})
}

// reserveUnreachableURL returns an http URL pointing at a port that was bound
// briefly and then released — there is a non-zero chance the OS rebinds the
// port to something else, so the test falls back to httptest.NewServer's closed
// listener pattern.
func reserveUnreachableURL(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.Listener.Addr().(*net.TCPAddr)
	srv.Close()

	return "http://" + addr.String() + "/mcp"
}

// TestSSEFallbackURLGeneration tests the SSE fallback URL generation logic
func TestSSEFallbackURLGeneration(t *testing.T) {
	tests := []struct {
		name        string
		serverURL   string
		expectedSSE string
	}{
		{
			name:        "MCP endpoint to SSE",
			serverURL:   "http://localhost:8080/mcp",
			expectedSSE: "http://localhost:8080/sse",
		},
		{
			name:        "Root path with trailing slash",
			serverURL:   "http://localhost:8080/",
			expectedSSE: "http://localhost:8080/sse",
		},
		{
			name:        "Root path without trailing slash",
			serverURL:   "http://localhost:8080",
			expectedSSE: "http://localhost:8080/sse",
		},
		{
			name:        "API endpoint",
			serverURL:   "http://localhost:8080/api/v1",
			expectedSSE: "http://localhost:8080/api/v1/sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				MCP: &config.MCPConfig{
					DialTimeout:           5 * time.Second,
					TlsHandshakeTimeout:   5 * time.Second,
					ResponseHeaderTimeout: 5 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
					ClientTimeout:         30 * time.Second,
					RequestTimeout:        10 * time.Second,
				},
			}

			testLogger, err := logger.NewLogger("test")
			require.NoError(t, err)
			mcpClient := mcp.NewMCPClient([]string{}, testLogger, cfg)

			actualSSE := mcpClient.(*mcp.MCPClient).BuildSSEFallbackURL(tt.serverURL)
			assert.Equal(t, tt.expectedSSE, actualSSE)
		})
	}
}
