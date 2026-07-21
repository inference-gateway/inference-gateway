package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// maxRuntimeLookups bounds concurrent runtime metadata calls per request.
const maxRuntimeLookups = 4

// resolveContextWindows fills the effective context window for models served by
// local runtimes (llama.cpp, Ollama). The runtime value overrides any
// provider-published one because it reflects what the server is actually
// configured with. A failed or slow lookup leaves the model unchanged; the
// render step turns still-unresolved windows into explicit nulls.
func (router *RouterImpl) resolveContextWindows(ctx context.Context, models []types.Model) {
	byProvider := make(map[types.Provider][]int)
	for i, model := range models {
		byProvider[model.ServedBy] = append(byProvider[model.ServedBy], i)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxRuntimeLookups)
	lookup := func(fn func()) {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fn()
		}()
	}

	for providerID, indexes := range byProvider {
		switch providerID {
		case constants.LlamacppID:
			lookup(func() {
				tokens, err := router.fetchLlamacppContextWindow(ctx, providerID)
				if err != nil {
					router.logger.Debug("failed to resolve runtime context window", "provider", providerID, "error", err)
					return
				}
				for _, i := range indexes {
					models[i].ContextWindow = &types.ContextWindow{
						Tokens: tokens,
						Source: types.ContextWindowSourceRuntime,
					}
				}
			})
		case constants.OllamaID:
			for _, i := range indexes {
				lookup(func() {
					tokens, err := router.fetchOllamaContextWindow(ctx, providerID, models[i].ID)
					if err != nil {
						router.logger.Debug("failed to resolve runtime context window", "provider", providerID, "model", models[i].ID, "error", err)
						return
					}
					models[i].ContextWindow = &types.ContextWindow{
						Tokens: tokens,
						Source: types.ContextWindowSourceRuntime,
					}
				})
			}
		}
	}
	wg.Wait()
}

// fetchLlamacppContextWindow reads the serving configuration from llama.cpp's
// /props endpoint; default_generation_settings.n_ctx is the effective context
// size the server was started with (--ctx-size), which can be smaller than the
// model's theoretical maximum. Fetchers validate range and return int so call
// sites never convert untrusted 64-bit values themselves.
func (router *RouterImpl) fetchLlamacppContextWindow(ctx context.Context, providerID types.Provider) (int, error) {
	var props struct {
		DefaultGenerationSettings struct {
			NCtx int64 `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	if err := router.runtimeAPICall(ctx, providerID, http.MethodGet, "/props", nil, &props); err != nil {
		return 0, err
	}
	tokens := props.DefaultGenerationSettings.NCtx
	if tokens <= 0 || tokens > math.MaxInt {
		return 0, fmt.Errorf("llama.cpp /props returned no usable context size (%d)", tokens)
	}
	return int(tokens), nil
}

// fetchOllamaContextWindow reads per-model metadata from Ollama's show API: a
// num_ctx entry in parameters when the modelfile overrides the context size,
// falling back to the architecture's context_length in model_info.
func (router *RouterImpl) fetchOllamaContextWindow(ctx context.Context, providerID types.Provider, modelID string) (int, error) {
	name := strings.TrimPrefix(modelID, string(providerID)+"/")
	body, err := json.Marshal(map[string]string{"model": name})
	if err != nil {
		return 0, err
	}

	var show struct {
		Parameters string         `json:"parameters"`
		ModelInfo  map[string]any `json:"model_info"`
	}
	if err := router.runtimeAPICall(ctx, providerID, http.MethodPost, "/api/show", body, &show); err != nil {
		return 0, err
	}

	for line := range strings.Lines(show.Parameters) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "num_ctx" {
			if tokens, err := strconv.Atoi(fields[1]); err == nil && tokens > 0 {
				return tokens, nil
			}
		}
	}
	for key, value := range show.ModelInfo {
		if !strings.HasSuffix(key, ".context_length") {
			continue
		}
		if tokens, ok := value.(float64); ok && tokens > 0 && tokens < math.MaxInt {
			return int(tokens), nil
		}
	}
	return 0, fmt.Errorf("ollama show returned no context length for %s", name)
}

// runtimeAPICall performs a request against the provider's server root and
// decodes the JSON response into out. Runtime admin APIs like llama.cpp /props
// and Ollama /api/show live at the server root, outside the OpenAI-compatible
// path prefix (e.g. /v1) of the provider base URL.
func (router *RouterImpl) runtimeAPICall(ctx context.Context, providerID types.Provider, method, path string, body []byte, out any) error {
	provider, err := router.registry.BuildProvider(providerID, router.client)
	if err != nil {
		return err
	}

	base, err := url.Parse(provider.GetURL())
	if err != nil {
		return err
	}
	if base.Scheme == "" || base.Host == "" {
		return fmt.Errorf("provider url %q has no scheme or host", provider.GetURL())
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base.Scheme+"://"+base.Host+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := applyProviderAuth(req, provider); err != nil {
		return err
	}

	resp, err := router.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s returned status %d", method, path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
