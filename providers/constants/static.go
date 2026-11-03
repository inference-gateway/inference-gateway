package constants

import (
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// The authentication type of the specific provider
const (
	AuthTypeBearer  = "bearer"
	AuthTypeXheader = "xheader"
	AuthTypeQuery   = "query"
	AuthTypeNone    = "none"
)

// DefaultAuthHeader is the header the xheader auth type sends the provider API
// key in when a provider declares no auth_header of its own in openapi.yaml.
const DefaultAuthHeader = "x-api-key"

// LocalID is the reserved pseudo-provider behind the local/ model prefix:
// models the gateway itself serves (llama-server children for GGUFs,
// llama-tts for speech). It is not a registry entry and has no API URL.
const (
	LocalID          types.Provider = "local"
	LocalDisplayName                = "Local"
)

// Environment names that toggle development-only behaviour across the gateway
const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// ListModelsTransformer interface for transforming provider-specific responses
type ListModelsTransformer interface {
	Transform() types.ListModelsResponse
}
