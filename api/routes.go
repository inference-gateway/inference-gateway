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

// RouterImpl serves every gateway capability, one file per capability.
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
