package middlewares

import (
	"context"
	"net/http"
	"strings"

	oidcV3 "github.com/coreos/go-oidc/v3/oidc"
	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type OIDCAuthenticator interface {
	Middleware() gin.HandlerFunc
}

type OIDCAuthenticatorImpl struct {
	logger   logger.Logger
	verifier *oidcV3.IDTokenVerifier
}

type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticatorMiddleware creates a new OIDCAuthenticator instance
func NewOIDCAuthenticatorMiddleware(logger logger.Logger, cfg config.Config) (OIDCAuthenticator, error) {
	if !cfg.Auth.Enable {
		return &OIDCAuthenticatorNoop{}, nil
	}

	provider, err := oidcV3.NewProvider(context.Background(), cfg.Auth.OidcIssuer)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidcV3.Config{
		ClientID: cfg.Auth.OidcClientId,
	}

	return &OIDCAuthenticatorImpl{
		logger:   logger,
		verifier: provider.Verifier(oidcConfig),
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
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if _, err := a.verifier.Verify(c.Request.Context(), token); err != nil {
			a.logger.Error("failed to verify id token", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), types.AuthTokenContextKey, token)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
