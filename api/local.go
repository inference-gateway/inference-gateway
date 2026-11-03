package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	llamacpp "github.com/inference-gateway/inference-gateway/internal/llamacpp"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// localProxyPrefix is what core.ProviderImpl puts in front of every
	// endpoint; the local client strips it to reach llama-server directly.
	localProxyPrefix = "/proxy/" + string(constants.LocalID)
	// llama-server's OpenAI-compatible routes.
	localChatEndpoint   = "/v1/chat/completions"
	localModelsEndpoint = "/v1/models"
	modelObject         = "model"
)

// WithLocalModels wraps base so BuildProvider(constants.LocalID) yields a
// provider proxying to the runtime's llama-server children. The chat
// completions handler and the MCP agent loop then serve local/ models through
// the ordinary provider path: streaming, tool calls and SSE handling are
// reused unchanged.
func WithLocalModels(base registry.ProviderRegistry, rt *llamacpp.Runtime, log logger.Logger) registry.ProviderRegistry {
	id := constants.LocalID
	return &localRegistry{
		ProviderRegistry: base,
		provider: &core.ProviderImpl{
			ID:        &id,
			Name:      constants.LocalDisplayName,
			AuthType:  constants.AuthTypeNone,
			Endpoints: types.Endpoints{Chat: localChatEndpoint, Models: localModelsEndpoint},
			Client:    &localClient{runtime: rt, http: &http.Client{}},
			Logger:    log,
		},
	}
}

type localRegistry struct {
	registry.ProviderRegistry
	provider core.IProvider
}

func (l *localRegistry) BuildProvider(id types.Provider, c client.Client) (core.IProvider, error) {
	if id == constants.LocalID {
		return l.provider, nil
	}
	return l.ProviderRegistry.BuildProvider(id, c)
}

// localClient sends a provider request to the llama-server child serving the
// model named in its body - the only place the reference lives once the
// request is inside core.ProviderImpl. A model that is not ready becomes a
// 503 + Retry-After response, which the provider relays as an HTTPError.
type localClient struct {
	runtime *llamacpp.Runtime
	http    *http.Client
}

func (c *localClient) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
	}
	var probe struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &probe)
	ref, err := llamacpp.ParseReference(llamacpp.ModelPrefix + probe.Model)
	if err != nil {
		return textResponse(req, http.StatusBadRequest, err.Error()), nil
	}

	base, release, err := c.runtime.Acquire(req.Context(), ref)
	var notReady *llamacpp.NotReadyError
	switch {
	case errors.As(err, &notReady):
		resp := textResponse(req, http.StatusServiceUnavailable, notReady.Error())
		resp.Header.Set("Retry-After", strconv.Itoa(llamacpp.RetryAfterSeconds))
		return resp, nil
	case err != nil:
		return nil, err
	}

	target, err := url.Parse(base + strings.TrimPrefix(req.URL.Path, localProxyPrefix))
	if err != nil {
		release()
		return nil, err
	}
	req.URL = target
	req.Host = ""
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	resp, err := c.http.Do(req)
	if err != nil {
		release()
		return nil, err
	}
	resp.Body = &releaseOnClose{ReadCloser: resp.Body, release: release}
	return resp, nil
}

// releaseOnClose hands the server slot back once the response is consumed,
// so a streaming completion is never evicted mid-stream.
type releaseOnClose struct {
	io.ReadCloser
	release func()
}

func (r *releaseOnClose) Close() error {
	r.release()
	return r.ReadCloser.Close()
}

func textResponse(req *http.Request, status int, msg string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": {"text/plain; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(msg)),
		Request:    req,
	}
}

// localModels lists the cached local/ GGUFs for GET /v1/models.
func (router *RouterImpl) localModels() []types.Model {
	if router.local == nil {
		return nil
	}
	refs := router.local.Cached()
	models := make([]types.Model, 0, len(refs))
	for _, ref := range refs {
		models = append(models, types.Model{
			ID:       ref.String(),
			Object:   modelObject,
			OwnedBy:  string(constants.LocalID),
			ServedBy: constants.LocalID,
		})
	}
	return models
}
