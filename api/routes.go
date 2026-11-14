package api

import (
	"net/http"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	lipsync "github.com/inference-gateway/inference-gateway/internal/lipsync"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
)

// RouterImpl serves every capability of the gateway: handlers live in files
// named for their capability (chat.go, messages.go, responses.go, decisions.go,
// models.go, images.go, speech.go, sfx.go, music.go, videos.go, proxy.go,
// health.go) and the shared plumbing in upstream.go, jsonproxy.go, sse.go,
// multipart.go and respond.go.
type RouterImpl struct {
	cfg               config.Config
	logger            logger.Logger
	registry          registry.ProviderRegistry
	client            client.Client
	mcpClient         mcp.MCPClientInterface
	mcpAgent          *mcp.Agent
	telemetry         otel.OpenTelemetry
	selector          *routing.Selector
	tts               *tts.Engine
	lipsync           *lipsync.Engine
	proxyTransport    http.RoundTripper
	proxyTransportErr error
	elevenlabsVoices  elevenlabs.VoiceLibrary
}

func NewRouter(
	cfg config.Config,
	logger logger.Logger,
	providerRegistry registry.ProviderRegistry,
	httpClient client.Client,
	mcpClient mcp.MCPClientInterface,
	mcpAgent *mcp.Agent,
	telemetry otel.OpenTelemetry,
	selector *routing.Selector,
	localTTS *tts.Engine,
) *RouterImpl {
	proxyTransport, proxyTransportErr := client.NewProxyTransport(cfg.Client)

	return &RouterImpl{
		cfg:               cfg,
		logger:            logger,
		registry:          providerRegistry,
		client:            httpClient,
		mcpClient:         mcpClient,
		mcpAgent:          mcpAgent,
		telemetry:         telemetry,
		selector:          selector,
		tts:               localTTS,
		lipsync:           lipsync.NewEngine(logger),
		proxyTransport:    proxyTransport,
		proxyTransportErr: proxyTransportErr,
	}
}

func (router *RouterImpl) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	router.logger.Warn("route not found", "path", r.URL.Path, "method", r.Method)
	middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}
