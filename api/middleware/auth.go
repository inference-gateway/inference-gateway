package middlewares

import (
	"context"
	"net/http"
	"strings"

	oidc "github.com/coreos/go-oidc"
	config "github.com/edenreich/inference-gateway/config"
	oauth2 "golang.org/x/oauth2"
)

type OIDCAuthenticator interface {
	Middleware(next http.Handler) http.Handler
}

type OIDCAuthenticatorImpl struct {
	verifier *oidc.IDTokenVerifier
	config   oauth2.Config
}

type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticator creates a new OIDCAuthenticator instance
func NewOIDCAuthenticator(cfg config.Config) (OIDCAuthenticator, error) {
	if !cfg.EnableAuth {
		return &OIDCAuthenticatorNoop{}, nil
	}

	provider, err := oidc.NewProvider(context.Background(), cfg.OIDCIssuerURL)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidc.Config{
		ClientID: cfg.OIDCClientID,
	}

	return &OIDCAuthenticatorImpl{
		verifier: provider.Verifier(oidcConfig),
		config: oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// Verify the ID token and if authenticated pass the request to the next handler
func (a *OIDCAuthenticatorImpl) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header missing", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		idToken, err := a.verifier.Verify(context.Background(), token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		type contextKey string
		const idTokenKey contextKey = "idToken"
		ctx := context.WithValue(r.Context(), idTokenKey, idToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Noop implementation of the OIDCAuthenticator interface
func (a *OIDCAuthenticatorNoop) Middleware(next http.Handler) http.Handler {
	return next
}
