package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	modelsdev "github.com/inference-gateway/inference-gateway/providers/modelsdev"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

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
