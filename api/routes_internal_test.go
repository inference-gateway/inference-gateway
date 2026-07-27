package api

import (
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
)

// TestResolveMaxRequestBodySize verifies the configured limit is honored and the
// default kicks in for unset/non-positive values (e.g. a config built without env parsing).
func TestResolveMaxRequestBodySize(t *testing.T) {
	assert.Equal(t, defaultMaxRequestBodySize, resolveMaxRequestBodySize(0), "unset falls back to default")
	assert.Equal(t, defaultMaxRequestBodySize, resolveMaxRequestBodySize(-1), "negative falls back to default")
	assert.Equal(t, 1234, resolveMaxRequestBodySize(1234), "configured value is honored")
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
