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

// Environment names that toggle development-only behaviour across the gateway
const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// ListModelsTransformer interface for transforming provider-specific responses
type ListModelsTransformer interface {
	Transform() types.ListModelsResponse
}
