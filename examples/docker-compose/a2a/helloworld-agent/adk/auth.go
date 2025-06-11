package adk

import (
	"context"
	"net/http"
	"strings"

	oidcV3 "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type contextKey string

const (
	AuthTokenContextKey contextKey = "authToken"
	IDTokenContextKey   contextKey = "idToken"
)

// OIDCAuthenticator interface for authentication middleware
type OIDCAuthenticator interface {
	Middleware() gin.HandlerFunc
}

// OIDCAuthenticatorImpl implements OIDC authentication
type OIDCAuthenticatorImpl struct {
	logger   *zap.Logger
	verifier *oidcV3.IDTokenVerifier
	config   oauth2.Config
}

// OIDCAuthenticatorNoop is a no-op authenticator for when auth is disabled
type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticatorMiddleware creates a new OIDC authenticator middleware
func NewOIDCAuthenticatorMiddleware(logger *zap.Logger, cfg Config) (OIDCAuthenticator, error) {
	if !cfg.AuthConfig.Enable {
		return &OIDCAuthenticatorNoop{}, nil
	}

	provider, err := oidcV3.NewProvider(context.Background(), cfg.AuthConfig.IssuerURL)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidcV3.Config{
		ClientID: cfg.AuthConfig.ClientID,
	}

	return &OIDCAuthenticatorImpl{
		logger:   logger,
		verifier: provider.Verifier(oidcConfig),
		config: oauth2.Config{
			ClientID:     cfg.AuthConfig.ClientID,
			ClientSecret: cfg.AuthConfig.ClientSecret,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidcV3.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// Middleware returns the OIDC authentication middleware for OIDCAuthenticatorImpl
func (auth *OIDCAuthenticatorImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			auth.logger.Error("missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			auth.logger.Error("invalid authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		idToken, err := auth.verifier.Verify(c.Request.Context(), token)
		if err != nil {
			auth.logger.Error("failed to verify id token", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set(string(AuthTokenContextKey), token)
		c.Set(string(IDTokenContextKey), idToken)
		c.Next()
	}
}

// Middleware returns a no-op middleware for OIDCAuthenticatorNoop
func (auth *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
