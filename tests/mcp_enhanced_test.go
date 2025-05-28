package tests

import (
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
