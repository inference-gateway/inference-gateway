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

// Environment names that toggle development-only behaviour across the gateway
const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// ListModelsTransformer interface for transforming provider-specific responses
type ListModelsTransformer interface {
	Transform() types.ListModelsResponse
}
