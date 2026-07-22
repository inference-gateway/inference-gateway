package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

func routingTestSetup(t *testing.T) (logger.Logger, config.Config) {
	t.Helper()
	log, err := logger.NewLogger("test")
	require.NoError(t, err)
	cfg := config.Config{
		Server:    &config.ServerConfig{ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second},
		Providers: map[types.Provider]*registry.ProviderConfig{},
	}
	return log, cfg
}

func routingSelector(t *testing.T, alias string, deps ...routing.Deployment) *routing.Selector {
	t.Helper()
	sel, err := routing.NewSelector(&routing.PoolsConfig{
		Models: map[string]routing.PoolConfig{alias: {Deployments: deps}},
	})
	require.NoError(t, err)
	return sel
}

func chatRequest(t *testing.T, model string, stream bool) *http.Request {
	t.Helper()
	body := types.CreateChatCompletionRequest{
		Model:    model,
		Messages: []types.Message{types.NewTextMessage(t, types.User, "hi")},
	}
	if stream {
		body.Stream = &stream
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(raw)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// Round-robin rotation resolves the alias to each deployment in order, dispatches
// to the resolved provider/model, and reports the selection via response headers.
func TestChatCompletionsRouting_RoundRobinRotation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log, cfg := routingTestSetup(t)

	mockClient := providersmocks.NewMockClient(ctrl)
	provA := providersmocks.NewMockIProvider(ctrl)
	provB := providersmocks.NewMockIProvider(ctrl)
	reg := providersmocks.NewMockProviderRegistry(ctrl)

	provA.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req types.CreateChatCompletionRequest) (types.CreateChatCompletionResponse, error) {
			assert.Equal(t, "model-a", req.Model)
			return types.CreateChatCompletionResponse{ID: "a", Model: req.Model}, nil
		})
	provB.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req types.CreateChatCompletionRequest) (types.CreateChatCompletionResponse, error) {
			assert.Equal(t, "model-b", req.Model)
			return types.CreateChatCompletionResponse{ID: "b", Model: req.Model}, nil
		})
	reg.EXPECT().BuildProvider(constants.OpenaiID, mockClient).Return(provA, nil)
	reg.EXPECT().BuildProvider(constants.GroqID, mockClient).Return(provB, nil)

	sel := routingSelector(t, "fast-chat",
		routing.Deployment{Provider: "openai", Model: "model-a"},
		routing.Deployment{Provider: "groq", Model: "model-b"},
	)
	router := api.NewRouter(cfg, log, reg, mockClient, nil, nil, sel)
	r := gin.New()
	r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

	want := []struct{ provider, model string }{{"openai", "model-a"}, {"groq", "model-b"}}
	for i, w := range want {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, chatRequest(t, "fast-chat", false))
		assert.Equal(t, http.StatusOK, rec.Code, "call %d", i)
		assert.Equal(t, w.provider, rec.Header().Get("X-Selected-Provider"), "call %d", i)
		assert.Equal(t, w.model, rec.Header().Get("X-Selected-Model"), "call %d", i)
	}
}

// Streaming selection is fixed before the first streamed byte: the resolved model
// reaches the provider and the selection headers coexist with the SSE headers.
func TestChatCompletionsRouting_StreamingPassthrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log, cfg := routingTestSetup(t)

	mockClient := providersmocks.NewMockClient(ctrl)
	prov := providersmocks.NewMockIProvider(ctrl)
	reg := providersmocks.NewMockProviderRegistry(ctrl)

	prov.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req types.CreateChatCompletionRequest) (<-chan []byte, error) {
			assert.Equal(t, "stream-model", req.Model)
			ch := make(chan []byte, 1)
			ch <- []byte("data: {}\n\n")
			close(ch)
			return ch, nil
		})
	reg.EXPECT().BuildProvider(constants.OpenaiID, mockClient).Return(prov, nil)

	sel := routingSelector(t, "stream-chat",
		routing.Deployment{Provider: "openai", Model: "stream-model"},
		routing.Deployment{Provider: "groq", Model: "stream-model-b"},
	)
	router := api.NewRouter(cfg, log, reg, mockClient, nil, nil, sel)
	r := gin.New()
	r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", chatRequest(t, "stream-chat", true).Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "openai", resp.Header.Get("X-Selected-Provider"))
	assert.Equal(t, "stream-model", resp.Header.Get("X-Selected-Model"))
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
}

// With routing disabled (nil selector) a bare logical name has no provider prefix,
// so behavior is identical to today: a 400 asking for an explicit provider.
func TestChatCompletionsRouting_DisabledPassthrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log, cfg := routingTestSetup(t)

	mockClient := providersmocks.NewMockClient(ctrl)
	reg := providersmocks.NewMockProviderRegistry(ctrl)

	router := api.NewRouter(cfg, log, reg, mockClient, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, chatRequest(t, "fast-chat", false))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, rec.Header().Get("X-Selected-Provider"))
}

// An explicit ?provider= wins over routing: the alias is passed through unresolved
// to the requested provider and no selection headers are emitted.
func TestChatCompletionsRouting_ExplicitProviderWins(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log, cfg := routingTestSetup(t)

	mockClient := providersmocks.NewMockClient(ctrl)
	prov := providersmocks.NewMockIProvider(ctrl)
	reg := providersmocks.NewMockProviderRegistry(ctrl)

	prov.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req types.CreateChatCompletionRequest) (types.CreateChatCompletionResponse, error) {
			assert.Equal(t, "fast-chat", req.Model)
			return types.CreateChatCompletionResponse{ID: "x", Model: req.Model}, nil
		})
	reg.EXPECT().BuildProvider(constants.GroqID, mockClient).Return(prov, nil)

	sel := routingSelector(t, "fast-chat",
		routing.Deployment{Provider: "openai", Model: "model-a"},
		routing.Deployment{Provider: "ollama", Model: "model-b"},
	)
	router := api.NewRouter(cfg, log, reg, mockClient, nil, nil, sel)
	r := gin.New()
	r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

	req := chatRequest(t, "fast-chat", false)
	req.URL.RawQuery = "provider=groq"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("X-Selected-Provider"))
}

// ALLOWED_MODELS is evaluated against the requested logical alias (the stable
// public name), not the resolved upstream model.
func TestChatCompletionsRouting_AllowedModelsFiltersAlias(t *testing.T) {
	tests := []struct {
		name       string
		allowed    string
		wantStatus int
		wantCall   bool
	}{
		{"alias allowed dispatches", "fast-chat", http.StatusOK, true},
		{"alias not allowed is forbidden", "other-model", http.StatusForbidden, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			log, cfg := routingTestSetup(t)
			cfg.AllowedModels = tt.allowed

			mockClient := providersmocks.NewMockClient(ctrl)
			prov := providersmocks.NewMockIProvider(ctrl)
			reg := providersmocks.NewMockProviderRegistry(ctrl)

			if tt.wantCall {
				prov.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).Return(
					types.CreateChatCompletionResponse{ID: "x", Model: "model-a"}, nil)
				reg.EXPECT().BuildProvider(constants.OpenaiID, mockClient).Return(prov, nil)
			}

			sel := routingSelector(t, "fast-chat",
				routing.Deployment{Provider: "openai", Model: "model-a"},
				routing.Deployment{Provider: "groq", Model: "model-b"},
			)
			router := api.NewRouter(cfg, log, reg, mockClient, nil, nil, sel)
			r := gin.New()
			r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, chatRequest(t, "fast-chat", false))
			assert.Equal(t, tt.wantStatus, rec.Code)
			if !tt.wantCall {
				assert.Empty(t, rec.Header().Get("X-Selected-Provider"))
			}
		})
	}
}
