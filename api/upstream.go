package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	otelapi "go.opentelemetry.io/otel"
	codes "go.opentelemetry.io/otel/codes"
	propagation "go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

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

const (
	// jobMaxResponseSize caps how much of a provider's asynchronous job
	// payload is read before it is decoded.
	jobMaxResponseSize = 1 << 20

	// generationIDPlaceholder is the token registry generation endpoints
	// carry for the upstream generation or job id.
	generationIDPlaceholder = "{generation_id}"
)
