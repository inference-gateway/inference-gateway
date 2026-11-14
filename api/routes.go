package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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

	otelapi "go.opentelemetry.io/otel"
	codes "go.opentelemetry.io/otel/codes"
	propagation "go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	lipsync "github.com/inference-gateway/inference-gateway/internal/lipsync"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
	proxy "github.com/inference-gateway/inference-gateway/internal/proxy"
	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
	systemone "github.com/inference-gateway/inference-gateway/internal/systemone"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	modelsdev "github.com/inference-gateway/inference-gateway/providers/modelsdev"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

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

// writeData writes body as the response with the given status, matching gin's
// Data rendering: Content-Type only when the response has none.
func writeData(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType != "" && len(w.Header()["Content-Type"]) == 0 {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeDataFromReader relays reader as the response body with the given status,
// length and content type, matching gin's DataFromReader rendering.
func writeDataFromReader(w http.ResponseWriter, status int, contentLength int64, contentType string, reader io.Reader) {
	if contentType != "" && len(w.Header()["Content-Type"]) == 0 {
		w.Header().Set("Content-Type", contentType)
	}
	if contentLength >= 0 && w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, reader)
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
func (router *RouterImpl) resolveProvider(r *http.Request, model, exampleModel string) (types.Provider, string, string) {
	providerID, resolved, ok := routing.ResolveProvider(r.URL.Query().Get("provider"), model)
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

func (router *RouterImpl) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	provider, msg, err := router.buildProvider(types.Provider(r.PathValue("provider")))
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if err := applyProviderAuth(r, provider); err != nil {
		middlewares.WriteJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	// Check if streaming is requested
	isStreaming := r.Header.Get("Accept") == contentTypeEventStream || r.Header.Get("Content-Type") == contentTypeEventStream

	if isStreaming {
		handleStreamingRequest(w, r, provider, router)
		return
	}

	// Non-streaming case: Setup reverse proxy
	handleProxyRequest(w, r, provider, router)
}

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, provider core.IProvider, router *RouterImpl) {
	middlewares.SetSSEHeaders(w)

	fullURL, err := constructProviderURL(provider, r.PathValue("path"), r.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}

	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	ctx := r.Context()
	upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, fullURL.String(), bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err, "method", r.Method, "url", fullURL.String())
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}

	upstreamReq.Header = r.Header.Clone()
	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		router.logger.Error("failed to make upstream request", err, "url", fullURL.String())
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	router.relaySSE(w, r, resp.Body, fullURL.String())
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
func (router *RouterImpl) relaySSE(w http.ResponseWriter, r *http.Request, upstream io.Reader, logURL string) {
	reader := bufio.NewReaderSize(upstream, sseReaderBufferSize)
	middlewares.StreamResponse(w, func(_ io.Writer) bool {
		middlewares.ResetWriteDeadline(w, router.cfg.Server.WriteTimeout)

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			router.logStreamChunk(r, line)
			if _, werr := w.Write(line); werr != nil {
				router.logger.Error("failed to write chunk", werr, "bytes", len(line))
				return false
			}
		}
		if err != nil {
			if err != io.EOF {
				router.logger.Error("failed to read stream", err, "url", logURL, "method", r.Method)
			}
			return false
		}
		return true
	})
}

// logStreamChunk emits a development-only preview of large (or sampled
// /proxy) SSE lines.
func (router *RouterImpl) logStreamChunk(r *http.Request, line []byte) {
	if router.cfg.Environment != constants.EnvironmentDevelopment {
		return
	}
	providerParam := r.PathValue("provider")
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
func (router *RouterImpl) rewriteModelOrRespond(w http.ResponseWriter, body []byte, model string) ([]byte, error) {
	out, err := rewriteModelField(body, model)
	if err != nil {
		status, msg := router.rewriteFailure(err)
		middlewares.WriteJSON(w, status, ErrorResponse{Error: msg})
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

func (f *upstreamFailure) Error() string { return f.message }

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
func (router *RouterImpl) callUpstream(r *http.Request, provider core.IProvider, req upstreamRequest) (*http.Response, func(), *upstreamFailure) {
	noop := func() {}

	ctx := r.Context()
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

	markUpstreamError(r, resp)
	return resp, done, nil
}

// forwardUpstream sends req to the provider endpoint and relays the upstream
// response verbatim - JSON with its Content-Type, or SSE via relaySSE when the
// upstream answers with text/event-stream. Non-streaming requests are bounded
// by the server read timeout. A nil return means the response was already
// written; otherwise the caller renders the failure in its own envelope.
func (router *RouterImpl) forwardUpstream(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest) *upstreamFailure {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		return failure
	}
	defer done()
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, contentTypeEventStream) {
		writeDataFromReader(w, resp.StatusCode, resp.ContentLength, contentType, resp.Body)
		return nil
	}

	middlewares.SetSSEHeaders(w)
	router.relaySSE(w, r, resp.Body, req.url(provider))
	return nil
}

// markUpstreamError flags the current span when the upstream answered with an
// error status.
func markUpstreamError(r *http.Request, resp *http.Response) {
	if resp.StatusCode < http.StatusBadRequest {
		return
	}
	span := trace.SpanFromContext(r.Context())
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

func handleProxyRequest(w http.ResponseWriter, r *http.Request, provider core.IProvider, router *RouterImpl) {
	if router.proxyTransportErr != nil {
		router.logger.Error("proxy transport unavailable", router.proxyTransportErr)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to configure outbound tls"})
		return
	}

	fullURL, err := constructProviderURL(provider, r.PathValue("path"), r.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}
	reverseProxy := &httputil.ReverseProxy{
		Transport: router.proxyTransport,
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

	reverseProxy.ServeHTTP(&middlewares.DeadlineResetWriter{ResponseWriter: w, Timeout: router.cfg.Server.WriteTimeout}, r)
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

func (router *RouterImpl) HealthcheckHandler(w http.ResponseWriter, r *http.Request) {
	router.logger.Debug("healthcheck")
	middlewares.WriteJSON(w, http.StatusOK, ResponseJSON{Message: "OK"})
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

// includesDeprecated reports whether the request asked to list models the
// models.dev table marks as deprecated, which are hidden otherwise.
func includesDeprecated(includeKeys []string) bool {
	return slices.Contains(includeKeys, string(types.ListModelsParamsIncludeDeprecated))
}

// renderModelsResponse writes the models list as JSON. When include keys are
// present it injects each requested key as an explicit null on every model
// unless already populated, keeping the requested-but-unavailable state
// distinguishable from an absent field. With no include keys the typed response
// is written unchanged so the default payload stays byte-for-byte
// OpenAI-compatible.
func (router *RouterImpl) renderModelsResponse(w http.ResponseWriter, resp types.ListModelsResponse, includeKeys []string) {
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
		middlewares.WriteJSON(w, http.StatusOK, resp)
		return
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		router.logger.Error("failed to marshal models response", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		router.logger.Error("failed to decode models response", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	if data, ok := envelope["data"].([]any); ok {
		for _, item := range data {
			model, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range includeKeys {
				if key == string(types.ListModelsParamsIncludeDeprecated) {
					continue
				}
				if _, exists := model[key]; !exists {
					model[key] = nil
				}
			}
		}
	}

	middlewares.WriteJSON(w, http.StatusOK, envelope)
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
//     fields to include (context_window, pricing, modalities). Keys are trimmed and
//     de-duplicated; an unknown key returns 400. Requested-but-unresolved keys are
//     returned as explicit null. When omitted, no metadata fields are added.
//     The `deprecated` key adds no field: it lists models the community status
//     table marks as deprecated, which are hidden by default.
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
func (router *RouterImpl) ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	includeKeys, err := parseIncludeParam(r.URL.Query().Get("include"))
	if err != nil {
		router.logger.Error("invalid include parameter", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	providerID := types.Provider(r.URL.Query().Get("provider"))
	if providerID != "" {
		provider, msg, err := router.buildProvider(providerID)
		if err != nil {
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		response, err := provider.ListModels(ctx)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				router.logger.Error("request timed out", err, "provider", provider.GetName())
				middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
				return
			}
			router.logger.Error("failed to list models", err, "provider", provider.GetName())
			middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to list models"})
			return
		}

		response.Data = routing.FilterModels(response.Data, router.cfg.AllowedModels, router.cfg.DisallowedModels, includesDeprecated(includeKeys))

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, response.Data)
		}

		router.renderModelsResponse(w, response, includeKeys)
	} else {
		var wg sync.WaitGroup
		providersCfg := router.cfg.Providers

		ch := make(chan types.ListModelsResponse, len(providersCfg))

		ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
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

		allModels = routing.FilterModels(allModels, router.cfg.AllowedModels, router.cfg.DisallowedModels, includesDeprecated(includeKeys))

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, allModels)
		}

		unifiedResponse := types.ListModelsResponse{
			Object: "list",
			Data:   allModels,
		}

		router.renderModelsResponse(w, unifiedResponse, includeKeys)
	}
}

// readBoundedBody reads the request body up to the configured max request
// body size, reporting tooLarge when the body exceeds the limit. A body of
// exactly the limit is accepted.
func (router *RouterImpl) readBoundedBody(r *http.Request) (body []byte, tooLarge bool, err error) {
	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	body, err = io.ReadAll(io.LimitReader(r.Body, int64(maxBodySize)+1))
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
func (router *RouterImpl) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	var req types.CreateChatCompletionRequest

	if mcpRequest, ok := middlewares.MCPRequestFromContext(r.Context()); ok {
		req = *mcpRequest
	} else {
		maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
		r.Body = http.MaxBytesReader(w, r.Body, int64(maxBodySize))
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				router.logger.Error("request body too large", err)
				middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
				return
			}
			router.logger.Error("failed to decode request", err)
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
	}

	model := req.Model
	originalModel := req.Model
	providerID := types.Provider(r.URL.Query().Get("provider"))

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
		providerID, model, msg = router.resolveProvider(r, model, "openai/gpt-4")
		if msg != "" {
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
			return
		}
	}
	req.Model = model

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
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
			if !modelsdev.AcceptsImages(providerID, req.Model) {
				router.logger.Info("filtering images from non-vision model request",
					"provider", providerID,
					"model", req.Model,
					"messagesWithImages", imageCount)

				for i := range req.Messages {
					if req.Messages[i].HasImageContent() {
						if err := req.Messages[i].StripImageContent(); err != nil {
							router.logger.Error("failed to strip image content from message", err)
							middlewares.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process message content"})
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
		w.Header().Set("X-Selected-Provider", routedProvider)
		w.Header().Set("X-Selected-Model", routedModel)
	}

	if req.Stream != nil && *req.Stream {
		middlewares.SetSSEHeaders(w)

		streamCtx := r.Context()
		streamCh, err := provider.StreamChatCompletions(streamCtx, req)
		if err != nil {
			router.logger.Error("failed to start streaming", err, "provider", providerID)

			writeProviderError(w, err)
			return
		}

		middlewares.StreamResponse(w, func(_ io.Writer) bool {
			select {
			case line, ok := <-streamCh:
				if !ok {
					router.logger.Debug("stream closed", "provider", providerID)
					return false
				}

				middlewares.ResetWriteDeadline(w, router.cfg.Server.WriteTimeout)

				router.logger.Debug("stream chunk",
					"provider", providerID,
					"bytes", len(line),
					"line", string(line))

				if _, err := w.Write(line); err != nil {
					router.logger.Error("failed to write chunk", err)
					return false
				}

				return true
			case <-streamCtx.Done():
				return false
			}
		})
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	response, err := provider.ChatCompletions(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", providerID)

		writeProviderError(w, err)
		return
	}

	middlewares.WriteJSON(w, http.StatusOK, response)
}

// writeProviderError relays an upstream provider failure to the client with
// the upstream status and its Retry-After header, so clients can tell a quota
// wall from a transient error. Errors without an upstream status become 400.
func writeProviderError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	var httpErr *core.HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.StatusCode
		if httpErr.RetryAfter != "" {
			w.Header().Set("Retry-After", httpErr.RetryAfter)
		}
	}
	middlewares.WriteJSON(w, status, ErrorResponse{Error: err.Error()})
}

// messagesError writes a gateway-generated error in the Anthropic error
// envelope ({"type": "error", "error": {"type": ..., "message": ...}}), which
// is what native Messages API clients expect to parse.
func messagesError(w http.ResponseWriter, status int, errType, message string) {
	resp := types.MessagesError{Type: types.MessagesErrorTypeError}
	resp.Error.Type = errType
	resp.Error.Message = message
	middlewares.WriteJSON(w, status, resp)
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
func (router *RouterImpl) MessagesHandler(w http.ResponseWriter, r *http.Request) {
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		messagesError(w, http.StatusBadRequest, "invalid_request_error", "Failed to read request")
		return
	}
	if tooLarge {
		messagesError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "Request body too large")
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		messagesError(w, http.StatusBadRequest, "invalid_request_error", "Failed to decode request")
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID, model, msg := router.resolveProvider(r, model, "anthropic/claude-sonnet-4-5")
	if msg != "" {
		messagesError(w, http.StatusBadRequest, "invalid_request_error", msg)
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		messagesError(w, http.StatusForbidden, "invalid_request_error", reason)
		return
	}

	if providerID != constants.AnthropicID {
		router.logger.Error("messages api not supported by provider", nil, "provider", providerID)
		messagesError(w, http.StatusBadRequest, "not_supported_error", "The Messages API is not supported by this provider yet.")
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		messagesError(w, http.StatusBadRequest, "invalid_request_error", msg)
		return
	}

	if model != originalModel {
		if body, err = rewriteModelField(body, model); err != nil {
			status, msg := router.rewriteFailure(err)
			errType := "invalid_request_error"
			if status == http.StatusInternalServerError {
				errType = "api_error"
			}
			messagesError(w, status, errType, msg)
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream
	if f := router.forwardUpstream(w, r, provider, upstreamRequest{
		endpointPath: "/messages",
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       acceptHeaderFor(isStreaming),
		streaming:    isStreaming,
	}); f != nil {
		messagesError(w, f.status, "api_error", f.message)
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
func (router *RouterImpl) ResponsesHandler(w http.ResponseWriter, r *http.Request) {
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID, model, msg := router.resolveProvider(r, model, "openai/gpt-4o")
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	if providerID != constants.OpenaiID {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint := provider.GetEndpoints().Responses
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
	}

	if model != originalModel {
		if body, err = router.rewriteModelOrRespond(w, body, model); err != nil {
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream
	if f := router.forwardUpstream(w, r, provider, upstreamRequest{
		endpointPath: *endpoint,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       acceptHeaderFor(isStreaming),
		streaming:    isStreaming,
	}); f != nil {
		middlewares.WriteJSON(w, f.status, ErrorResponse{Error: f.message})
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
func (router *RouterImpl) ImagesHandler(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      "Images API",
		exampleModel: "openai/gpt-image-2",
		accept:       contentTypeJSON,
		endpointOf:   func(e types.Endpoints) *string { return e.Images },
	})
}

// DecisionsHandler implements an OpenAI-compatible POST /v1/decisions
// endpoint: https://developers.openai.com/api/docs/guides/decisions
//
// The request body is forwarded byte-for-byte (only the `model` prefix is
// stripped); providers without a decisions endpoint receive a 400. Opt-in via
// DECISIONS_ENABLED while the upstream decision APIs are in preview.
func (router *RouterImpl) DecisionsHandler(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.DecisionsEnabled {
		router.logger.Error("decisions api not enabled", nil)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Decisions API is not enabled. Set DECISIONS_ENABLED=true to enable it."})
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:        "Decisions API",
		exampleModel:   "openai/gpt-6-luna",
		accept:         contentTypeJSON,
		endpointOf:     func(e types.Endpoints) *string { return e.Decisions },
		outputModality: types.ModalityDecisions,
		translators: map[types.Provider]jsonTranslator{
			constants.OllamaID:      systemOneDecisions,
			constants.OllamaCloudID: systemOneDecisions,
			constants.LlamacppID:    systemOneDecisions,
		},
		responseTranslators: map[types.Provider]jsonResponseTranslator{
			constants.OllamaID:      systemone.Response,
			constants.OllamaCloudID: systemone.Response,
			constants.LlamacppID:    systemone.Response,
		},
	})
}

// jsonTranslator rewrites an OpenAI-style JSON request into a provider's own
// request shape. endpoint is the provider's registry endpoint template and
// model the request model with the provider prefix already stripped; it returns
// the upstream path, its query string ("" for none) and the upstream body.
type jsonTranslator func(endpoint, model string, body []byte) (path, query string, out []byte, err error)

// jsonResponseTranslator rewrites a provider's successful JSON response body
// into the gateway's own shape. Error responses are relayed untouched.
type jsonResponseTranslator func(body []byte) ([]byte, error)

// upstreamResponder sends the translated request itself and writes the whole
// client response, for providers whose answer is not the response body the
// gateway relays (an asynchronous job, say).
type upstreamResponder func(router *RouterImpl, w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest)

// jsonPreparer rewrites a request body with provider calls of its own, before
// the translator runs. It serves what one upstream call cannot, such as
// cloning a voice before speaking in it.
type jsonPreparer func(r *http.Request, provider core.IProvider, body []byte) ([]byte, *upstreamFailure)

// jsonProxy describes a JSON pass-through to a provider endpoint. apiName
// appears in client-facing errors, exampleModel in routing hints, accept sets
// the upstream Accept header when non-empty, endpointOf picks the endpoint
// off the resolved provider and outputModality, when set, is the output
// modality a model must list in the models.dev table to be forwarded.
type jsonProxy struct {
	responders          map[types.Provider]upstreamResponder
	apiName             string
	exampleModel        string
	accept              string
	endpointOf          func(types.Endpoints) *string
	notSupported        string
	preparers           map[types.Provider]jsonPreparer
	translators         map[types.Provider]jsonTranslator
	responseTranslators map[types.Provider]jsonResponseTranslator
	outputModality      types.Modality
}

// modelLacksModality returns the client-facing error when the model's known
// output modalities exclude p.outputModality, and "" when the model qualifies
// or is unknown to the table: an unknown model is relayed so the upstream
// decides, exactly as before the gate.
func (p jsonProxy) modelLacksModality(providerID types.Provider, model string) string {
	if p.outputModality == "" {
		return ""
	}
	known := modelsdev.ForProvider(providerID).Lookup(model).Modalities
	if known == nil || slices.Contains(known.Output, p.outputModality) {
		return ""
	}
	return fmt.Sprintf("Model %s/%s is not supported by the %s: its output modalities are %v, not %s.", providerID, model, p.apiName, known.Output, p.outputModality)
}

// proxyJSONBody forwards an OpenAI-style JSON request byte-for-byte to the
// provider endpoint selected by p.endpointOf (only the `model` field is
// rewritten when the provider prefix is stripped) and relays the upstream
// response with its Content-Type. Providers listed in p.translators get their
// request rewritten into the provider's own shape instead.
func (router *RouterImpl) proxyJSONBody(w http.ResponseWriter, r *http.Request, p jsonProxy) {
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model *string `json:"model,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	model := ""
	if req.Model != nil {
		model = *req.Model
	}
	originalModel := model

	providerID, model, msg := router.resolveProvider(r, model, p.exampleModel)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint, msg := router.providerEndpoint(provider, p.endpointOf, p.apiName, p.notSupported)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if msg := p.modelLacksModality(providerID, model); msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if prepare := p.preparers[providerID]; prepare != nil {
		prepared, failure := prepare(r, provider, body)
		if failure != nil {
			middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
			return
		}
		body = prepared
	}

	path, query := endpoint, ""
	if translate := p.translators[providerID]; translate != nil {
		if path, query, body, err = translate(endpoint, model, body); err != nil {
			router.logger.Error("failed to translate request for provider", err, "api", p.apiName, "provider", providerID)
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
	} else if model != originalModel {
		if body, err = router.rewriteModelOrRespond(w, body, model); err != nil {
			return
		}
	}

	upstream := upstreamRequest{
		endpointPath: path,
		query:        query,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       p.accept,
	}
	if translate := p.responseTranslators[providerID]; translate != nil {
		router.forwardTranslated(w, r, provider, upstream, translate)
		return
	}
	if respond := p.responders[providerID]; respond != nil {
		respond(router, w, r, provider, upstream)
		return
	}
	if f := router.forwardUpstream(w, r, provider, upstream); f != nil {
		middlewares.WriteJSON(w, f.status, ErrorResponse{Error: f.message})
	}
}

// forwardTranslated relays req and rewrites a successful JSON response with
// translate before writing it. Upstream errors are relayed byte-for-byte so
// the provider's own message reaches the client.
func (router *RouterImpl) forwardTranslated(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest, translate jsonResponseTranslator) {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return
	}
	defer done()
	defer resp.Body.Close()

	upstream, err := io.ReadAll(resp.Body)
	if err != nil {
		router.logger.Error("failed to read upstream response", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read upstream response"})
		return
	}
	if resp.StatusCode >= http.StatusBadRequest {
		writeDataFromReader(w, resp.StatusCode, int64(len(upstream)), resp.Header.Get("Content-Type"), bytes.NewReader(upstream))
		return
	}
	out, err := translate(upstream)
	if err != nil {
		router.logger.Error("failed to translate upstream response", err, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return
	}
	writeDataFromReader(w, resp.StatusCode, int64(len(out)), contentTypeJSON, bytes.NewReader(out))
}

// systemOneDecisions rewrites a CreateDecisionRequest into the System One
// request Ollama and llama.cpp serve at /systemone.
func systemOneDecisions(endpoint, model string, body []byte) (string, string, []byte, error) {
	out, err := systemone.Request(model, body)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, "", out, nil
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
func (router *RouterImpl) SpeechHandler(w http.ResponseWriter, r *http.Request) {
	if !router.audioEnabled(w) {
		return
	}

	// The reserved local/ prefix is served by the built-in llama-tts engine
	// instead of a provider; everything else is proxied byte-for-byte as before.
	if r.URL.Query().Get("provider") == "" && router.serveLocalSpeech(w, r) {
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      "Audio API",
		exampleModel: "openai/tts-1",
		endpointOf:   func(e types.Endpoints) *string { return e.Speech },
		preparers:    map[types.Provider]jsonPreparer{constants.ElevenlabsID: router.elevenlabsSampleVoice},
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsSpeech},
	})
}

// SFXHandler implements POST /v1/audio/sfx, the gateway extension that
// generates a non-speech audio clip - a sound effect or ambience - from a text
// prompt. OpenAI has no sound-effects endpoint, so the request mirrors
// /audio/speech (JSON in, raw audio bytes out) and only providers that carry an
// Sfx endpoint (currently elevenlabs and mirelo) can serve it; the rest receive
// a 400, mirroring the schema's SFXNotSupported response.
//
// The endpoint shares the AUDIO_ENABLED toggle with /audio/speech.
func (router *RouterImpl) SFXHandler(w http.ResponseWriter, r *http.Request) {
	if !router.audioEnabled(w) {
		return
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeMultipart) {
		router.serveSFXFromVideo(w, r)
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      sfxAPIName,
		exampleModel: "elevenlabs/eleven_text_to_sound_v2",
		endpointOf:   func(e types.Endpoints) *string { return e.SFX },
		notSupported: sfxNotSupportedMessage,
		translators: map[types.Provider]jsonTranslator{
			constants.ElevenlabsID: elevenlabsSFX,
			constants.MireloID:     mireloSFX,
		},
		responders: map[types.Provider]upstreamResponder{constants.MireloID: (*RouterImpl).relayMireloAudio},
	})
}

// mireloSFX rewrites a gateway CreateSFXRequest into the Mirelo text-to-sfx
// shape.
func mireloSFX(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateSFXRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	out, err := sfx.FromText(model, req)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, "", out, nil
}

// Sound-effect constants.
const (
	audioMedia                  = "audio"
	sfxAPIName                  = "Sound effect generation"
	sfxNotSupportedMessage      = "Sound effect generation is not supported by this provider yet."
	sfxVideoNotSupportedMessage = "Video scoring is not supported by this provider yet."
	sfxVideoExampleModel        = "mirelo/sfx-2.0"
	sfxFormFieldVideo           = "video"
	contentTypeMultipart        = "multipart/form-data"
	mireloNotReadyMessage       = "The sound effect was not ready in time. Retry the request or ask for a shorter clip."
	uploadFailedMessage         = "Failed to upload the video to the provider"

	// trackArchiveMaxSize caps how much of an archive of separated audio
	// tracks is read to answer with one of them.
	trackArchiveMaxSize = 256 << 20
)

// relayMireloAudio answers a text-to-sfx request with the mix of its job.
func (router *RouterImpl) relayMireloAudio(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest) {
	router.relaySFXJob(w, r, provider, req, sfx.OutputMix)
}

// relaySFXJob turns Mirelo's asynchronous job into the raw audio bytes
// /v1/audio/sfx answers with: it creates the job, long-polls it to a final
// state and streams the wanted track of it. A create refused because the
// uploaded asset is still settling is retried within the same budget.
// shortcut: no sleep between polls, it relies on Mirelo holding ?wait= open - add a Retry-After sleep if it ever answers early.
// shortcut: a poll failure is terminal and the client re-requests - add in-budget retries only if transient poll failures show up in practice.
func (router *RouterImpl) relaySFXJob(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest, output string) {
	createPath := req.endpointPath
	plan := sfx.Plan(router.cfg.Server.ReadTimeout, router.responseHeaderTimeout())
	req.query = plan.Query
	req.accept = contentTypeJSON
	payload, err := io.ReadAll(req.body)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request body"})
		return
	}
	req.body = bytes.NewReader(payload)

	for range plan.Requests {
		router.extendWriteDeadline(w)
		job, outcome, wait := router.mireloJob(w, r, provider, req)
		switch outcome {
		case jobAnswered:
			return
		case jobAssetNotReady:
			if !pause(r, wait) {
				return
			}
			req.body = bytes.NewReader(payload)
			continue
		case jobReady:
		}
		if job.Pending() {
			req = upstreamRequest{method: http.MethodGet, endpointPath: sfx.JobPath(createPath, job.ID), query: req.query, accept: contentTypeJSON}
			continue
		}

		audioURL, contentType, err := job.Audio(output)
		if err != nil {
			router.logger.Error("mirelo job did not produce audio", err, "job_id", job.ID)
			middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
			return
		}
		router.extendWriteDeadline(w)
		router.streamDownload(w, r, audioURL, audioMedia, contentType)
		return
	}

	router.logger.Error("mirelo job still pending after the last poll", nil, "requests", plan.Requests)
	middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: mireloNotReadyMessage})
}

// responseHeaderTimeout is how long the provider client waits for response
// headers, the ceiling a long-poll must stay under. Zero when not configured.
func (router *RouterImpl) responseHeaderTimeout() time.Duration {
	if router.cfg.Client == nil {
		return 0
	}
	return router.cfg.Client.ClientResponseHeaderTimeout
}

// extendWriteDeadline gives the response one more upstream round trip plus the
// time to write it, so a job polled over several requests outlives WRITE_TIMEOUT.
func (router *RouterImpl) extendWriteDeadline(w http.ResponseWriter) {
	middlewares.ResetWriteDeadline(w, router.cfg.Server.ReadTimeout+router.cfg.Server.WriteTimeout)
}

// jobOutcome says what mireloJob did with the client response.
type jobOutcome int

const (
	jobReady jobOutcome = iota
	jobAnswered
	jobAssetNotReady
)

// mireloJob sends req to Mirelo and decodes the job it answers with. Failures
// and upstream error responses are already written to w (jobAnswered), except
// a refusal because the uploaded asset is still settling, which is reported
// with the pause Mirelo asks for so the caller can retry.
func (router *RouterImpl) mireloJob(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest) (sfx.Job, jobOutcome, time.Duration) {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return sfx.Job{}, jobAnswered, 0
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read mirelo job response", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return sfx.Job{}, jobAnswered, 0
	}
	if sfx.AssetNotReady(resp.StatusCode, body) {
		return sfx.Job{}, jobAssetNotReady, sfx.RetryAfter(resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return sfx.Job{}, jobAnswered, 0
	}

	job, err := sfx.ParseJob(body)
	if err != nil {
		router.logger.Error("failed to decode mirelo job response", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return sfx.Job{}, jobAnswered, 0
	}
	return job, jobReady, 0
}

// pause waits for d unless the client leaves first.
func pause(r *http.Request, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-r.Context().Done():
		return false
	}
}

// serveSFXFromVideo answers a multipart /v1/audio/sfx request: the uploaded
// video is scored by the provider and the wanted track is streamed back as raw
// audio. Providers without an asset-and-job flow are served by their own
// scorer.
func (router *RouterImpl) serveSFXFromVideo(w http.ResponseWriter, r *http.Request) {
	form, ok := router.parseBoundedMultipart(w, r)
	if !ok {
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	originalModel := imagesFormValue(form, imageFormFieldModel)
	providerID, model, msg := router.resolveProvider(r, originalModel, sfxVideoExampleModel)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}
	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}
	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}
	createPath, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.SFXVideo }, sfxAPIName, sfxVideoNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}
	videos := form.File[sfxFormFieldVideo]
	if len(videos) == 0 {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The '%s' file is required to score a video.", sfxFormFieldVideo)})
		return
	}
	if score, translated := sfxVideoScorers[providerID]; translated {
		score(router, w, r, provider, model, createPath, form)
		return
	}

	assetsPath, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.Assets }, sfxAPIName, sfxVideoNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	assetID, ok := router.uploadAsset(w, r, provider, assetsPath, videos[0])
	if !ok {
		return
	}
	body, err := sfx.FromVideo(model, form, assetID)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", sfxAPIName, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	router.relaySFXJob(w, r, provider, upstreamRequest{endpointPath: createPath, body: bytes.NewReader(body), contentType: contentTypeJSON}, imagesFormValue(form, sfx.FieldOutput))
}

// sfxVideoScorers score a video for the providers whose video-to-sfx API is
// not an asset upload followed by a job, in place of the default flow.
var sfxVideoScorers = map[types.Provider]func(router *RouterImpl, w http.ResponseWriter, r *http.Request, provider core.IProvider, model, endpointPath string, form *multipart.Form){
	constants.ElevenlabsID: (*RouterImpl).elevenlabsSFXFromVideo,
}

// elevenlabsSFXFromVideo scores a video through ElevenLabs' video-to-music
// endpoint, which takes the clips and answers the finished audio in one call.
// Asking for the sound effects alone answers an archive of separate tracks,
// which the music track is dropped from here.
// shortcut: the score must finish inside SERVER_READ_TIMEOUT, which the operator raises for long clips - give the call its own window if that proves too coarse.
func (router *RouterImpl) elevenlabsSFXFromVideo(w http.ResponseWriter, r *http.Request, provider core.IProvider, model, endpointPath string, form *multipart.Form) {
	music, err := elevenlabs.SFXFromVideo(model, form)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", sfxAPIName, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		pw.CloseWithError(writeVideoMusicForm(mw, music.Fields, form.File[sfxFormFieldVideo]))
	}()

	router.extendWriteDeadline(w)
	resp, done, failure := router.callUpstream(r, provider, upstreamRequest{endpointPath: endpointPath, query: music.Query, body: pr, contentType: mw.FormDataContentType()})
	if failure != nil {
		_ = pr.CloseWithError(io.ErrClosedPipe)
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return
	}
	defer done()
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
		if readErr != nil {
			router.logger.Error("failed to read the elevenlabs video-to-music response", readErr)
			middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
			return
		}
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return
	}
	if !music.SeparateTracks {
		writeDataFromReader(w, resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body)
		return
	}

	archive, err := io.ReadAll(io.LimitReader(resp.Body, trackArchiveMaxSize))
	if err != nil {
		router.logger.Error("failed to read the elevenlabs track archive", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return
	}
	track, err := elevenlabs.EffectsTrack(archive)
	if err != nil {
		router.logger.Error("failed to unpack the elevenlabs sound-effects track", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return
	}
	writeData(w, http.StatusOK, music.ContentType, track)
}

// writeVideoMusicForm writes the video-to-music upload: the request's fields,
// then every clip to score as a repeated part, announced with the container
// type the provider needs to decode it.
func writeVideoMusicForm(mw *multipart.Writer, fields map[string]string, videos []*multipart.FileHeader) error {
	for name, value := range fields {
		if err := mw.WriteField(name, value); err != nil {
			return err
		}
	}
	for _, video := range videos {
		content, contentType, file, err := openFormFile(video)
		if err != nil {
			return err
		}
		err = writeFilePart(mw, elevenlabs.VideoClipPart, video.Filename, sfx.VideoContentType(video.Filename, contentType), content)
		_ = file.Close()
		if err != nil {
			return err
		}
	}
	return mw.Close()
}

// uploadAsset stores the uploaded video with the provider and returns the
// asset id a generation can reference. The bytes go to the signed URL the
// provider's upload ticket names, without the API key.
func (router *RouterImpl) uploadAsset(w http.ResponseWriter, r *http.Request, provider core.IProvider, assetsPath string, video *multipart.FileHeader) (string, bool) {
	content, contentType, file, err := openFormFile(video)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read the uploaded video"})
		return "", false
	}
	defer file.Close()
	contentType = sfx.VideoContentType(video.Filename, contentType)

	ticket, ok := router.assetTicket(w, r, provider, assetsPath, contentType)
	if !ok {
		return "", false
	}
	target, usable := downloadURL(ticket.UploadURL)
	if !usable {
		router.logger.Error("provider returned an unusable upload url", nil, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "The provider returned an unusable upload URL."})
		return "", false
	}

	router.extendWriteDeadline(w)
	ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	body, formType, length, err := sfx.UploadBody(ticket, video.Filename, contentType, content, video.Size)
	if err != nil {
		router.logger.Error("failed to build the upload form", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), body)
	if err != nil {
		router.logger.Error("failed to create upload request", err, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return "", false
	}
	req.Header.Set("Content-Type", formType)
	req.ContentLength = length
	resp, err := router.client.Do(req)
	if err != nil {
		router.logger.Error("failed to upload the video to the provider", err, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: uploadFailedMessage})
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		router.logger.Error("the provider refused the video upload", nil, "status", resp.StatusCode, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: uploadFailedMessage})
		return "", false
	}
	return ticket.ID, true
}

// assetTicket asks the provider where to upload a file of contentType.
// Failures are already written to w, reported by a false ok.
func (router *RouterImpl) assetTicket(w http.ResponseWriter, r *http.Request, provider core.IProvider, assetsPath, contentType string) (sfx.AssetTicket, bool) {
	resp, done, failure := router.callUpstream(r, provider, upstreamRequest{endpointPath: assetsPath, body: bytes.NewReader(sfx.AssetRequest(contentType)), contentType: contentTypeJSON, accept: contentTypeJSON})
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return sfx.AssetTicket{}, false
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read mirelo upload ticket", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return sfx.AssetTicket{}, false
	}
	if resp.StatusCode >= http.StatusBadRequest {
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return sfx.AssetTicket{}, false
	}
	ticket, err := sfx.ParseAssetTicket(body)
	if err != nil {
		router.logger.Error("failed to decode mirelo upload ticket", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return sfx.AssetTicket{}, false
	}
	return ticket, true
}

// MusicHandler implements POST /v1/audio/music, the gateway extension that
// composes a music clip from a text prompt. OpenAI has no music endpoint, so
// the request mirrors /audio/sfx (JSON in, raw audio bytes out) and only
// providers that carry a Music endpoint (currently elevenlabs) can serve it;
// the rest receive a 400, mirroring the schema's MusicNotSupported response.
//
// The endpoint shares the AUDIO_ENABLED toggle with /audio/speech.
func (router *RouterImpl) MusicHandler(w http.ResponseWriter, r *http.Request) {
	if !router.audioEnabled(w) {
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      "Music generation",
		exampleModel: "elevenlabs/music_v2",
		endpointOf:   func(e types.Endpoints) *string { return e.Music },
		notSupported: "Music generation is not supported by this provider yet.",
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsMusic},
	})
}

// audioEnabled reports whether the Audio API is switched on, writing the 404
// AUDIO_ENABLED response when it is not.
func (router *RouterImpl) audioEnabled(w http.ResponseWriter) bool {
	if router.cfg.AudioEnabled {
		return true
	}
	router.logger.Error("audio api not enabled", nil)
	middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Audio API is not enabled. Set AUDIO_ENABLED=true to enable it."})
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

const (
	// voicesAPIName labels ElevenLabs' voices endpoints in logs and errors.
	voicesAPIName = "Voices API"

	// elevenlabsMaxResponseSize caps how much of an ElevenLabs account response
	// is read. A voice list carries every voice of the account.
	elevenlabsMaxResponseSize = 8 << 20
)

// elevenlabsSampleVoice has a speech request that brings a voice sample speak
// in the ElevenLabs voice cloned from it.
func (router *RouterImpl) elevenlabsSampleVoice(r *http.Request, provider core.IProvider, body []byte) ([]byte, *upstreamFailure) {
	var req types.CreateSpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode speech request", err)
		return nil, &upstreamFailure{http.StatusBadRequest, "Failed to decode request"}
	}

	account := elevenlabsAccount{router: router, request: r, provider: provider}
	req, err := router.elevenlabsVoices.InSampleVoice(r.Context(), account, req)
	if err != nil {
		router.logger.Error("failed to speak in the voice sample", err)
		return nil, elevenlabsFailure(err)
	}

	prepared, err := json.Marshal(req)
	if err != nil {
		router.logger.Error("failed to encode speech request", err)
		return nil, &upstreamFailure{http.StatusInternalServerError, "Failed to encode request"}
	}
	return prepared, nil
}

// elevenlabsFailure renders an ElevenLabs error for the caller: a failure of
// the gateway's own, a refusal with ElevenLabs' status, a generation it turned
// down or ran out of time for, or a bad gateway.
func elevenlabsFailure(err error) *upstreamFailure {
	var failure *upstreamFailure
	if errors.As(err, &failure) {
		return failure
	}
	var refusal *elevenlabs.Refusal
	if errors.As(err, &refusal) {
		return &upstreamFailure{refusal.Status, err.Error()}
	}
	var generation *elevenlabs.GenerationFailure
	if errors.As(err, &generation) && generation.Rejected() {
		return &upstreamFailure{http.StatusBadRequest, err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &upstreamFailure{http.StatusGatewayTimeout, "The provider did not finish in time."}
	}
	return &upstreamFailure{http.StatusBadGateway, err.Error()}
}

// elevenlabsAccount reaches the ElevenLabs account of the gateway's key for
// one request, with the provider's auth and the request's tracing.
type elevenlabsAccount struct {
	router   *RouterImpl
	request  *http.Request
	provider core.IProvider
}

func (a elevenlabsAccount) ListVoices(ctx context.Context) ([]byte, error) {
	return a.call(ctx, voicesAPIName, func(e types.Endpoints) *string { return e.Voices }, upstreamRequest{method: http.MethodGet})
}

func (a elevenlabsAccount) CreateVoice(ctx context.Context, contentType string, form []byte) ([]byte, error) {
	return a.call(ctx, voicesAPIName, func(e types.Endpoints) *string { return e.VoicesCreate }, upstreamRequest{body: bytes.NewReader(form), contentType: contentType})
}

func (a elevenlabsAccount) CreateImage(ctx context.Context, generation []byte) ([]byte, error) {
	return a.call(ctx, imagesAPIName, func(e types.Endpoints) *string { return e.ImagesEdits }, upstreamRequest{body: bytes.NewReader(generation), contentType: contentTypeJSON, accept: contentTypeJSON})
}

func (a elevenlabsAccount) Image(ctx context.Context, generationID string) ([]byte, error) {
	generationOf := func(e types.Endpoints) *string {
		if e.ImagesRetrieve == nil {
			return nil
		}
		endpoint := strings.ReplaceAll(*e.ImagesRetrieve, imageIDPlaceholder, url.PathEscape(generationID))
		return &endpoint
	}
	return a.call(ctx, imagesAPIName, generationOf, upstreamRequest{method: http.MethodGet, accept: contentTypeJSON})
}

func (a elevenlabsAccount) call(ctx context.Context, apiName string, endpointOf func(types.Endpoints) *string, req upstreamRequest) ([]byte, error) {
	endpoint, msg := a.router.providerEndpoint(a.provider, endpointOf, apiName, "")
	if msg != "" {
		return nil, &upstreamFailure{http.StatusBadRequest, msg}
	}
	req.endpointPath = endpoint

	resp, done, failure := a.router.callUpstream(a.request.WithContext(ctx), a.provider, req)
	if failure != nil {
		return nil, failure
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, elevenlabsMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading the elevenlabs response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, elevenlabs.NewRefusal(resp.StatusCode, body)
	}
	return body, nil
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
func (router *RouterImpl) serveLocalSpeech(w http.ResponseWriter, r *http.Request) bool {
	if router.tts == nil {
		return false
	}
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return true
	}
	if tooLarge {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return true
	}

	var probe struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return false // let the provider path report the malformed body
	}
	if !strings.HasPrefix(probe.Model, tts.ModelPrefix) {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return false
	}
	if probe.Model != tts.ReservedModelID {
		router.logger.Error("unknown local speech model", nil, "model", probe.Model)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown local speech model %q. %q is the only local speech model.", probe.Model, tts.ReservedModelID)})
		return true
	}
	if reason := router.modelDenied(probe.Model); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
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
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return true
	}
	if strings.TrimSpace(req.Input) == "" {
		router.logger.Error("local speech request missing input", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The 'input' field is required."})
		return true
	}
	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		router.logger.Error("unsupported response_format for local engine", nil, "response_format", req.ResponseFormat)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: `The local speech engine only supports response_format "wav".`})
		return true
	}
	if req.Language != "" && !slices.Contains(tts.SupportedLanguages, req.Language) {
		router.logger.Error("unsupported language for local engine", nil, "language", req.Language)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The local speech engine does not support language %q. Supported languages: %s.", req.Language, strings.Join(tts.SupportedLanguages, ", "))})
		return true
	}

	middlewares.ResetWriteDeadline(w, router.localSpeechWriteWindow())
	audio, err := router.tts.Synthesize(r.Context(), tts.Request{
		Input:          req.Input,
		ReferenceAudio: req.ReferenceAudio,
		Language:       req.Language,
	})
	if err != nil {
		var notReady *tts.NotReadyError
		switch {
		case errors.As(err, &notReady):
			router.logger.Warn("local speech assets not ready", nil, "detail", notReady.Error())
			w.Header().Set("Retry-After", strconv.Itoa(tts.RetryAfterSeconds))
			middlewares.WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: notReady.Error()})
		case errors.Is(err, context.DeadlineExceeded):
			router.logger.Error("local speech synthesis timed out", err)
			middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: "Local speech synthesis timed out"})
		default:
			router.logger.Error("local speech synthesis failed", err)
			middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to synthesize local speech"})
		}
		return true
	}

	writeData(w, http.StatusOK, "audio/wav", audio)
	return true
}

// localSpeechWriteWindow is how long a local speech response may take: the
// longest a synthesis runs, then the usual time to write the answer. Without
// it the server's write timeout closes the connection mid-synthesis.
func (router *RouterImpl) localSpeechWriteWindow() time.Duration {
	return time.Duration(router.cfg.AudioLocalTimeout)*time.Second + router.cfg.Server.WriteTimeout
}

// Multipart form field names used by the /images/edits endpoint.
const (
	imageFormFieldImage = "image"
	// imageFormFieldImageArray is the multi-image variant accepted by
	// gpt-image-1 edits.
	imageFormFieldImageArray = "image[]"
	imageFormFieldPrompt     = "prompt"
	imageFormFieldModel      = "model"

	imageFormFieldResponseFormat = "response_format"
	imageResponseFormatBase64    = "b64_json"

	// imagesAPIName labels the Images API in logs and errors.
	imagesAPIName = "Images API"

	// imageIDPlaceholder is the token registry image endpoints carry for the
	// upstream generation id.
	imageIDPlaceholder = "{generation_id}"

	// imageGenerationWindow is how long a provider that renders an image as a
	// job may take before the request gives up.
	imageGenerationWindow = 5 * time.Minute

	// imageMaxDownloadSize caps how much of a generated image is read to
	// answer with it as base64.
	imageMaxDownloadSize = 64 << 20

	// multipartMaxMemory caps how much of a multipart upload is kept in
	// memory; parts above it spill to temp files instead.
	multipartMaxMemory = 1 << 20
)

// ImagesEditsHandler implements POST /v1/images/edits (multipart/form-data).
func (router *RouterImpl) ImagesEditsHandler(w http.ResponseWriter, r *http.Request) {
	router.handleImagesMultipart(w, r)
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
func (router *RouterImpl) handleImagesMultipart(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	form, ok := router.parseBoundedMultipart(w, r)
	if !ok {
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	if len(form.File[imageFormFieldImage])+len(form.File[imageFormFieldImageArray]) == 0 {
		router.logger.Error("images request missing image file", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The 'image' file is required."})
		return
	}
	if strings.TrimSpace(imagesFormValue(form, imageFormFieldPrompt)) == "" {
		router.logger.Error("images edit request missing prompt", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The 'prompt' field is required."})
		return
	}

	model := imagesFormValue(form, imageFormFieldModel)
	originalModel := model

	providerID, model, msg := router.resolveProvider(r, model, "openai/gpt-image-2")
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint := provider.GetEndpoints().ImagesEdits
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("images api not supported by provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The Images API is not supported by this provider yet."})
		return
	}

	if edit, translated := imageEditors[providerID]; translated {
		edit(router, w, r, provider, model, form)
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

	if f := router.forwardUpstream(w, r, provider, upstreamRequest{
		endpointPath: *endpoint,
		body:         pr,
		contentType:  mw.FormDataContentType(),
		accept:       contentTypeJSON,
	}); f != nil {
		_ = pr.CloseWithError(io.ErrClosedPipe)
		middlewares.WriteJSON(w, f.status, ErrorResponse{Error: f.message})
		return
	}
}

// imageEditors serve an image edit for the providers that have no OpenAI-shaped
// Images API, in place of the multipart passthrough.
var imageEditors = map[types.Provider]func(router *RouterImpl, w http.ResponseWriter, r *http.Request, provider core.IProvider, model string, form *multipart.Form){
	constants.ElevenlabsID: (*RouterImpl).elevenlabsImageEdit,
}

// elevenlabsImageEdit serves an image edit through an ElevenLabs image
// generation and answers in the OpenAI Images shape. The request stays open
// while ElevenLabs renders, so the write deadline covers that wait.
func (router *RouterImpl) elevenlabsImageEdit(w http.ResponseWriter, r *http.Request, provider core.IProvider, model string, form *multipart.Form) {
	generation, err := elevenlabs.ImageEdit(model, form)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", imagesAPIName, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	middlewares.ResetWriteDeadline(w, imageGenerationWindow+router.cfg.Server.WriteTimeout)
	ctx, cancel := context.WithTimeout(r.Context(), imageGenerationWindow)
	defer cancel()

	account := elevenlabsAccount{router: router, request: r, provider: provider}
	generated, err := elevenlabs.GenerateImage(ctx, account, generation)
	if err != nil {
		router.logger.Error("failed to generate the image", err, "api", imagesAPIName, "provider", provider.GetName())
		failure := elevenlabsFailure(err)
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return
	}

	image := types.Image{URL: &generated.URL}
	if imagesFormValue(form, imageFormFieldResponseFormat) == imageResponseFormatBase64 {
		encoded, failure := router.base64Image(ctx, generated.URL)
		if failure != nil {
			middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
			return
		}
		image = types.Image{B64Json: &encoded}
	}
	middlewares.WriteJSON(w, http.StatusOK, types.ImagesResponse{Created: time.Now().Unix(), Data: []types.Image{image}})
}

// base64Image downloads a generated image from the provider-supplied URL and
// returns it base64-encoded. The URL is already signed, so no credential is
// attached.
func (router *RouterImpl) base64Image(ctx context.Context, imageURL string) (string, *upstreamFailure) {
	parsed, usable := downloadURL(imageURL)
	if !usable {
		router.logger.Error("provider returned an unusable image download url", nil, "url", imageURL)
		return "", &upstreamFailure{http.StatusBadGateway, "The provider returned an unusable image download URL."}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		router.logger.Error("failed to create image download request", err, "url", imageURL)
		return "", &upstreamFailure{http.StatusInternalServerError, "Failed to create upstream request"}
	}

	resp, err := router.client.Do(req)
	if err != nil {
		router.logger.Error("failed to download generated image", err, "url", imageURL)
		return "", &upstreamFailure{http.StatusBadGateway, "Failed to download the generated image"}
	}
	defer resp.Body.Close()

	image, err := io.ReadAll(io.LimitReader(resp.Body, imageMaxDownloadSize))
	if err != nil || resp.StatusCode != http.StatusOK {
		router.logger.Error("failed to read generated image", err, "url", imageURL, "status", resp.StatusCode)
		return "", &upstreamFailure{http.StatusBadGateway, "Failed to download the generated image"}
	}
	return base64.StdEncoding.EncodeToString(image), nil
}

// downloadURL parses a provider-supplied download URL. It has to be an absolute
// HTTP(S) URL, so a malformed payload cannot make the gateway fetch a file://
// or other non-HTTP address.
func downloadURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, false
	}
	return parsed, true
}

// parseBoundedMultipart parses the request's multipart/form-data body within the
// configured max request body size and returns the form. It writes the client
// response and reports false when the body is too large or malformed; the caller
// still owns removing the form's temp files.
func (router *RouterImpl) parseBoundedMultipart(w http.ResponseWriter, r *http.Request) (*multipart.Form, bool) {
	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBodySize))
	if err := r.ParseMultipartForm(multipartMaxMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
			return nil, false
		}
		router.logger.Error("failed to parse multipart form", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to parse multipart/form-data request"})
		return nil, false
	}
	return r.MultipartForm, true
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
// each uploaded file straight through (preserving an explicit Content-Type,
// sniffing a missing or octet-stream one) so the
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

// Multipart file part content types.
const (
	contentTypeOctetStream = "application/octet-stream"
	// contentSniffLen is how many leading bytes http.DetectContentType reads.
	contentSniffLen = 512
)

// copyFormFile copies one uploaded file into mw. A part without a type or
// typed application/octet-stream (what Go's multipart.CreateFormFile and many
// clients send) is labelled from its leading bytes, because providers such as
// OpenAI's /images/edits reject octet-stream uploads; an explicit type is kept.
func copyFormFile(mw *multipart.Writer, field string, fh *multipart.FileHeader) error {
	body, contentType, file, err := openFormFile(fh)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeFilePart(mw, field, fh.Filename, contentType, body)
}

// openFormFile opens an uploaded part and settles its media type, sniffing
// the leading bytes when the client sent none or octet-stream. The caller
// closes the returned file once the reader is drained.
func openFormFile(fh *multipart.FileHeader) (io.Reader, string, io.Closer, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, "", nil, err
	}

	var body io.Reader = src
	ct := fh.Header.Get("Content-Type")
	if ct == "" || ct == contentTypeOctetStream {
		head := make([]byte, contentSniffLen)
		n, err := io.ReadFull(src, head)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			_ = src.Close()
			return nil, "", nil, err
		}
		if n > 0 {
			ct = http.DetectContentType(head[:n])
		}
		body = io.MultiReader(bytes.NewReader(head[:n]), src)
	}
	return body, ct, src, nil
}

// writeFilePart adds body to mw as the file part named field.
func writeFilePart(mw *multipart.Writer, field, filename, contentType string, body io.Reader) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	dst, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, body)
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

	// videoMedia names the media kind in download errors.
	videoMedia = "video"

	// jobMaxResponseSize caps how much of a provider's asynchronous job payload
	// is read before it is decoded.
	jobMaxResponseSize = 1 << 20

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
func (router *RouterImpl) VideosHandler(w http.ResponseWriter, r *http.Request) {
	if !router.videosEnabled(w) {
		return
	}

	form, ok := router.parseBoundedMultipart(w, r)
	if !ok {
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	model := imagesFormValue(form, imageFormFieldModel)
	originalModel := model

	if strings.HasPrefix(model, tts.ModelPrefix) {
		router.serveLocalVideo(w, model, form)
		return
	}

	providerID, model, msg := router.resolveProvider(r, model, videosExampleModel)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.Videos }, videosAPIName, videosNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	body, err := videoRequestTranslators[providerID](model, form)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", videosAPIName, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	job, _, ok := router.videoJob(w, r, provider, providerID, model, upstreamRequest{
		endpointPath: endpoint,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	job.ID = string(providerID) + videoJobIDSeparator + job.ID
	middlewares.WriteJSON(w, http.StatusOK, job)
}

// RetrieveVideoHandler implements GET /v1/videos/{video_id}. The gateway keeps
// no job state, so the request is forwarded to the provider encoded in the job
// id (or named by ?provider=) and the upstream payload is mapped back onto a
// VideoJob.
func (router *RouterImpl) RetrieveVideoHandler(w http.ResponseWriter, r *http.Request) {
	if id, local := strings.CutPrefix(r.PathValue("video_id"), localVideoIDPrefix); local {
		router.serveLocalVideoJob(w, id)
		return
	}

	provider, providerID, videoID, endpoint, ok := router.resolveVideoTarget(w, r, func(e types.Endpoints) *string { return e.VideosRetrieve })
	if !ok {
		return
	}

	job, _, ok := router.videoJob(w, r, provider, providerID, "", upstreamRequest{
		method:       http.MethodGet,
		endpointPath: strings.ReplaceAll(endpoint, videoIDPlaceholder, url.PathEscape(videoID)),
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	job.ID = string(providerID) + videoJobIDSeparator + job.ID
	middlewares.WriteJSON(w, http.StatusOK, job)
}

// DownloadVideoContentHandler implements GET /v1/videos/{video_id}/content. No
// provider serves the rendered bytes from its own API - the render lands on a
// signed CDN URL - so the job is retrieved first and the video is streamed from
// the download URL it carries. A job that is still queued, in progress or failed
// has no bytes to serve and returns 404, as the schema requires.
func (router *RouterImpl) DownloadVideoContentHandler(w http.ResponseWriter, r *http.Request) {
	if id, local := strings.CutPrefix(r.PathValue("video_id"), localVideoIDPrefix); local {
		router.serveLocalVideoContent(w, r, id)
		return
	}

	provider, providerID, videoID, endpoint, ok := router.resolveVideoTarget(w, r, func(e types.Endpoints) *string { return e.VideosRetrieve })
	if !ok {
		return
	}

	job, downloadURL, ok := router.videoJob(w, r, provider, providerID, "", upstreamRequest{
		method:       http.MethodGet,
		endpointPath: strings.ReplaceAll(endpoint, videoIDPlaceholder, url.PathEscape(videoID)),
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	if job.Status != types.VideoJobStatusCompleted || downloadURL == "" {
		router.logger.Warn("rendered video not available", "video_id", videoID, "status", job.Status)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: videoNotReadyMessage})
		return
	}

	router.streamDownload(w, r, downloadURL, videoMedia, "")
}

// streamDownload relays rendered media (a "video" or "audio" file) from the
// provider-supplied, already signed download URL, so no credential is attached.
// The URL must be absolute HTTP(S) so a malformed payload cannot make the
// gateway fetch a file:// address. A non-empty contentType overrides the
// download's own on success.
func (router *RouterImpl) streamDownload(w http.ResponseWriter, r *http.Request, raw, media, contentType string) {
	parsed, usable := downloadURL(raw)
	if !usable {
		router.logger.Error("provider returned an unusable download url", nil, "media", media, "url", raw)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "The provider returned an unusable " + media + " download URL."})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		router.logger.Error("failed to create download request", err, "media", media, "url", raw)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}

	resp, err := router.client.Do(req)
	if err != nil {
		router.logger.Error("failed to download rendered media", err, "media", media, "url", raw)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to download the rendered " + media})
		return
	}
	defer resp.Body.Close()

	markUpstreamError(r, resp)
	if contentType == "" || resp.StatusCode >= http.StatusBadRequest {
		contentType = resp.Header.Get("Content-Type")
	}
	writeDataFromReader(w, resp.StatusCode, resp.ContentLength, contentType, resp.Body)
}

// resolveVideoTarget resolves the provider, upstream job id and endpoint for a
// per-job Videos request, writing the client response and reporting false on
// failure.
func (router *RouterImpl) resolveVideoTarget(w http.ResponseWriter, r *http.Request, endpointOf func(types.Endpoints) *string) (core.IProvider, types.Provider, string, string, bool) {
	if !router.videosEnabled(w) {
		return nil, "", "", "", false
	}

	providerID, videoID, msg := router.resolveVideoJobID(r, r.PathValue("video_id"))
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	endpoint, msg := router.providerEndpoint(provider, endpointOf, videosAPIName, videosNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	return provider, providerID, videoID, endpoint, true
}

// resolveVideoJobID splits the gateway's "provider:job_id" video id into the
// provider and the upstream id that must be sent back verbatim. An explicit
// ?provider= wins over the prefix, mirroring model routing; the prefix is
// stripped either way. It returns the client-facing message ("" on success) when
// neither names a provider.
func (router *RouterImpl) resolveVideoJobID(r *http.Request, videoID string) (types.Provider, string, string) {
	providerID := types.Provider(r.URL.Query().Get("provider"))

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
func (router *RouterImpl) videoJob(w http.ResponseWriter, r *http.Request, provider core.IProvider, providerID types.Provider, fallbackModel string, req upstreamRequest) (types.VideoJob, string, bool) {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return types.VideoJob{}, "", false
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read video job response", err, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return types.VideoJob{}, "", false
	}

	// Upstream errors are relayed verbatim so the provider's own explanation
	// (quota, unsupported size, unknown job) reaches the caller unaltered.
	if resp.StatusCode >= http.StatusBadRequest {
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return types.VideoJob{}, "", false
	}

	mapper := videoJobMappers[providerID]
	if mapper == nil {
		router.logger.Error("no video job mapping for provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: videosNotSupportedMessage})
		return types.VideoJob{}, "", false
	}

	job, downloadURL, err := mapper(body, fallbackModel, int(time.Now().Unix()))
	if err != nil {
		router.logger.Error("failed to map video job response", err, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return types.VideoJob{}, "", false
	}

	return job, downloadURL, true
}

// Local video constants.
const (
	// localVideoIDPrefix marks the video id of a render the built-in engine
	// holds, where a provider's id carries the provider.
	localVideoIDPrefix = "local" + videoJobIDSeparator

	videoFormFieldReference = "input_reference"
	videoFormFieldAudio     = "audio"

	videoNotReadyMessage = "The rendered video is not available yet. Poll GET /v1/videos/{video_id} until the status is completed."
)

// serveLocalVideo queues a lip-sync render on the built-in engine for a
// request that names a local/ model, and answers with its job.
func (router *RouterImpl) serveLocalVideo(w http.ResponseWriter, model string, form *multipart.Form) {
	if model != lipsync.ReservedModelID {
		router.logger.Error("unknown local video model", nil, "model", model)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown local video model %q. %q is the only local video model.", model, lipsync.ReservedModelID)})
		return
	}
	if reason := router.modelDenied(model); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	portraits, clips := form.File[videoFormFieldReference], form.File[videoFormFieldAudio]
	if len(portraits) == 0 || len(clips) == 0 {
		router.logger.Error("local video request missing its portrait or clip", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("%s lip-syncs a portrait to a clip: the '%s' image and the '%s' clip are both required.", lipsync.ReservedModelID, videoFormFieldReference, videoFormFieldAudio)})
		return
	}
	portrait, err := portraits[0].Open()
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read the uploaded portrait"})
		return
	}
	defer portrait.Close()
	clip, err := clips[0].Open()
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read the uploaded clip"})
		return
	}
	defer clip.Close()

	job, err := router.lipsync.Submit(
		lipsync.Upload{Name: portraits[0].Filename, Data: portrait},
		lipsync.Upload{Name: clips[0].Filename, Data: clip},
	)
	if err != nil {
		var notReady *lipsync.NotReadyError
		if errors.As(err, &notReady) {
			router.logger.Warn("local lip-sync engine not ready", nil, "detail", notReady.Error())
			w.Header().Set("Retry-After", strconv.Itoa(lipsync.RetryAfterSeconds))
			middlewares.WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: notReady.Error()})
			return
		}
		router.logger.Error("failed to queue the local render", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to queue the local render"})
		return
	}

	job.ID = localVideoIDPrefix + job.ID
	middlewares.WriteJSON(w, http.StatusOK, job)
}

// localVideo returns a job the built-in engine holds with the path of its
// video, writing the 404 for an id it does not know. The engine keeps its jobs
// in memory, so a restart forgets them.
func (router *RouterImpl) localVideo(w http.ResponseWriter, id string) (types.VideoJob, string, bool) {
	if !router.videosEnabled(w) {
		return types.VideoJob{}, "", false
	}
	job, clip, known := router.lipsync.Job(id)
	if !known {
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Unknown local video id. Local renders are kept in memory, so a gateway restart forgets them."})
		return types.VideoJob{}, "", false
	}
	job.ID = localVideoIDPrefix + job.ID
	return job, clip, true
}

func (router *RouterImpl) serveLocalVideoJob(w http.ResponseWriter, id string) {
	if job, _, ok := router.localVideo(w, id); ok {
		middlewares.WriteJSON(w, http.StatusOK, job)
	}
}

func (router *RouterImpl) serveLocalVideoContent(w http.ResponseWriter, r *http.Request, id string) {
	job, clip, ok := router.localVideo(w, id)
	if !ok {
		return
	}
	if job.Status != types.VideoJobStatusCompleted || clip == "" {
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: videoNotReadyMessage})
		return
	}
	http.ServeFile(w, r, clip)
}

// videosEnabled reports whether the Videos API is switched on, writing the 404
// VIDEOS_ENABLED response when it is not.
func (router *RouterImpl) videosEnabled(w http.ResponseWriter) bool {
	if router.cfg.VideosEnabled {
		return true
	}
	router.logger.Error("videos api not enabled", nil)
	middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Videos API is not enabled. Set VIDEOS_ENABLED=true to enable it."})
	return false
}
