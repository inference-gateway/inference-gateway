package types

type ContextKey string

const AuthTokenContextKey ContextKey = "authToken"

// ClaimsContextKey holds the verified OIDC claims (map[string]any) of the caller.
const ClaimsContextKey ContextKey = "claims"
