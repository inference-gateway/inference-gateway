package middleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"
	jose "github.com/go-jose/go-jose/v4"
	jwt "github.com/go-jose/go-jose/v4/jwt"

	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	testClientID    = "inference-gateway-client"
	testAPIAudience = "https://api.example.com"
	testSubject     = "user-1"
	testKeyID       = "test-key"
	testTokenTTL    = time.Hour
	testRSABits     = 2048
	testRoute       = "/v1/models"

	wwwAuthenticate  = "WWW-Authenticate"
	challengeMissing = `Bearer realm="inference-gateway"`
	challengeInvalid = `Bearer realm="inference-gateway", error="invalid_token"`

	testResourceURL         = "https://gateway.example.com/mcp"
	testRequestResource     = "http://example.com" + middlewares.MCPPath
	metadataFromResourceURL = `, resource_metadata="https://gateway.example.com/.well-known/oauth-protected-resource/mcp"`
	metadataFromRequest     = `, resource_metadata="http://example.com/.well-known/oauth-protected-resource/mcp"`
)

// fakeIdP serves the two OIDC endpoints go-oidc needs (discovery and JWKS)
// and mints RS256 tokens signed by the key it publishes.
type fakeIdP struct {
	issuer string
	signer jose.Signer
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, testRSABits)
	require.NoError(t, err)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"jwks_uri":                              srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{string(jose.RS256)},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: testKeyID, Algorithm: string(jose.RS256), Use: "sig"},
		}})
	})

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: &jose.JSONWebKey{Key: key, KeyID: testKeyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)
	return &fakeIdP{issuer: srv.URL, signer: signer}
}

// mint signs a valid token for this issuer; override lets a case tamper with
// the registered claims before signing and private adds non-standard ones.
func (p *fakeIdP) mint(t *testing.T, override func(*jwt.Claims), private ...map[string]any) string {
	t.Helper()
	now := time.Now()
	claims := jwt.Claims{
		Issuer:   p.issuer,
		Subject:  testSubject,
		Audience: jwt.Audience{testClientID},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(testTokenTTL)),
	}
	if override != nil {
		override(&claims)
	}
	builder := jwt.Signed(p.signer).Claims(claims)
	for _, extra := range private {
		builder = builder.Claims(extra)
	}
	raw, err := builder.Serialize()
	require.NoError(t, err)
	return raw
}

// newAuthEngine wires the real middleware in front of a handler that echoes
// what the middleware stored on the request context, with the routes
// cmd/gateway/main.go registers. mcp is nil unless the case needs the gateway
// to be an exposed MCP server.
func newAuthEngine(t *testing.T, auth config.AuthConfig, mcp *config.MCPConfig) *gin.Engine {
	t.Helper()
	cfg := config.Config{Auth: &auth, MCP: mcp}
	mw, err := middlewares.NewOIDCAuthenticatorMiddleware(logger.NewNoopLogger(), cfg)
	require.NoError(t, err)
	router := api.NewRouter(cfg, logger.NewNoopLogger(), nil, nil, nil, nil, nil, nil, nil)

	r := gin.New()
	r.Use(mw.Middleware())
	echo := func(c *gin.Context) {
		claims, _ := c.Request.Context().Value(types.ClaimsContextKey).(map[string]any)
		token, _ := c.Request.Context().Value(types.AuthTokenContextKey).(string)
		c.JSON(http.StatusOK, gin.H{"sub": claims["sub"], "token": token})
	}
	r.GET(middlewares.HealthPath, echo)
	r.GET(middlewares.MCPProtectedResourcePath, router.MCPProtectedResourceMetadataHandler)
	r.GET(testRoute, echo)
	r.POST(middlewares.MCPPath, echo)
	return r
}

func TestNewOIDCAuthenticatorMiddleware(t *testing.T) {
	idp := newFakeIdP(t)

	tests := []struct {
		name     string
		auth     config.AuthConfig
		wantNoop bool
		wantErr  bool
	}{
		{name: "Disabled returns Noop", auth: config.AuthConfig{Enabled: false}, wantNoop: true},
		{name: "Enabled without issuer fails", auth: config.AuthConfig{Enabled: true, OidcClientId: testClientID}, wantErr: true},
		{name: "Enabled without audience or client id fails", auth: config.AuthConfig{Enabled: true, OidcIssuer: idp.issuer}, wantErr: true},
		{name: "Unreachable issuer fails", auth: config.AuthConfig{Enabled: true, OidcIssuer: "http://127.0.0.1:1", OidcClientId: testClientID}, wantErr: true},
		{name: "Discovery succeeds", auth: config.AuthConfig{Enabled: true, OidcIssuer: idp.issuer, OidcClientId: testClientID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw, err := middlewares.NewOIDCAuthenticatorMiddleware(logger.NewNoopLogger(), config.Config{Auth: &tt.auth})
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			_, isNoop := mw.(*middlewares.OIDCAuthenticatorNoop)
			assert.Equal(t, tt.wantNoop, isNoop)
		})
	}
}

func TestOIDCAuthenticatorMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idp := newFakeIdP(t)
	otherIdP := newFakeIdP(t)

	// Audience list set: the client ID alone is no longer accepted.
	withAudienceList := newAuthEngine(t, config.AuthConfig{
		Enabled: true, OidcIssuer: idp.issuer, OidcClientId: testClientID,
		OidcAudience: testAPIAudience + ", second-api",
	}, nil)
	// Audience list empty: falls back to the client ID.
	authOnly := config.AuthConfig{Enabled: true, OidcIssuer: idp.issuer, OidcClientId: testClientID}
	withClientID := newAuthEngine(t, authOnly, nil)
	withMCPExposed := newAuthEngine(t, authOnly, &config.MCPConfig{Enabled: true, Expose: true})
	withMCPResourceURL := newAuthEngine(t, authOnly, &config.MCPConfig{Enabled: true, Expose: true, ResourceUrl: testResourceURL})

	valid := idp.mint(t, nil)
	forAPI := idp.mint(t, func(c *jwt.Claims) { c.Audience = jwt.Audience{testAPIAudience} })

	tests := []struct {
		name          string
		engine        *gin.Engine
		method        string
		path          string
		header        string
		wantStatus    int
		wantChallenge string
		wantToken     string
	}{
		{name: "Missing header", engine: withClientID, path: testRoute, wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing},
		{name: "Wrong scheme", engine: withClientID, path: testRoute, header: "Basic " + valid, wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing},
		{name: "Bare token without scheme", engine: withClientID, path: testRoute, header: valid, wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing},
		{name: "Bearer with empty token", engine: withClientID, path: testRoute, header: "Bearer ", wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing},
		{name: "Malformed token", engine: withClientID, path: testRoute, header: "Bearer not-a-jwt", wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{
			name: "Expired token", engine: withClientID, path: testRoute,
			header: "Bearer " + idp.mint(t, func(c *jwt.Claims) {
				c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * testTokenTTL))
				c.Expiry = jwt.NewNumericDate(time.Now().Add(-testTokenTTL))
			}),
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid,
		},
		{name: "Token from another issuer", engine: withClientID, path: testRoute, header: "Bearer " + otherIdP.mint(t, nil), wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{
			name: "Wrong audience", engine: withClientID, path: testRoute,
			header:     "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = jwt.Audience{"someone-else"} }),
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid,
		},
		{name: "Client id rejected when audience list is set", engine: withAudienceList, path: testRoute, header: "Bearer " + valid, wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "API audience accepted from list", engine: withAudienceList, path: testRoute, header: "Bearer " + forAPI, wantStatus: http.StatusOK, wantToken: forAPI},
		{
			name: "Second audience accepted from list", engine: withAudienceList, path: testRoute,
			header:     "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = jwt.Audience{"second-api"} }),
			wantStatus: http.StatusOK,
		},
		{
			name: "No aud falls back to client_id claim", engine: withClientID, path: testRoute,
			header:     "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = nil }, map[string]any{"client_id": testClientID}),
			wantStatus: http.StatusOK,
		},
		{
			name: "No aud and unknown client_id rejected", engine: withClientID, path: testRoute,
			header:     "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = nil }, map[string]any{"client_id": "someone-else"}),
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid,
		},
		{
			name: "No aud and no client_id rejected", engine: withClientID, path: testRoute,
			header:     "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = nil }),
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid,
		},
		{name: "Valid token", engine: withClientID, path: testRoute, header: "Bearer " + valid, wantStatus: http.StatusOK, wantToken: valid},
		{name: "Lowercase scheme accepted", engine: withClientID, path: testRoute, header: "bearer " + valid, wantStatus: http.StatusOK, wantToken: valid},
		{name: "Health bypasses auth", engine: withClientID, path: middlewares.HealthPath, wantStatus: http.StatusOK},
		{
			name: "MCP endpoint requires a token", engine: withClientID, method: http.MethodPost, path: middlewares.MCPPath,
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing,
		},
		{
			name: "MCP endpoint accepts a valid token", engine: withClientID, method: http.MethodPost, path: middlewares.MCPPath,
			header: "Bearer " + valid, wantStatus: http.StatusOK, wantToken: valid,
		},
		{
			name: "MCP challenge omits the metadata while /mcp is not exposed", engine: withClientID, method: http.MethodPost, path: middlewares.MCPPath,
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing,
		},
		{
			name: "MCP missing token challenge points at the metadata", engine: withMCPExposed, method: http.MethodPost, path: middlewares.MCPPath,
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing + metadataFromRequest,
		},
		{
			name: "MCP invalid token challenge points at the metadata", engine: withMCPExposed, method: http.MethodPost, path: middlewares.MCPPath,
			header: "Bearer not-a-jwt", wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid + metadataFromRequest,
		},
		{
			name: "MCP metadata follows the configured resource url", engine: withMCPResourceURL, method: http.MethodPost, path: middlewares.MCPPath,
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing + metadataFromResourceURL,
		},
		{
			name: "Other endpoints do not advertise the MCP metadata", engine: withMCPExposed, path: testRoute,
			wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing,
		},
		{name: "Protected resource metadata bypasses auth", engine: withMCPExposed, path: middlewares.MCPProtectedResourcePath, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			req := httptest.NewRequest(method, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			tt.engine.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantChallenge, w.Header().Get(wwwAuthenticate))
			if tt.wantToken == "" {
				return
			}
			var body struct {
				Sub   string `json:"sub"`
				Token string `json:"token"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, testSubject, body.Sub)
			assert.Equal(t, tt.wantToken, body.Token)
		})
	}
}

// TestMCPProtectedResourceDiscovery walks the flow MCP 2026-07-28 expects of a
// client that knows only the /mcp URL: the 401 hands it the metadata document,
// the document names the issuer, and its resource is the endpoint the
// challenge came from - which is what the client checks before trusting it.
func TestMCPProtectedResourceDiscovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idp := newFakeIdP(t)
	engine := newAuthEngine(t,
		config.AuthConfig{Enabled: true, OidcIssuer: idp.issuer, OidcClientId: testClientID},
		&config.MCPConfig{Enabled: true, Expose: true})

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, middlewares.MCPPath, nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)

	_, advertised, ok := strings.Cut(w.Header().Get(wwwAuthenticate), `resource_metadata="`)
	require.True(t, ok, "the challenge must point at the metadata document")
	metadataURL, _, _ := strings.Cut(advertised, `"`)
	u, err := url.Parse(metadataURL)
	require.NoError(t, err)

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, u.Path, nil))
	require.Equal(t, http.StatusOK, w.Code, "the document is fetched without a token")

	var doc types.OAuthProtectedResourceMetadata
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.Equal(t, []string{idp.issuer}, doc.AuthorizationServers)
	assert.Equal(t, testRequestResource, doc.Resource)
	assert.Equal(t, metadataURL, middlewares.ProtectedResourceMetadataURL(doc.Resource))
}
