package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"

	otelapi "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	proxy "github.com/inference-gateway/inference-gateway/internal/proxy"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

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
