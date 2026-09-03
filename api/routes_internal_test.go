package api

import (
	"fmt"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

// TestToMCPTool_NilDescription verifies a server tool published without a
// description does not panic and yields an empty description.
func TestToMCPTool_NilDescription(t *testing.T) {
	got := toMCPTool(mcp.Tool{Name: "no_desc"}, "http://server")
	assert.Equal(t, "mcp_no_desc", got.Name)
	assert.Equal(t, "", got.Description)
	assert.Equal(t, "http://server", got.Server)

	desc := "Reads a file"
	got = toMCPTool(mcp.Tool{Name: "read_file", Description: &desc}, "http://server")
	assert.Equal(t, desc, got.Description)
}

// TestBuildProvider_MapsRegistryErrors pins the client-facing message to the
// registry sentinel rather than to error message text.
func TestBuildProvider_MapsRegistryErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	reg := providersmocks.NewMockProviderRegistry(ctrl)
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
