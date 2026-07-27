package api

import (
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
)

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

// TestRedactSecret verifies the provider credential (e.g. a query-auth API key
// carried in the URL) is masked before it reaches logs or client responses, and
// that an empty token leaves text untouched.
func TestRedactSecret(t *testing.T) {
	const secret = "AIzaSy-provider-secret"
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + secret

	got := redactSecret(url, secret)
	assert.NotContains(t, got, secret, "provider API key must not appear in logged text")
	assert.Contains(t, got, "REDACTED")

	assert.Equal(t, url, redactSecret(url, ""), "empty token must be a no-op")
}
