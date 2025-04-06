package middlewares

import (
	"context"
	"net/http"
	"strings"

	oidcV3 "github.com/coreos/go-oidc/v3/oidc"
	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	oauth2 "golang.org/x/oauth2"
)

type contextKey string

const (
	AuthTokenContextKey contextKey = "authToken"
	IDTokenContextKey   contextKey = "idToken"
)

type OIDCAuthenticator interface {
	Middleware() gin.HandlerFunc
}

type OIDCAuthenticatorImpl struct {
	logger   logger.Logger
	verifier *oidcV3.IDTokenVerifier
	config   oauth2.Config
}

type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticatorMiddleware creates a new OIDCAuthenticator instance
func NewOIDCAuthenticatorMiddleware(logger logger.Logger, cfg config.Config) (OIDCAuthenticator, error) {
	if !cfg.EnableAuth {
		return &OIDCAuthenticatorNoop{}, nil
	}

	provider, err := oidcV3.NewProvider(context.Background(), cfg.OIDC.IssuerUrl)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidcV3.Config{
		ClientID: cfg.OIDC.ClientId,
	}

	return &OIDCAuthenticatorImpl{
		logger:   logger,
		verifier: provider.Verifier(oidcConfig),
		config: oauth2.Config{
			ClientID:     cfg.OIDC.ClientId,
			ClientSecret: cfg.OIDC.ClientSecret,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidcV3.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// Noop implementation of the OIDCAuthenticator interface
func (a *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware implementation of the OIDCAuthenticator interface
func (a *OIDCAuthenticatorImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		idToken, err := a.verifier.Verify(context.Background(), token)
		if err != nil {
			a.logger.Error("Failed to verify ID token: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Set(string(AuthTokenContextKey), token)
		c.Set(string(IDTokenContextKey), idToken)

		c.Next()
	}
}
