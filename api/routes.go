package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	gin "github.com/gin-gonic/gin"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelapi "go.opentelemetry.io/otel"
	codes "go.opentelemetry.io/otel/codes"
	propagation "go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	proxy "github.com/inference-gateway/inference-gateway/internal/proxy"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	logger "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type RouterImpl struct {
	cfg       config.Config
	logger    logger.Logger
	registry  registry.ProviderRegistry
	client    client.Client
	mcpClient mcp.MCPClientInterface
	telemetry otel.OpenTelemetry
	selector  *routing.Selector
	tts       *tts.Engine
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ResponseJSON struct {
	Message string `json:"message"`
}

func NewRouter(
	cfg config.Config,
	logger logger.Logger,
	providerRegistry registry.ProviderRegistry,
	httpClient client.Client,
	mcpClient mcp.MCPClientInterface,
	telemetry otel.OpenTelemetry,
	selector *routing.Selector,
	localTTS *tts.Engine,
) *RouterImpl {
	return &RouterImpl{
		cfg,
		logger,
		providerRegistry,
		httpClient,
		mcpClient,
		telemetry,
		selector,
		localTTS,
	}
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Warn("route not found", "path", c.Request.URL.Path, "method", c.Request.Method)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

// Client-facing messages for provider resolution failures.
const (
	errMsgProviderNeedsAPIKey = "Provider requires an API key. Please configure the provider's API key."
	errMsgProviderNotFound    = "Provider not found. Please check the list of supported providers."
)

// buildProvider resolves a provider and, on failure, logs it and returns the
// client-facing message to send back. Callers pick the response envelope.
func (router *RouterImpl) buildProvider(providerID types.Provider) (core.IProvider, string, error) {
	provider, err := router.registry.BuildProvider(providerID, router.client)
	switch {
	case err == nil:
		return provider, "", nil
	case errors.Is(err, registry.ErrTokenNotConfigured):
		router.logger.Error("provider requires authentication but no api key was configured", err, "provider", providerID)
		return nil, errMsgProviderNeedsAPIKey, err
	default:
		router.logger.Error("provider not found or not supported", err, "provider", providerID)
		return nil, errMsgProviderNotFound, err
	}
}

// resolveProvider picks the provider from ?provider= or from the model's
// "provider/" prefix. It returns the provider, the model with the prefix
// stripped, and the client-facing message when neither is present ("" on
// success). Callers render the message in their own envelope.
func (router *RouterImpl) resolveProvider(c *gin.Context, model, exampleModel string) (types.Provider, string, string) {
	providerID, resolved, ok := routing.ResolveProvider(c.Query("provider"), model)
	if ok {
		return providerID, resolved, ""
	}

	hint := fmt.Sprintf(routing.ProviderHintFormat, exampleModel)
	if model == "" {
		router.logger.Error("no provider specified", nil)
		return "", model, "No provider specified. " + hint
	}

	router.logger.Error("unable to determine provider for model", nil, "model", model)
	return "", model, "Unable to determine provider for model. " + hint
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	provider, msg, err := router.buildProvider(types.Provider(c.Param("provider")))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if err := applyProviderAuth(c.Request, provider); err != nil {
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	// Check if streaming is requested
	isStreaming := c.Request.Header.Get("Accept") == contentTypeEventStream || c.Request.Header.Get("Content-Type") == contentTypeEventStream

	if isStreaming {
		handleStreamingRequest(c, provider, router)
		return
	}

	// Non-streaming case: Setup reverse proxy
	handleProxyRequest(c, provider, router)
}

func handleStreamingRequest(c *gin.Context, provider core.IProvider, router *RouterImpl) {
	middlewares.SetSSEHeaders(c)

	fullURL, err := constructProviderURL(provider, c.Param("path"), c.Request.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}

	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	ctx := c.Request.Context()
	upstreamReq, err := http.NewRequestWithContext(ctx, c.Request.Method, fullURL.String(), bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err, "method", c.Request.Method, "url", fullURL.String())
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}

	upstreamReq.Header = c.Request.Header.Clone()
	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		router.logger.Error("failed to make upstream request", err, "url", fullURL.String())
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	router.relaySSE(c, resp.Body, fullURL.String())
}

const (
	// sseReaderBufferSize is the bufio buffer used when relaying upstream SSE lines.
	sseReaderBufferSize = 4096
	// sseChunkLogMinBytes is the line size above which development mode logs a chunk preview.
	sseChunkLogMinBytes = 512
	// sseChunkPreviewBytes caps the logged preview length.
	sseChunkPreviewBytes = 200
	// sseChunkSampleEvery samples smaller /proxy chunks for the development preview.
	sseChunkSampleEvery = 10
)

// relaySSE streams upstream SSE lines to the client, resetting the write
// deadline per line. A partial trailing line is written before the read error
// is inspected so nothing buffered is dropped. The upstream request carries
// the client's context, so cancellation surfaces here as a read error - no
// separate ctx.Done() check is needed.
func (router *RouterImpl) relaySSE(c *gin.Context, upstream io.Reader, logURL string) {
	reader := bufio.NewReaderSize(upstream, sseReaderBufferSize)
	c.Stream(func(w io.Writer) bool {
		middlewares.ResetWriteDeadline(c, router.cfg.Server.WriteTimeout)

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			router.logStreamChunk(c, line)
			if _, werr := w.Write(line); werr != nil {
				router.logger.Error("failed to write chunk", werr, "bytes", len(line))
				return false
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				router.logger.Error("failed to read stream", err, "url", logURL, "method", c.Request.Method)
			}
			return false
		}
		return true
	})
}

// logStreamChunk emits a development-only preview of large (or sampled
// /proxy) SSE lines.
func (router *RouterImpl) logStreamChunk(c *gin.Context, line []byte) {
	if router.cfg.Environment != constants.EnvironmentDevelopment {
		return
	}
	providerParam := c.Param("provider")
	if len(line) <= sseChunkLogMinBytes && (providerParam == "" || len(line)%sseChunkSampleEvery != 0) {
		return
	}
	preview := string(bytes.TrimSpace(line))
	if len(preview) > sseChunkPreviewBytes {
		preview = preview[:sseChunkPreviewBytes] + "... (truncated)"
	}
	router.logger.Debug("stream chunk", "provider", providerParam, "bytes", len(line), "data_preview", preview)
}

// errEncodeRequest marks a rewriteModelField failure that happened while
// re-encoding (an internal error) rather than while decoding the client body.
var errEncodeRequest = errors.New("encode request")

// rewriteModelField returns body with its top-level "model" field replaced,
// preserving number formatting and leaving every other field untouched. A
// decode failure is returned as-is; an encode failure wraps errEncodeRequest.
func rewriteModelField(body []byte, model string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var payload map[string]any
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	payload["model"] = model
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errEncodeRequest, err)
	}
	return out, nil
}

// rewriteFailure logs a rewriteModelField error and maps it to the
// client-facing status and message: 500 for the internal re-encode failure,
// 400 for a client body that failed to decode. Callers pick the envelope.
func (router *RouterImpl) rewriteFailure(err error) (int, string) {
	if errors.Is(err, errEncodeRequest) {
		router.logger.Error("failed to encode request", err)
		return http.StatusInternalServerError, "Failed to encode request"
	}
	router.logger.Error("failed to decode request", err)
	return http.StatusBadRequest, "Failed to decode request"
}

// rewriteModelOrRespond wraps rewriteModelField for handlers using the plain
// ErrorResponse envelope: on failure it logs, writes the 400/500 response and
// returns the error so the caller can simply return.
func (router *RouterImpl) rewriteModelOrRespond(c *gin.Context, body []byte, model string) ([]byte, error) {
	out, err := rewriteModelField(body, model)
	if err != nil {
		status, msg := router.rewriteFailure(err)
		c.JSON(status, ErrorResponse{Error: msg})
		return nil, err
	}
	return out, nil
}

// upstreamFailure describes a forwardUpstream error the calling handler still
// has to render in its own error envelope.
type upstreamFailure struct {
	status  int
	message string
}

// upstreamRequest describes a raw pass-through request to a provider endpoint.
// An empty method means POST, an empty contentType leaves the upstream
// Content-Type header unset (for GETs with no body) and an empty accept leaves
// the upstream Accept header unset; streaming skips the server read timeout so
// an open-ended (SSE) response is not cut short.
type upstreamRequest struct {
	method       string
	endpointPath string
	query        string
	body         io.Reader
	contentType  string
	accept       string
	streaming    bool
}

// url renders the absolute upstream URL for req against the provider base URL.
func (req upstreamRequest) url(provider core.IProvider) string {
	upstreamURL := strings.TrimSuffix(provider.GetURL(), "/") + req.endpointPath
	if req.query != "" {
		upstreamURL += "?" + req.query
	}
	return upstreamURL
}

// callUpstream sends req to the provider and hands the live response back for
// the caller to read. The caller closes resp.Body and calls done when it is
// finished, which releases the read-timeout context; on failure done has
// already run.
func (router *RouterImpl) callUpstream(c *gin.Context, provider core.IProvider, req upstreamRequest) (*http.Response, func(), *upstreamFailure) {
	noop := func() {}

	ctx := c.Request.Context()
	done := noop
	if !req.streaming {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, router.cfg.Server.ReadTimeout)
		done = cancel
	}

	method := req.method
	if method == "" {
		method = http.MethodPost
	}

	upstreamURL := req.url(provider)
	upstreamReq, err := http.NewRequestWithContext(ctx, method, upstreamURL, req.body)
	if err != nil {
		done()
		router.logger.Error("failed to create upstream request", err, "url", upstreamURL)
		return nil, noop, &upstreamFailure{http.StatusInternalServerError, "Failed to create upstream request"}
	}
	if req.contentType != "" {
		upstreamReq.Header.Set("Content-Type", req.contentType)
	}
	if req.accept != "" {
		upstreamReq.Header.Set("Accept", req.accept)
	}

	if err := applyProviderAuth(upstreamReq, provider); err != nil {
		done()
		router.logger.Error("unsupported auth type", err, "provider", provider.GetName())
		return nil, noop, &upstreamFailure{http.StatusUnprocessableEntity, "Unsupported auth type"}
	}

	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		done()
		if ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "url", upstreamURL, "provider", provider.GetName())
			return nil, noop, &upstreamFailure{http.StatusGatewayTimeout, "Request timed out"}
		}
		router.logger.Error("failed to reach upstream server", err, "url", upstreamURL, "provider", provider.GetName())
		return nil, noop, &upstreamFailure{http.StatusBadGateway, "Failed to reach upstream server"}
	}

	markUpstreamError(c, resp)
	return resp, done, nil
}

// forwardUpstream sends req to the provider endpoint and relays the upstream
// response verbatim - JSON with its Content-Type, or SSE via relaySSE when the
// upstream answers with text/event-stream. Non-streaming requests are bounded
// by the server read timeout. A nil return means the response was already
// written; otherwise the caller renders the failure in its own envelope.
func (router *RouterImpl) forwardUpstream(c *gin.Context, provider core.IProvider, req upstreamRequest) *upstreamFailure {
	resp, done, failure := router.callUpstream(c, provider, req)
	if failure != nil {
		return failure
	}
	defer done()
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, contentTypeEventStream) {
		c.DataFromReader(resp.StatusCode, resp.ContentLength, contentType, resp.Body, nil)
		return nil
	}

	middlewares.SetSSEHeaders(c)
	router.relaySSE(c, resp.Body, req.url(provider))
	return nil
}

// markUpstreamError flags the current span when the upstream answered with an
// error status.
func markUpstreamError(c *gin.Context, resp *http.Response) {
	if resp.StatusCode < http.StatusBadRequest {
		return
	}
	span := trace.SpanFromContext(c.Request.Context())
	span.SetStatus(codes.Error, resp.Status)
	span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
}

// acceptHeaderFor returns the upstream Accept header for the streaming-aware
// Messages and Responses pass-through requests.
func acceptHeaderFor(streaming bool) string {
	if streaming {
		return contentTypeEventStream
	}
	return contentTypeJSON
}

func handleProxyRequest(c *gin.Context, provider core.IProvider, router *RouterImpl) {
	fullURL, err := constructProviderURL(provider, c.Param("path"), c.Request.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}
	reverseProxy := &httputil.ReverseProxy{
		Transport: proxyTransport,
	}

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		router.logger.Error("proxy request failed", err, "url", fullURL.String())
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusBadGateway)
		err = json.NewEncoder(w).Encode(ErrorResponse{
			Error: fmt.Sprintf("Failed to reach upstream server: %v", err),
		})
		if err != nil {
			router.logger.Error("failed to write error response", err)
		}
	}

	var reqModifier *proxy.DevRequestModifier
	if router.cfg.Environment == constants.EnvironmentDevelopment {
		reqModifier = proxy.NewDevRequestModifier(router.logger, &router.cfg)
		reverseProxy.ModifyResponse = proxy.NewDevResponseModifier(router.logger).Modify
	}

	reverseProxy.Rewrite = func(pr *httputil.ProxyRequest) {
		pr.SetURL(fullURL)
		pr.Out.URL.Path = fullURL.Path
		pr.Out.URL.RawQuery = fullURL.RawQuery
		pr.Out.Header = pr.In.Header.Clone()
		pr.Out.Header.Set("Content-Type", contentTypeJSON)
		pr.Out.Header.Set("Accept", contentTypeJSON)
		otelapi.GetTextMapPropagator().Inject(pr.Out.Context(), propagation.HeaderCarrier(pr.Out.Header))

		if reqModifier != nil {
			if err := reqModifier.Modify(pr.Out); err != nil {
				router.logger.Error("failed to modify request", err)
				return
			}
		}
	}

	reverseProxy.ServeHTTP(&middlewares.DeadlineResetWriter{ResponseWriter: c.Writer, Timeout: router.cfg.Server.WriteTimeout}, c.Request)
}

// applyProviderAuth sets the provider's auth credential (header or query
// param) and extra headers on req. An unrecognized auth type is returned as an
// error so misconfigured providers fail loudly instead of sending
// unauthenticated requests upstream.
//
// The caller's inbound Authorization header is always removed first so it is
// never forwarded to the upstream provider: the client authenticates to the
// gateway (and the gateway's self-proxy hop carries that token so OIDC can
// re-verify /proxy), but only the provider's own credential must leave the
// gateway. Bearer providers overwrite the header below; the others (x-api-key,
// query key, none) authenticate elsewhere, so without this removal the caller's
// bearer/OIDC token would leak to third-party providers.
func applyProviderAuth(req *http.Request, provider core.IProvider) error {
	req.Header.Del("Authorization")

	token := provider.GetToken()
	switch provider.GetAuthType() {
	case constants.AuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+token)
	case constants.AuthTypeXheader:
		header := provider.GetAuthHeader()
		if header == "" {
			header = constants.DefaultAuthHeader
		}
		req.Header.Set(header, token)
	case constants.AuthTypeQuery:
		query := req.URL.Query()
		query.Set("key", token)
		req.URL.RawQuery = query.Encode()
	case constants.AuthTypeNone:
		// Do Nothing
	default:
		return fmt.Errorf("unsupported auth type %q", provider.GetAuthType())
	}

	for key, values := range provider.GetExtraHeaders() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return nil
}

// constructProviderURL builds the provider URL consistently to avoid path duplication.
// It ensures that the path from the provider URL is handled correctly with the path parameter.
func constructProviderURL(provider core.IProvider, pathParam, rawQuery string) (*url.URL, error) {
	providerURL, err := url.Parse(provider.GetURL())
	if err != nil {
		return nil, err
	}

	url := &url.URL{
		Scheme:   providerURL.Scheme,
		Host:     providerURL.Host,
		Path:     strings.TrimSuffix(providerURL.Path, "/") + "/" + strings.TrimPrefix(pathParam, "/"),
		RawQuery: rawQuery,
	}

	return url, nil
}

func (router *RouterImpl) HealthcheckHandler(c *gin.Context) {
	router.logger.Debug("healthcheck")
	c.JSON(http.StatusOK, ResponseJSON{Message: "OK"})
}

// parseIncludeParam splits the comma-separated `include` value into a
// de-duplicated list of known metadata keys, preserving first-seen order. An
// empty value yields no keys; an unrecognized key is rejected so typos fail
// loudly instead of silently returning less data. Validity is checked against
// the generated ListModelsParamsInclude enum, keeping the accepted set in sync
// with openapi.yaml as new metadata fields are added.
func parseIncludeParam(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if !types.ListModelsParamsInclude(key).Valid() {
			return nil, fmt.Errorf("unknown include value %q", key)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys, nil
}

// renderModelsResponse writes the models list as JSON. When include keys are
// present it injects each requested key as an explicit null on every model
// unless already populated, keeping the requested-but-unavailable state
// distinguishable from an absent field. With no include keys the typed response
// is written unchanged so the default payload stays byte-for-byte
// OpenAI-compatible.
func (router *RouterImpl) renderModelsResponse(c *gin.Context, resp types.ListModelsResponse, includeKeys []string) {
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
		for i := range resp.Data {
			resp.Data[i].ContextWindow = nil
		}
	}
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludePricing)) {
		for i := range resp.Data {
			resp.Data[i].Pricing = nil
		}
	}
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludeModalities)) {
		for i := range resp.Data {
			resp.Data[i].Modalities = nil
		}
	}

	if len(includeKeys) == 0 {
		c.JSON(http.StatusOK, resp)
		return
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		router.logger.Error("failed to marshal models response", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		router.logger.Error("failed to decode models response", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	if data, ok := envelope["data"].([]any); ok {
		for _, item := range data {
			model, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range includeKeys {
				if _, exists := model[key]; !exists {
					model[key] = nil
				}
			}
		}
	}

	c.JSON(http.StatusOK, envelope)
}

// ListModelsHandler implements an OpenAI-compatible API endpoint
// that returns model information in the standard OpenAI format.
//
// This handler supports the OpenAI GET /v1/models endpoint specification:
// https://platform.openai.com/docs/api-reference/models/list
//
// Parameters:
//   - provider (query): Optional. When specified, returns models from only that provider.
//     If not specified, returns models from all configured providers.
//   - include (query): Optional. Comma-separated list of extra per-model metadata
//     fields to include (context_window, pricing). Keys are trimmed and
//     de-duplicated; an unknown key returns 400. Requested-but-unresolved keys are
//     returned as explicit null. When omitted, no metadata fields are added.
//
// Response format:
//
//	{
//	  "object": "list",
//	  "data": [
//	   {
//	      "id": "model-id",
//	      "object": "model",
//	      "created": 1686935002,
//	      "owned_by": "provider-name",
//	      "served_by": "provider-name"
//	   },
//	   ...
//	  ]
//	}
//
// This endpoint allows applications built for OpenAI's API to work seamlessly
// with the Inference Gateway's multi-provider architecture.
func (router *RouterImpl) ListModelsHandler(c *gin.Context) {
	includeKeys, err := parseIncludeParam(c.Query("include"))
	if err != nil {
		router.logger.Error("invalid include parameter", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	providerID := types.Provider(c.Query("provider"))
	if providerID != "" {
		provider, msg, err := router.buildProvider(providerID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		response, err := provider.ListModels(ctx)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				router.logger.Error("request timed out", err, "provider", provider.GetName())
				c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
				return
			}
			router.logger.Error("failed to list models", err, "provider", provider.GetName())
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to list models"})
			return
		}

		response.Data = routing.FilterModels(response.Data, router.cfg.AllowedModels, router.cfg.DisallowedModels)

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, response.Data)
		}

		router.renderModelsResponse(c, response, includeKeys)
	} else {
		var wg sync.WaitGroup
		providersCfg := router.cfg.Providers

		ch := make(chan types.ListModelsResponse, len(providersCfg))

		ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		for providerID := range providersCfg {
			wg.Add(1)
			go func(id types.Provider) {
				defer wg.Done()

				provider, err := router.registry.BuildProvider(id, router.client)
				if err != nil {
					router.logger.Error("failed to create provider", err, "provider", id)
					return
				}

				response, err := provider.ListModels(ctx)
				if err != nil {
					if ctx.Err() == context.DeadlineExceeded {
						router.logger.Error("request timed out", err, "provider", id)
						return
					}
					router.logger.Error("failed to list models", err, "provider", id)
					return
				}

				if response.Data == nil {
					response.Data = make([]types.Model, 0)
				}
				ch <- response
			}(providerID)
		}

		wg.Wait()
		close(ch)

		var allModels []types.Model
		for response := range ch {
			allModels = append(allModels, response.Data...)
		}

		if allModels == nil {
			allModels = make([]types.Model, 0)
		}

		allModels = routing.FilterModels(allModels, router.cfg.AllowedModels, router.cfg.DisallowedModels)

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, allModels)
		}

		unifiedResponse := types.ListModelsResponse{
			Object: "list",
			Data:   allModels,
		}

		router.renderModelsResponse(c, unifiedResponse, includeKeys)
	}
}

// readBoundedBody reads the request body up to the configured max request
// body size, reporting tooLarge when the body exceeds the limit. A body of
// exactly the limit is accepted.
func (router *RouterImpl) readBoundedBody(c *gin.Context) (body []byte, tooLarge bool, err error) {
	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	body, err = io.ReadAll(io.LimitReader(c.Request.Body, int64(maxBodySize)+1))
	if err != nil {
		return nil, false, err
	}
	return body, len(body) > maxBodySize, nil
}

// modelDenied reports whether model is blocked by ALLOWED_MODELS /
// DISALLOWED_MODELS, returning the client-facing reason ("" when permitted).
// ALLOWED_MODELS takes precedence: when it is set, DISALLOWED_MODELS is ignored.
func (router *RouterImpl) modelDenied(model string) string {
	if allowed := routing.ParseModelSet(router.cfg.AllowedModels); len(allowed) > 0 {
		if !routing.ModelMatches(allowed, model) {
			router.logger.Error("model not in allowed list", nil, "model", model, "allowed_models", router.cfg.AllowedModels)
			return "Model not allowed. Please check the list of allowed models."
		}
		return ""
	}
	if disallowed := routing.ParseModelSet(router.cfg.DisallowedModels); len(disallowed) > 0 && routing.ModelMatches(disallowed, model) {
		router.logger.Error("model is disallowed", nil, "model", model, "disallowed_models", router.cfg.DisallowedModels)
		return "Model is disallowed. Please use a different model."
	}
	return ""
}

// ChatCompletionsHandler implements an OpenAI-compatible API endpoint
// that generates text completions in the standard OpenAI format.
//
// Regular response format:
//
//	{
//	  "choices": [
//	    {
//	      "finish_reason": "stop",
//	      "message": {
//	        "content": "Hello, how can I help you today?",
//	        "role": "assistant"
//	      }
//	    }
//	  ],
//	  "created": 1742165657,
//	  "id": "chatcmpl-118",
//	  "model": "deepseek-r1:1.5b",
//	  "object": "chat.completion",
//	  "usage": {
//	    "completion_tokens": 139,
//	    "prompt_tokens": 10,
//	    "total_tokens": 149
//	  }
//	}
//
// Streaming response format:
//
//	{
//	  "choices": [
//	    {
//	      "index": 0,
//	      "finish_reason": "stop",
//	      "delta": {
//	        "content": "Hello",
//	        "role": "assistant"
//	      }
//	    }
//	  ],
//	  "created": 1742165657,
//	  "id": "chatcmpl-118",
//	  "model": "deepseek-r1:1.5b",
//	  "object": "chat.completion.chunk",
//	  "usage": {
//	    "completion_tokens": 139,
//	    "prompt_tokens": 10,
//	    "total_tokens": 149
//	  }
//	}
//
// It returns token completions as chat in the standard OpenAI format, allowing applications
// built for OpenAI's API to work seamlessly with the Inference Gateway's multi-provider
// architecture.
func (router *RouterImpl) ChatCompletionsHandler(c *gin.Context) {
	var req types.CreateChatCompletionRequest

	if mcpRequest, exists := c.Get(middlewares.MCPBypassHeader); exists {
		if parsedRequest, ok := mcpRequest.(*types.CreateChatCompletionRequest); ok {
			req = *parsedRequest
		} else {
			router.logger.Error("invalid mcp request type in context", nil)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
			return
		}
	} else {
		maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBodySize))
		if err := c.ShouldBindJSON(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				router.logger.Error("request body too large", err)
				c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
				return
			}
			router.logger.Error("failed to decode request", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
	}

	model := req.Model
	originalModel := req.Model
	providerID := types.Provider(c.Query("provider"))

	var routedProvider, routedModel string
	if router.selector != nil && providerID == "" {
		if dep, ok := router.selector.Select(model); ok {
			providerID = types.Provider(dep.Provider)
			model = dep.Model
			routedProvider, routedModel = dep.Provider, dep.Model
			router.logger.Debug("routed logical model", "alias", originalModel, "provider", dep.Provider, "model", dep.Model)
		}
	}

	if providerID == "" {
		var msg string
		providerID, model, msg = router.resolveProvider(c, model, "openai/gpt-4")
		if msg != "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
			return
		}
	}
	req.Model = model

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	if router.cfg.VisionEnabled {
		hasImageContent := false
		imageCount := 0
		for _, message := range req.Messages {
			if message.HasImageContent() {
				hasImageContent = true
				imageCount++
			}
		}

		if hasImageContent {
			if !core.ModelAcceptsImages(providerID, req.Model) {
				router.logger.Info("filtering images from non-vision model request",
					"provider", providerID,
					"model", req.Model,
					"messagesWithImages", imageCount)

				for i := range req.Messages {
					if req.Messages[i].HasImageContent() {
						if err := req.Messages[i].StripImageContent(); err != nil {
							router.logger.Error("failed to strip image content from message", err)
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process message content"})
							return
						}
					}
				}

				router.logger.Debug("images stripped from request, continuing with text-only content")
			}
		}
	}

	router.logger.Debug("server read timeout", "timeout", router.cfg.Server.ReadTimeout)

	if routedProvider != "" {
		c.Header("X-Selected-Provider", routedProvider)
		c.Header("X-Selected-Model", routedModel)
	}

	if req.Stream != nil && *req.Stream {
		middlewares.SetSSEHeaders(c)

		streamCtx := c.Request.Context()
		streamCh, err := provider.StreamChatCompletions(streamCtx, req)
		if err != nil {
			router.logger.Error("failed to start streaming", err, "provider", providerID)

			c.JSON(httpErrorStatus(err), ErrorResponse{Error: err.Error()})
			return
		}

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-streamCh:
				if !ok {
					router.logger.Debug("stream closed", "provider", providerID)
					return false
				}

				middlewares.ResetWriteDeadline(c, router.cfg.Server.WriteTimeout)

				router.logger.Debug("stream chunk",
					"provider", providerID,
					"bytes", len(line),
					"line", string(line))

				if _, err := w.Write(line); err != nil {
					router.logger.Error("failed to write chunk", err)
					return false
				}

				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				return true
			case <-streamCtx.Done():
				return false
			}
		})
		return
	}

	c.Header("Content-Type", contentTypeJSON)
	response, err := provider.ChatCompletions(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", providerID)

		c.JSON(httpErrorStatus(err), ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// httpErrorStatus returns the upstream status carried by a *core.HTTPError
// anywhere in err's chain, or 400 otherwise.
func httpErrorStatus(err error) int {
	var httpErr *core.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return http.StatusBadRequest
}

// messagesError writes a gateway-generated error in the Anthropic error
// envelope ({"type": "error", "error": {"type": ..., "message": ...}}), which
// is what native Messages API clients expect to parse.
func messagesError(c *gin.Context, status int, errType, message string) {
	resp := types.MessagesError{Type: types.MessagesErrorTypeError}
	resp.Error.Type = errType
	resp.Error.Message = message
	c.JSON(status, resp)
}

// MessagesHandler implements an Anthropic-compatible POST /v1/messages
// endpoint: https://docs.anthropic.com/en/api/messages
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// `cache_control` breakpoints and any future Anthropic request fields pass
// through untouched, and the upstream response - including
// `cache_creation_input_tokens` / `cache_read_input_tokens` usage and the
// Anthropic SSE event envelope when streaming - is relayed verbatim.
//
// Only providers that natively implement the Messages API are supported
// (currently Anthropic); other providers receive a 400 in the Anthropic error
// envelope, mirroring the schema's MessagesNotSupported response.
func (router *RouterImpl) MessagesHandler(c *gin.Context) {
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		messagesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request")
		return
	}
	if tooLarge {
		messagesError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "Request body too large")
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		messagesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to decode request")
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID, model, msg := router.resolveProvider(c, model, "anthropic/claude-sonnet-4-5")
	if msg != "" {
		messagesError(c, http.StatusBadRequest, "invalid_request_error", msg)
		return
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		messagesError(c, http.StatusForbidden, "invalid_request_error", reason)
		return
	}

	if providerID != constants.AnthropicID {
		router.logger.Error("messages api not supported by provider", nil, "provider", providerID)
		messagesError(c, http.StatusBadRequest, "not_supported_error", "The Messages API is not supported by this provider yet.")
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		messagesError(c, http.StatusBadRequest, "invalid_request_error", msg)
		return
	}

	if model != originalModel {
		if body, err = rewriteModelField(body, model); err != nil {
			status, msg := router.rewriteFailure(err)
			errType := "invalid_request_error"
			if status == http.StatusInternalServerError {
				errType = "api_error"
			}
			messagesError(c, status, errType, msg)
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream
	if f := router.forwardUpstream(c, provider, upstreamRequest{
		endpointPath: "/messages",
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       acceptHeaderFor(isStreaming),
		streaming:    isStreaming,
	}); f != nil {
		messagesError(c, f.status, "api_error", f.message)
	}
}

// ResponsesHandler implements an OpenAI-compatible POST /v1/responses
// endpoint: https://platform.openai.com/docs/api-reference/responses
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Responses API fields pass through untouched, and the upstream response
// - including ResponseStreamEvent frames when streaming - is relayed verbatim.
//
// Only providers that natively implement the Responses API are supported
// (currently OpenAI); other providers receive a 400, mirroring the schema's
// ResponsesNotSupported response.
func (router *RouterImpl) ResponsesHandler(c *gin.Context) {
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID, model, msg := router.resolveProvider(c, model, "openai/gpt-4o")
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	if providerID != constants.OpenaiID {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint := provider.GetEndpoints().Responses
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
	}

	if model != originalModel {
		if body, err = router.rewriteModelOrRespond(c, body, model); err != nil {
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream
	if f := router.forwardUpstream(c, provider, upstreamRequest{
		endpointPath: *endpoint,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       acceptHeaderFor(isStreaming),
		streaming:    isStreaming,
	}); f != nil {
		c.JSON(f.status, ErrorResponse{Error: f.message})
	}
}

// ImagesHandler implements an OpenAI-compatible POST /v1/images/generations
// endpoint: https://platform.openai.com/docs/api-reference/images/create
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Images API fields pass through untouched.
//
// Only providers that natively implement the Images API are supported
// (currently OpenAI); other providers receive a 400, mirroring the schema's
// ImagesNotSupported response.
//
// The endpoint is opt-in via IMAGES_ENABLED (default off). When disabled, the
// handler returns 404.
func (router *RouterImpl) ImagesHandler(c *gin.Context) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	router.proxyJSONBody(c, jsonProxy{
		apiName:      "Images API",
		exampleModel: "openai/gpt-image-2",
		accept:       contentTypeJSON,
		endpointOf:   func(e types.Endpoints) *string { return e.Images },
	})
}

// jsonTranslator rewrites an OpenAI-style JSON request into a provider's own
// request shape. endpoint is the provider's registry endpoint template and
// model the request model with the provider prefix already stripped; it returns
// the upstream path, its query string ("" for none) and the upstream body.
type jsonTranslator func(endpoint, model string, body []byte) (path, query string, out []byte, err error)

// jsonProxy describes a JSON pass-through to a provider endpoint. apiName
// appears in client-facing errors, exampleModel in routing hints, accept sets
// the upstream Accept header when non-empty and endpointOf picks the endpoint
// off the resolved provider.
type jsonProxy struct {
	apiName      string
	exampleModel string
	accept       string
	endpointOf   func(types.Endpoints) *string
	notSupported string
	translators  map[types.Provider]jsonTranslator
}

// proxyJSONBody forwards an OpenAI-style JSON request byte-for-byte to the
// provider endpoint selected by p.endpointOf (only the `model` field is
// rewritten when the provider prefix is stripped) and relays the upstream
// response with its Content-Type. Providers listed in p.translators get their
// request rewritten into the provider's own shape instead.
func (router *RouterImpl) proxyJSONBody(c *gin.Context, p jsonProxy) {
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model *string `json:"model,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	model := ""
	if req.Model != nil {
		model = *req.Model
	}
	originalModel := model

	providerID, model, msg := router.resolveProvider(c, model, p.exampleModel)
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint, msg := router.providerEndpoint(provider, p.endpointOf, p.apiName, p.notSupported)
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	path, query := endpoint, ""
	if translate := p.translators[providerID]; translate != nil {
		if path, query, body, err = translate(endpoint, model, body); err != nil {
			router.logger.Error("failed to translate request for provider", err, "api", p.apiName, "provider", providerID)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
	} else if model != originalModel {
		if body, err = router.rewriteModelOrRespond(c, body, model); err != nil {
			return
		}
	}

	if f := router.forwardUpstream(c, provider, upstreamRequest{
		endpointPath: path,
		query:        query,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       p.accept,
	}); f != nil {
		c.JSON(f.status, ErrorResponse{Error: f.message})
	}
}

// providerEndpoint resolves an optional endpoint off the provider's registry
// entry, returning the client-facing message ("" on success) when the provider
// does not implement it. notSupported overrides the default message.
func (router *RouterImpl) providerEndpoint(provider core.IProvider, endpointOf func(types.Endpoints) *string, apiName, notSupported string) (string, string) {
	endpoint := endpointOf(provider.GetEndpoints())
	if endpoint != nil && *endpoint != "" {
		return *endpoint, ""
	}

	router.logger.Error("api not supported by provider", nil, "api", apiName, "provider", provider.GetName())
	if notSupported != "" {
		return "", notSupported
	}
	return "", "The " + apiName + " is not supported by this provider yet."
}

// SpeechHandler implements an OpenAI-compatible POST /v1/audio/speech
// endpoint: https://platform.openai.com/docs/api-reference/audio/createSpeech
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Audio API fields pass through untouched. The synthesized audio comes
// back as raw binary and is streamed to the caller with the upstream's
// Content-Type (e.g. audio/mpeg for response_format mp3).
//
// ElevenLabs is the one provider whose audio API is not OpenAI-compatible, so
// its speech and sound-effect requests are rewritten instead of proxied
// byte-for-byte.
//
// Only providers that natively implement the Audio API are supported
// (currently openai and elevenlabs); other providers receive a 400, mirroring
// the schema's SpeechNotSupported response. The llamacpp registry entry also
// carries a Speech endpoint, but that path is a work in progress and not
// supported yet.
//
// The endpoint is opt-in via AUDIO_ENABLED (default off). When disabled, the
// handler returns 404.
func (router *RouterImpl) SpeechHandler(c *gin.Context) {
	if !router.audioEnabled(c) {
		return
	}

	// The reserved local/ prefix is served by the built-in llama-tts engine
	// instead of a provider; everything else is proxied byte-for-byte as before.
	if c.Query("provider") == "" && router.serveLocalSpeech(c) {
		return
	}

	router.proxyJSONBody(c, jsonProxy{
		apiName:      "Audio API",
		exampleModel: "openai/tts-1",
		endpointOf:   func(e types.Endpoints) *string { return e.Speech },
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsSpeech},
	})
}

// SFXHandler implements POST /v1/audio/sfx, the gateway extension that
// generates a non-speech audio clip - a sound effect or ambience - from a text
// prompt. OpenAI has no sound-effects endpoint, so the request mirrors
// /audio/speech (JSON in, raw audio bytes out) and only providers that carry an
// Sfx endpoint (currently elevenlabs) can serve it; the rest receive a 400,
// mirroring the schema's SFXNotSupported response.
//
// The endpoint shares the AUDIO_ENABLED toggle with /audio/speech.
func (router *RouterImpl) SFXHandler(c *gin.Context) {
	if !router.audioEnabled(c) {
		return
	}

	router.proxyJSONBody(c, jsonProxy{
		apiName:      "Sound effect generation",
		exampleModel: "elevenlabs/eleven_text_to_sound_v2",
		endpointOf:   func(e types.Endpoints) *string { return e.SFX },
		notSupported: "Sound effect generation is not supported by this provider yet.",
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsSFX},
	})
}

// MusicHandler implements POST /v1/audio/music, the gateway extension that
// composes a music clip from a text prompt. OpenAI has no music endpoint, so
// the request mirrors /audio/sfx (JSON in, raw audio bytes out) and only
// providers that carry a Music endpoint (currently elevenlabs) can serve it;
// the rest receive a 400, mirroring the schema's MusicNotSupported response.
//
// The endpoint shares the AUDIO_ENABLED toggle with /audio/speech.
func (router *RouterImpl) MusicHandler(c *gin.Context) {
	if !router.audioEnabled(c) {
		return
	}

	router.proxyJSONBody(c, jsonProxy{
		apiName:      "Music generation",
		exampleModel: "elevenlabs/music_v2",
		endpointOf:   func(e types.Endpoints) *string { return e.Music },
		notSupported: "Music generation is not supported by this provider yet.",
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsMusic},
	})
}

// audioEnabled reports whether the Audio API is switched on, writing the 404
// AUDIO_ENABLED response when it is not.
func (router *RouterImpl) audioEnabled(c *gin.Context) bool {
	if router.cfg.AudioEnabled {
		return true
	}
	router.logger.Error("audio api not enabled", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Audio API is not enabled. Set AUDIO_ENABLED=true to enable it."})
	return false
}

// elevenlabsSpeech rewrites an OpenAI CreateSpeechRequest into the ElevenLabs
// text-to-speech shape, which carries the voice in the URL path and the audio
// container in an output_format query parameter.
func elevenlabsSpeech(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateSpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	return elevenlabs.Speech(endpoint, model, req)
}

// elevenlabsSFX rewrites a gateway CreateSFXRequest into the ElevenLabs
// sound-generation shape.
func elevenlabsSFX(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateSFXRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	out, err := elevenlabs.SFX(model, req)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, "", out, nil
}

// elevenlabsMusic rewrites a gateway CreateMusicRequest into the ElevenLabs
// music shape, whose audio container rides in an output_format query parameter.
func elevenlabsMusic(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateMusicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	query, out, err := elevenlabs.Music(model, req)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, query, out, nil
}

// serveLocalSpeech handles the reserved local/ model prefix with the built-in
// llama-tts engine and reports whether the request was handled. The body is
// read once and rewound so the provider proxy path proceeds unchanged.
func (router *RouterImpl) serveLocalSpeech(c *gin.Context) bool {
	if router.tts == nil {
		return false
	}
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return true
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return true
	}

	var probe struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		return false // let the provider path report the malformed body
	}
	if !strings.HasPrefix(probe.Model, tts.ModelPrefix) {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		return false
	}
	if probe.Model != tts.ReservedModelID {
		router.logger.Error("unknown local speech model", nil, "model", probe.Model)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown local speech model %q. %q is the only local speech model.", probe.Model, tts.ReservedModelID)})
		return true
	}
	if reason := router.modelDenied(probe.Model); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return true
	}

	var req struct {
		Input          string `json:"input"`
		ResponseFormat string `json:"response_format"`
		ReferenceAudio []byte `json:"reference_audio"`
		Language       string `json:"language"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return true
	}
	if strings.TrimSpace(req.Input) == "" {
		router.logger.Error("local speech request missing input", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The 'input' field is required."})
		return true
	}
	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		router.logger.Error("unsupported response_format for local engine", nil, "response_format", req.ResponseFormat)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: `The local speech engine only supports response_format "wav".`})
		return true
	}
	if req.Language != "" && !slices.Contains(tts.SupportedLanguages, req.Language) {
		router.logger.Error("unsupported language for local engine", nil, "language", req.Language)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The local speech engine does not support language %q. Supported languages: %s.", req.Language, strings.Join(tts.SupportedLanguages, ", "))})
		return true
	}

	audio, err := router.tts.Synthesize(c.Request.Context(), tts.Request{
		Input:          req.Input,
		ReferenceAudio: req.ReferenceAudio,
		Language:       req.Language,
	})
	if err != nil {
		var notReady *tts.NotReadyError
		switch {
		case errors.As(err, &notReady):
			router.logger.Warn("local speech assets not ready", nil, "detail", notReady.Error())
			c.Header("Retry-After", strconv.Itoa(tts.RetryAfterSeconds))
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: notReady.Error()})
		case errors.Is(err, context.DeadlineExceeded):
			router.logger.Error("local speech synthesis timed out", err)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Local speech synthesis timed out"})
		default:
			router.logger.Error("local speech synthesis failed", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to synthesize local speech"})
		}
		return true
	}

	c.Data(http.StatusOK, "audio/wav", audio)
	return true
}

// Multipart form field names shared by the /images/edits and
// /images/variations endpoints.
const (
	imageFormFieldImage = "image"
	// imageFormFieldImageArray is the multi-image variant accepted by
	// gpt-image-1 edits.
	imageFormFieldImageArray = "image[]"
	imageFormFieldPrompt     = "prompt"
	imageFormFieldModel      = "model"

	// multipartMaxMemory caps how much of a multipart upload is kept in
	// memory; parts above it spill to temp files instead.
	multipartMaxMemory = 1 << 20
)

// imagesMultipartTarget selects which multipart Images endpoint a request is
// forwarded to and whether a prompt is required (edits require one, variations
// do not).
type imagesMultipartTarget struct {
	endpoint      func(types.Endpoints) *string
	requirePrompt bool
}

// proxyTransport wraps http.DefaultTransport with OpenTelemetry instrumentation
// so that every non-streaming reverse proxy call emits a distinct client span.
var proxyTransport = otelhttp.NewTransport(http.DefaultTransport, client.SpanNameFormatter())

var (
	imagesEditsTarget = imagesMultipartTarget{
		endpoint:      func(e types.Endpoints) *string { return e.ImagesEdits },
		requirePrompt: true,
	}
	imagesVariationsTarget = imagesMultipartTarget{
		endpoint:      func(e types.Endpoints) *string { return e.ImagesVariations },
		requirePrompt: false,
	}
)

// ImagesEditsHandler implements POST /v1/images/edits (multipart/form-data).
func (router *RouterImpl) ImagesEditsHandler(c *gin.Context) {
	router.handleImagesMultipart(c, imagesEditsTarget)
}

// ImagesVariationsHandler implements POST /v1/images/variations
// (multipart/form-data).
func (router *RouterImpl) ImagesVariationsHandler(c *gin.Context) {
	router.handleImagesMultipart(c, imagesVariationsTarget)
}

// handleImagesMultipart proxies a multipart Images upload to the resolved
// provider. It parses the form to validate required fields and resolve the
// provider (via ?provider= or the model prefix), then re-encodes the parts and
// streams them to the upstream through an io.Pipe so the binary image/mask
// files are streamed to the upstream without a second full in-memory copy
// (ParseMultipartForm has already spilled anything over 1 MiB to temp files).
//
// Behaviour mirrors ImagesHandler: opt-in via IMAGES_ENABLED (404 when off) and
// only providers that natively implement the endpoint are supported (others
// receive a 400).
func (router *RouterImpl) handleImagesMultipart(c *gin.Context, target imagesMultipartTarget) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	form, ok := router.parseBoundedMultipart(c)
	if !ok {
		return
	}
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()

	if len(form.File[imageFormFieldImage])+len(form.File[imageFormFieldImageArray]) == 0 {
		router.logger.Error("images request missing image file", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The 'image' file is required."})
		return
	}
	if target.requirePrompt && strings.TrimSpace(imagesFormValue(form, imageFormFieldPrompt)) == "" {
		router.logger.Error("images edit request missing prompt", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The 'prompt' field is required."})
		return
	}

	model := imagesFormValue(form, imageFormFieldModel)
	originalModel := model

	providerID, model, msg := router.resolveProvider(c, model, "openai/gpt-image-2")
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint := target.endpoint(provider.GetEndpoints())
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("images api not supported by provider", nil, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The Images API is not supported by this provider yet."})
		return
	}

	if model != originalModel {
		form.Value[imageFormFieldModel] = []string{model}
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		pw.CloseWithError(writeMultipartForm(mw, form))
	}()

	if f := router.forwardUpstream(c, provider, upstreamRequest{
		endpointPath: *endpoint,
		body:         pr,
		contentType:  mw.FormDataContentType(),
		accept:       contentTypeJSON,
	}); f != nil {
		_ = pr.CloseWithError(io.ErrClosedPipe)
		c.JSON(f.status, ErrorResponse{Error: f.message})
		return
	}
}

// parseBoundedMultipart parses the request's multipart/form-data body within the
// configured max request body size and returns the form. It writes the client
// response and reports false when the body is too large or malformed; the caller
// still owns removing the form's temp files.
func (router *RouterImpl) parseBoundedMultipart(c *gin.Context) (*multipart.Form, bool) {
	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBodySize))
	if err := c.Request.ParseMultipartForm(multipartMaxMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
			return nil, false
		}
		router.logger.Error("failed to parse multipart form", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to parse multipart/form-data request"})
		return nil, false
	}
	return c.Request.MultipartForm, true
}

// imagesFormValue returns the first value for key in a parsed multipart form,
// or "" when absent.
func imagesFormValue(form *multipart.Form, key string) string {
	if vs := form.Value[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// writeMultipartForm re-encodes a parsed multipart form onto mw, copying
// each uploaded file straight through (preserving its Content-Type) so the
// payload streams to the upstream without a second full in-memory copy. It
// runs in its own goroutine writing to the io.Pipe. Shared by the Images and
// Videos multipart endpoints.
func writeMultipartForm(mw *multipart.Writer, form *multipart.Form) error {
	for field, values := range form.Value {
		for _, v := range values {
			if err := mw.WriteField(field, v); err != nil {
				return err
			}
		}
	}
	for field, headers := range form.File {
		for _, fh := range headers {
			if err := copyFormFile(mw, field, fh); err != nil {
				return err
			}
		}
	}
	return mw.Close()
}

func copyFormFile(mw *multipart.Writer, field string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, fh.Filename))
	if ct := fh.Header.Get("Content-Type"); ct != "" {
		h.Set("Content-Type", ct)
	}
	dst, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}

// Videos API constants.
const (
	// videosExampleModel is the routing hint used in client-facing Videos errors.
	videosExampleModel = "elevenlabs/creatify-aurora"

	// videosNotSupportedMessage mirrors the schema's VideosNotSupported response.
	videosNotSupportedMessage = "The Videos API is not supported by this provider yet."

	// videosAPIName labels the Videos API in logs and errors.
	videosAPIName = "Videos API"

	// videoJobMaxResponseSize caps how much of a provider's job payload is read
	// before mapping it onto a VideoJob.
	videoJobMaxResponseSize = 1 << 20

	// videoIDPlaceholder is the token registry video endpoints carry for the
	// upstream job id.
	videoIDPlaceholder = "{generation_id}"

	// videoJobIDSeparator joins the provider to the upstream job id in the video
	// id handed back to clients. Models use "provider/model", but a video id has
	// to survive as a single path segment of GET /v1/videos/{video_id}, so this
	// one cannot be a slash.
	videoJobIDSeparator = ":"

	// videoJobIDHint explains how to route a per-job Videos request.
	videoJobIDHint = "Please specify a provider using the ?provider= query parameter or pass the video id returned by POST /v1/videos (e.g., elevenlabs" +
		videoJobIDSeparator + "gen_abc123)."
)

// videoJobMapper maps a provider's job payload onto the schema's VideoJob and
// returns the URL the rendered video can be downloaded from, empty while the
// render is unfinished. createdAt is the fallback timestamp for payloads that do
// not carry one and fallbackModel the model to report when it is not echoed back.
type videoJobMapper func(raw []byte, fallbackModel string, createdAt int) (types.VideoJob, string, error)

// videoJobMappers holds the per-provider response mappings. Every provider that
// carries a Videos endpoint in the registry needs an entry here; there is no
// OpenAI-shaped passthrough because no provider answers with a schema-shaped
// VideoJob yet.
var videoJobMappers = map[types.Provider]videoJobMapper{
	constants.ElevenlabsID: elevenlabs.Job,
}

// videoRequestTranslators rewrite the OpenAI Videos multipart form into each
// provider's native create-job payload. No provider speaks the OpenAI shape,
// so every provider that carries a Videos endpoint must have an entry here.
var videoRequestTranslators = map[types.Provider]func(model string, form *multipart.Form) ([]byte, error){
	constants.ElevenlabsID: elevenlabs.Video,
}

// VideosHandler implements POST /v1/videos, the OpenAI-compatible Videos API:
// https://platform.openai.com/docs/api-reference/videos/create
//
// Video generation is asynchronous at every provider, so the multipart request
// (prompt plus optional reference image and driving audio) is forwarded to the
// provider and the created job is returned immediately. Clients poll
// GET /v1/videos/{video_id} until the status is completed, then download the
// bytes from GET /v1/videos/{video_id}/content.
//
// The returned job id is prefixed with the provider that created it
// ("elevenlabs:abc123", see videoJobIDSeparator), because the gateway keeps no
// job state and has to route the follow-up calls somewhere.
//
// Only providers that carry a Videos endpoint (currently elevenlabs) can serve
// the request; the rest receive a 400, mirroring the schema's VideosNotSupported
// response. The endpoint is opt-in via VIDEOS_ENABLED (default off).
func (router *RouterImpl) VideosHandler(c *gin.Context) {
	if !router.videosEnabled(c) {
		return
	}

	form, ok := router.parseBoundedMultipart(c)
	if !ok {
		return
	}
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()

	model := imagesFormValue(form, imageFormFieldModel)
	originalModel := model

	providerID, model, msg := router.resolveProvider(c, model, videosExampleModel)
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.Videos }, videosAPIName, videosNotSupportedMessage)
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	body, err := videoRequestTranslators[providerID](model, form)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", videosAPIName, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	job, _, ok := router.videoJob(c, provider, providerID, model, upstreamRequest{
		endpointPath: endpoint,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	job.ID = string(providerID) + videoJobIDSeparator + job.ID
	c.JSON(http.StatusOK, job)
}

// RetrieveVideoHandler implements GET /v1/videos/{video_id}. The gateway keeps
// no job state, so the request is forwarded to the provider encoded in the job
// id (or named by ?provider=) and the upstream payload is mapped back onto a
// VideoJob.
func (router *RouterImpl) RetrieveVideoHandler(c *gin.Context) {
	provider, providerID, videoID, endpoint, ok := router.resolveVideoTarget(c, func(e types.Endpoints) *string { return e.VideosRetrieve })
	if !ok {
		return
	}

	job, _, ok := router.videoJob(c, provider, providerID, "", upstreamRequest{
		method:       http.MethodGet,
		endpointPath: strings.ReplaceAll(endpoint, videoIDPlaceholder, url.PathEscape(videoID)),
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	job.ID = string(providerID) + videoJobIDSeparator + job.ID
	c.JSON(http.StatusOK, job)
}

// DownloadVideoContentHandler implements GET /v1/videos/{video_id}/content. No
// provider serves the rendered bytes from its own API - the render lands on a
// signed CDN URL - so the job is retrieved first and the video is streamed from
// the download URL it carries. A job that is still queued, in progress or failed
// has no bytes to serve and returns 404, as the schema requires.
func (router *RouterImpl) DownloadVideoContentHandler(c *gin.Context) {
	provider, providerID, videoID, endpoint, ok := router.resolveVideoTarget(c, func(e types.Endpoints) *string { return e.VideosRetrieve })
	if !ok {
		return
	}

	job, downloadURL, ok := router.videoJob(c, provider, providerID, "", upstreamRequest{
		method:       http.MethodGet,
		endpointPath: strings.ReplaceAll(endpoint, videoIDPlaceholder, url.PathEscape(videoID)),
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	if job.Status != types.VideoJobStatusCompleted || downloadURL == "" {
		router.logger.Warn("rendered video not available", "video_id", videoID, "status", job.Status)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "The rendered video is not available yet. Poll GET /v1/videos/{video_id} until the status is completed."})
		return
	}

	router.streamRenderedVideo(c, downloadURL)
}

// streamRenderedVideo relays the rendered video from the provider-supplied
// download URL. The URL comes from the provider's own authenticated response and
// is already signed, so no gateway or provider credential is attached; it is
// still required to be an absolute HTTP(S) URL so a malformed payload cannot
// make the gateway fetch a file:// or other non-HTTP address.
func (router *RouterImpl) streamRenderedVideo(c *gin.Context, downloadURL string) {
	parsed, err := url.Parse(downloadURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		router.logger.Error("provider returned an unusable video download url", err, "url", downloadURL)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "The provider returned an unusable video download URL."})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		router.logger.Error("failed to create video download request", err, "url", downloadURL)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}

	resp, err := router.client.Do(req)
	if err != nil {
		router.logger.Error("failed to download rendered video", err, "url", downloadURL)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to download the rendered video"})
		return
	}
	defer resp.Body.Close()

	markUpstreamError(c, resp)
	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// resolveVideoTarget resolves the provider, upstream job id and endpoint for a
// per-job Videos request, writing the client response and reporting false on
// failure.
func (router *RouterImpl) resolveVideoTarget(c *gin.Context, endpointOf func(types.Endpoints) *string) (core.IProvider, types.Provider, string, string, bool) {
	if !router.videosEnabled(c) {
		return nil, "", "", "", false
	}

	providerID, videoID, msg := router.resolveVideoJobID(c, c.Param("video_id"))
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	endpoint, msg := router.providerEndpoint(provider, endpointOf, videosAPIName, videosNotSupportedMessage)
	if msg != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	return provider, providerID, videoID, endpoint, true
}

// resolveVideoJobID splits the gateway's "provider:job_id" video id into the
// provider and the upstream id that must be sent back verbatim. An explicit
// ?provider= wins over the prefix, mirroring model routing; the prefix is
// stripped either way. It returns the client-facing message ("" on success) when
// neither names a provider.
func (router *RouterImpl) resolveVideoJobID(c *gin.Context, videoID string) (types.Provider, string, string) {
	providerID := types.Provider(c.Query("provider"))

	if prefix, rest, found := strings.Cut(videoID, videoJobIDSeparator); found {
		if detected := types.Provider(strings.ToLower(prefix)); registry.Registry[detected] != nil {
			videoID = rest
			if providerID == "" {
				providerID = detected
			}
		}
	}

	if providerID == "" {
		router.logger.Error("unable to determine provider for video job", nil, "video_id", videoID)
		return "", "", "Unable to determine provider for this video id. " + videoJobIDHint
	}
	if videoID == "" {
		router.logger.Error("video job request carries no id", nil)
		return "", "", "The video id is required."
	}

	return providerID, videoID, ""
}

// videoJob sends req to the provider and maps the payload onto a VideoJob,
// returning it alongside the URL its rendered video can be downloaded from
// (empty until the render completes). Failures and upstream error responses are
// already written to c, reported by a false ok.
func (router *RouterImpl) videoJob(c *gin.Context, provider core.IProvider, providerID types.Provider, fallbackModel string, req upstreamRequest) (types.VideoJob, string, bool) {
	resp, done, failure := router.callUpstream(c, provider, req)
	if failure != nil {
		c.JSON(failure.status, ErrorResponse{Error: failure.message})
		return types.VideoJob{}, "", false
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, videoJobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read video job response", err, "provider", providerID)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return types.VideoJob{}, "", false
	}

	// Upstream errors are relayed verbatim so the provider's own explanation
	// (quota, unsupported size, unknown job) reaches the caller unaltered.
	if resp.StatusCode >= http.StatusBadRequest {
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return types.VideoJob{}, "", false
	}

	mapper := videoJobMappers[providerID]
	if mapper == nil {
		router.logger.Error("no video job mapping for provider", nil, "provider", providerID)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: videosNotSupportedMessage})
		return types.VideoJob{}, "", false
	}

	job, downloadURL, err := mapper(body, fallbackModel, int(time.Now().Unix()))
	if err != nil {
		router.logger.Error("failed to map video job response", err, "provider", providerID)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return types.VideoJob{}, "", false
	}

	return job, downloadURL, true
}

// videosEnabled reports whether the Videos API is switched on, writing the 404
// VIDEOS_ENABLED response when it is not.
func (router *RouterImpl) videosEnabled(c *gin.Context) bool {
	if router.cfg.VideosEnabled {
		return true
	}
	router.logger.Error("videos api not enabled", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Videos API is not enabled. Set VIDEOS_ENABLED=true to enable it."})
	return false
}

// ListToolsHandler implements an endpoint that returns available MCP tools
// when EXPOSE_MCP environment variable is enabled.
//
// Response format when MCP is exposed:
//
//	{
//	  "object": "list",
//	  "data": [
//	    {
//	      "name": "read_file",
//	      "description": "Read the contents of a file",
//	      "server": "filesystem-server",
//	      "input_schema": {...}
//	    },
//	    ...
//	  ]
//	}
//
// Response when MCP is not exposed:
//
//	{
//	  "error": "MCP tools endpoint is not exposed"
//	}
func (router *RouterImpl) ListToolsHandler(c *gin.Context) {
	if !router.cfg.MCP.Expose {
		router.logger.Error("mcp tools endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "mcp tools endpoint is not exposed"})
		return
	}

	var allTools []types.MCPTool

	switch {
	case router.mcpClient == nil:
		router.logger.Debug("mcp client is nil, returning empty tools list")
		allTools = make([]types.MCPTool, 0)
	case !router.mcpClient.IsInitialized():
		router.logger.Info("mcp client not initialized, no tools available")
		allTools = make([]types.MCPTool, 0)
	default:
		servers := router.mcpClient.GetServers()

		for _, serverURL := range servers {
			tools, err := router.mcpClient.GetServerTools(serverURL)
			if err != nil {
				router.logger.Error("failed to get tools from mcp server", err, "server", serverURL)
				continue
			}

			for _, tool := range tools {
				allTools = append(allTools, toMCPTool(tool, serverURL))
			}
		}

		if allTools == nil {
			allTools = make([]types.MCPTool, 0)
		}
	}

	response := types.ListToolsResponse{
		Object: "list",
		Data:   allTools,
	}

	c.JSON(http.StatusOK, response)
}

// toMCPTool converts a server tool to the gateway's list entry. Description is
// optional in the MCP schema, so a nil one becomes an empty string.
func toMCPTool(tool mcp.Tool, serverURL string) types.MCPTool {
	var description string
	if tool.Description != nil {
		description = *tool.Description
	}
	return types.MCPTool{
		Name:        mcp.ToolNamePrefix + tool.Name,
		Description: description,
		Server:      serverURL,
		InputSchema: &tool.InputSchema,
	}
}
