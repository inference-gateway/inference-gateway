package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	providers "github.com/inference-gateway/inference-gateway/tests/mocks/providers"

	config "github.com/inference-gateway/inference-gateway/config"
	images "github.com/inference-gateway/inference-gateway/internal/images"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	speech "github.com/inference-gateway/inference-gateway/internal/speech"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// TestBuildProvider_MapsRegistryErrors pins the client-facing message to the
// registry sentinel rather than to error message text.
func TestBuildProvider_MapsRegistryErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	reg := providers.NewMockProviderRegistry(ctrl)
	router := &RouterImpl{logger: logger.NewNoopLogger(), registry: reg}

	reg.EXPECT().BuildProvider(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("wrapped: %w", registry.ErrTokenNotConfigured))
	_, msg, err := router.buildProvider("openai")
	require.Error(t, err)
	assert.Equal(t, errMsgProviderNeedsAPIKey, msg)

	reg.EXPECT().BuildProvider(gomock.Any(), gomock.Any()).Return(nil, registry.ErrProviderNotFound)
	_, msg, err = router.buildProvider("nope")
	require.Error(t, err)
	assert.Equal(t, errMsgProviderNotFound, msg)
}

// TestRewriteModelField pins the pass-through contract: only "model" changes,
// large/float numbers keep their original formatting, unknown fields survive,
// and a malformed body is a plain (non-encode) error.
func TestRewriteModelField(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o","max_tokens":12345678901234567890,"temperature":0.10,"metadata":{"nested":[1,2.50]},"extra":null}`)

	got, err := rewriteModelField(body, "gpt-4o")
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-4o","max_tokens":12345678901234567890,"temperature":0.10,"metadata":{"nested":[1,2.50]},"extra":null}`, string(got))
	assert.Contains(t, string(got), `12345678901234567890`, "big integers must not be re-encoded as floats")
	assert.Contains(t, string(got), `0.10`, "float formatting must be preserved")

	_, err = rewriteModelField([]byte(`{not json`), "gpt-4o")
	require.Error(t, err)
	assert.False(t, errors.Is(err, errEncodeRequest), "decode failures must map to 400, not 500")
}

func TestAcceptHeaderFor(t *testing.T) {
	assert.Equal(t, "text/event-stream", acceptHeaderFor(true))
	assert.Equal(t, "application/json", acceptHeaderFor(false))
}

// TestApplyProviderAuth_StripsCallerAuthorization verifies the caller's inbound
// Authorization header never leaks to the upstream provider: bearer providers
// overwrite it with their own token, and every other auth type removes it while
// applying the provider credential elsewhere.
func TestApplyProviderAuth_StripsCallerAuthorization(t *testing.T) {
	cases := []struct {
		name         string
		authType     string
		wantAuth     string
		wantAPIKey   string
		wantQueryKey string
	}{
		{"bearer overwrites caller token", constants.AuthTypeBearer, "Bearer provider-secret", "", ""},
		{"xheader strips caller token", constants.AuthTypeXheader, "", "provider-secret", ""},
		{"query strips caller token", constants.AuthTypeQuery, "", "", "provider-secret"},
		{"none strips caller token", constants.AuthTypeNone, "", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/proxy/p/v1/x", nil)
			req.Header.Set("Authorization", "Bearer caller-oidc-token")
			provider := &core.ProviderImpl{Token: "provider-secret", AuthType: tc.authType}

			require.NoError(t, applyProviderAuth(req, provider))

			assert.Equal(t, tc.wantAuth, req.Header.Get("Authorization"), "caller Authorization must not leak upstream")
			assert.Equal(t, tc.wantAPIKey, req.Header.Get("x-api-key"))
			assert.Equal(t, tc.wantQueryKey, req.URL.Query().Get("key"))
		})
	}
}

// TestElevenlabsFailure pins the status each kind of ElevenLabs error reaches
// the caller with.
func TestElevenlabsFailure(t *testing.T) {
	const refusedStatus = http.StatusPaymentRequired

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"a refusal keeps the status elevenlabs answered", speech.NewRefusal(refusedStatus, []byte("paid plan required")), refusedStatus},
		{"a moderated generation is the caller's to fix", &images.GenerationFailure{Reason: "moderated", Message: "blocked"}, http.StatusBadRequest},
		{"a generation that broke is the provider's fault", &images.GenerationFailure{Reason: "model_error", Message: "broke"}, http.StatusBadGateway},
		{"a generation that ran out of time", fmt.Errorf("waiting: %w", context.DeadlineExceeded), http.StatusGatewayTimeout},
		{"anything else is a bad gateway", errors.New("connection reset"), http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantStatus, elevenlabsFailure(tt.err).status)
		})
	}
}

// TestStripImagesForNonVisionModel pins the vision rule: images survive a
// disabled VISION_ENABLED, a vision-capable model and an unclassifiable
// model, and only a table-confirmed text-only model gets them stripped,
// with the text parts preserved.
func TestStripImagesForNonVisionModel(t *testing.T) {
	multimodalRequest := func(t *testing.T, model string) *types.CreateChatCompletionRequest {
		return &types.CreateChatCompletionRequest{
			Model: model,
			Messages: []types.Message{
				types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "What is in this image?"),
					types.NewImageContentPart(t, "https://example.com/cat.png", nil)),
			},
		}
	}
	textRequest := func(t *testing.T, model string) *types.CreateChatCompletionRequest {
		return &types.CreateChatCompletionRequest{
			Model:    model,
			Messages: []types.Message{types.NewTextMessage(t, types.User, "hi")},
		}
	}

	tests := []struct {
		name        string
		vision      bool
		providerID  types.Provider
		model       string
		request     func(t *testing.T, model string) *types.CreateChatCompletionRequest
		wantImages  bool
		wantContent string
	}{
		{"no images is a no-op", true, "openai", "gpt-4-0613", textRequest, false, "hi"},
		{"vision disabled forwards images untouched", false, "openai", "gpt-4-0613", multimodalRequest, true, ""},
		{"vision model keeps its images", true, "anthropic", "claude-haiku-4-5", multimodalRequest, true, ""},
		{"text-only model loses its images", true, "openai", "gpt-4-0613", multimodalRequest, false, "What is in this image?"},
		{"unknown model is permissive", true, "openai", "gpt-nonexistent", multimodalRequest, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := &RouterImpl{cfg: config.Config{VisionEnabled: tt.vision}, logger: logger.NewNoopLogger()}

			req := tt.request(t, tt.model)
			require.NoError(t, router.stripImagesForNonVisionModel(req, tt.providerID))

			assert.Equal(t, tt.wantImages, req.Messages[0].HasImageContent())
			text, err := req.Messages[0].Content.AsMessageContent0()
			if !tt.wantImages {
				require.NoError(t, err)
				assert.Equal(t, tt.wantContent, text)
			}
		})
	}
}
