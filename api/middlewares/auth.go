package middlewares

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	gin "github.com/gin-gonic/gin"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// oidcHTTPTimeout bounds OIDC discovery at startup and every JWKS refresh
	// go-oidc performs when it meets an unknown key ID, so a stalled issuer
	// cannot hang boot or pin request goroutines.
	oidcHTTPTimeout = 10 * time.Second

	bearerScheme = "Bearer"

	// clientIDClaim is checked in place of aud when a token has no aud.
	clientIDClaim = "client_id"

	// RFC 6750 §3 challenges: no error code when the request carried no bearer
	// credentials, invalid_token when it carried one that failed verification.
	wwwAuthenticateHeader  = "WWW-Authenticate"
	wwwAuthenticateMissing = `Bearer realm="inference-gateway"`
	wwwAuthenticateInvalid = `Bearer realm="inference-gateway", error="invalid_token"`
)

type OIDCAuthenticator interface {
	Middleware() gin.HandlerFunc
}

type OIDCAuthenticatorImpl struct {
	logger    logger.Logger
	verifier  *oidc.IDTokenVerifier
	audiences []string
	mcp       *config.MCPConfig
}

type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticatorMiddleware creates a new OIDCAuthenticator instance
func NewOIDCAuthenticatorMiddleware(logger logger.Logger, cfg config.Config) (OIDCAuthenticator, error) {
	if !cfg.Auth.Enabled {
		return &OIDCAuthenticatorNoop{}, nil
	}
	if cfg.Auth.OidcIssuer == "" {
		return nil, errors.New("AUTH_OIDC_ISSUER is required when AUTH_ENABLED=true")
	}
	audiences := strings.FieldsFunc(cmp.Or(cfg.Auth.OidcAudience, cfg.Auth.OidcClientId), isListSeparator)
	if len(audiences) == 0 {
		return nil, errors.New("AUTH_OIDC_AUDIENCE or AUTH_OIDC_CLIENT_ID is required when AUTH_ENABLED=true")
	}

	ctx := oidc.ClientContext(context.Background(), &http.Client{Timeout: oidcHTTPTimeout})
	provider, err := oidc.NewProvider(ctx, cfg.Auth.OidcIssuer)
	if err != nil {
		return nil, err
	}

	return &OIDCAuthenticatorImpl{
		logger:    logger,
		verifier:  provider.Verifier(&oidc.Config{SkipClientIDCheck: true}),
		audiences: audiences,
		mcp:       cfg.MCP,
	}, nil
}

func isListSeparator(r rune) bool { return r == ',' || unicode.IsSpace(r) }

// Noop implementation of the OIDCAuthenticator interface
func (a *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware implementation of the OIDCAuthenticator interface
func (a *OIDCAuthenticatorImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.URL.Path {
		case HealthPath, MCPProtectedResourcePath:
			c.Next()
			return
		}

		scheme, token, _ := strings.Cut(c.GetHeader("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !strings.EqualFold(scheme, bearerScheme) || token == "" {
			a.unauthorized(c, wwwAuthenticateMissing)
			return
		}

		idToken, err := a.verifier.Verify(c.Request.Context(), token)
		if err != nil {
			a.logger.Error("failed to verify bearer token", err)
			a.unauthorized(c, wwwAuthenticateInvalid)
			return
		}

		var claims map[string]any
		if err := idToken.Claims(&claims); err != nil {
			a.logger.Error("failed to decode bearer token claims", err)
			a.unauthorized(c, wwwAuthenticateInvalid)
			return
		}

		audiences := idToken.Audience
		if len(audiences) == 0 {
			if clientID, _ := claims[clientIDClaim].(string); clientID != "" {
				audiences = []string{clientID}
			}
		}
		if !slices.ContainsFunc(audiences, func(aud string) bool { return slices.Contains(a.audiences, aud) }) {
			a.logger.Error("failed to verify bearer token",
				fmt.Errorf("oidc: expected one of audiences %q got %q", a.audiences, audiences))
			a.unauthorized(c, wwwAuthenticateInvalid)
			return
		}

		ctx := context.WithValue(c.Request.Context(), types.AuthTokenContextKey, token)
		ctx = context.WithValue(ctx, types.ClaimsContextKey, claims)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// unauthorized answers the 401 challenge. On /mcp it also points at the RFC
// 9728 document, which is how an MCP client that knows only the endpoint URL
// discovers the authorization server (MCP 2026-07-28 authorization).
func (a *OIDCAuthenticatorImpl) unauthorized(c *gin.Context, challenge string) {
	if c.Request.URL.Path == MCPPath && MCPExposed(a.mcp) {
		metadata := ProtectedResourceMetadataURL(MCPResourceURL(a.mcp, c.Request))
		challenge += fmt.Sprintf(", resource_metadata=%q", metadata)
	}
	c.Header(wwwAuthenticateHeader, challenge)
	c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	c.Abort()
}
