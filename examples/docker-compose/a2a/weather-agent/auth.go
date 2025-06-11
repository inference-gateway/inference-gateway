package main

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

// Middleware returns a no-op middleware for the noop authenticator
func (a *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware returns the OIDC authentication middleware
func (a *OIDCAuthenticatorImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for health check and agent info endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/.well-known/agent.json" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := authHeader[len(bearerPrefix):]
		idToken, err := a.verifier.Verify(context.Background(), token)
		if err != nil {
			a.logger.Error("failed to verify id token", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Set(string(AuthTokenContextKey), token)
		c.Set(string(IDTokenContextKey), idToken)

		c.Next()
	}
}
