package tests

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	openapi "github.com/inference-gateway/inference-gateway/internal/openapi"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	endpointKeyModels           = "models"
	endpointKeyChat             = "chat"
	endpointKeyResponses        = "responses"
	endpointKeyImages           = "images"
	endpointKeyImagesEdits      = "images_edits"
	endpointKeyImagesVariations = "images_variations"
	endpointKeySpeech           = "speech"
)

// TestProviderEndpointsMatchSchema fails when an endpoint declared under
// x-provider-configs in openapi.yaml never reaches the generated registry.
// The optional endpoints (responses, images) gate whole handlers, so a
// generator that silently drops them turns the feature off at runtime.
func TestProviderEndpointsMatchSchema(t *testing.T) {
	schema, err := openapi.Read("../openapi.yaml")
	require.NoError(t, err)

	for name, cfg := range schema.Components.Schemas.Provider.XProviderConfigs {
		t.Run(name, func(t *testing.T) {
			provider, ok := registry.Registry[types.Provider(cfg.ID)]
			require.True(t, ok, "provider %s missing from registry", cfg.ID)

			assert.Equal(t, cfg.Endpoints[endpointKeyModels].Endpoint, provider.Endpoints.Models)
			assert.Equal(t, cfg.Endpoints[endpointKeyChat].Endpoint, provider.Endpoints.Chat)
			assertOptionalEndpoint(t, cfg.Endpoints[endpointKeyResponses].Endpoint, provider.Endpoints.Responses, endpointKeyResponses)
			assertOptionalEndpoint(t, cfg.Endpoints[endpointKeyImages].Endpoint, provider.Endpoints.Images, endpointKeyImages)
			assertOptionalEndpoint(t, cfg.Endpoints[endpointKeyImagesEdits].Endpoint, provider.Endpoints.ImagesEdits, endpointKeyImagesEdits)
			assertOptionalEndpoint(t, cfg.Endpoints[endpointKeyImagesVariations].Endpoint, provider.Endpoints.ImagesVariations, endpointKeyImagesVariations)
			assertOptionalEndpoint(t, cfg.Endpoints[endpointKeySpeech].Endpoint, provider.Endpoints.Speech, endpointKeySpeech)
		})
	}
}

func assertOptionalEndpoint(t *testing.T, expected string, actual *string, key string) {
	t.Helper()

	if expected == "" {
		assert.Nil(t, actual, "%s endpoint not declared in openapi.yaml", key)
		return
	}

	require.NotNil(t, actual, "%s endpoint declared in openapi.yaml but missing from the registry", key)
	assert.Equal(t, expected, *actual)
}
