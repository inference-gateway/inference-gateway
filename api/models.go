package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// parseIncludeParam splits the comma-separated `include` value into a
// de-duplicated list of known metadata keys, preserving first-seen order. An
// empty value yields no keys; an unrecognized key is rejected so typos fail
// loudly instead of silently returning less data. Validity is checked against
// the generated ListModelsParamsInclude enum, keeping the accepted set in sync
// with openapi.yaml as new metadata fields are added.
func parseIncludeParam(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if !types.ListModelsParamsInclude(key).Valid() {
			return nil, fmt.Errorf("unknown include value %q", key)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys, nil
}

// includesDeprecated reports whether the request asked to list models the
// models.dev table marks as deprecated, which are hidden otherwise.
func includesDeprecated(includeKeys []string) bool {
	return slices.Contains(includeKeys, string(types.ListModelsParamsIncludeDeprecated))
}

// renderModelsResponse writes the models list as JSON. When include keys are
// present it injects each requested key as an explicit null on every model
// unless already populated, keeping the requested-but-unavailable state
// distinguishable from an absent field. With no include keys the typed response
// is written unchanged so the default payload stays byte-for-byte
// OpenAI-compatible.
func (router *RouterImpl) renderModelsResponse(w http.ResponseWriter, resp types.ListModelsResponse, includeKeys []string) {
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
		for i := range resp.Data {
			resp.Data[i].ContextWindow = nil
		}
	}
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludePricing)) {
		for i := range resp.Data {
			resp.Data[i].Pricing = nil
		}
	}
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludeModalities)) {
		for i := range resp.Data {
			resp.Data[i].Modalities = nil
		}
	}

	if len(includeKeys) == 0 {
		middlewares.WriteJSON(w, http.StatusOK, resp)
		return
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		router.logger.Error("failed to marshal models response", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		router.logger.Error("failed to decode models response", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	if data, ok := envelope["data"].([]any); ok {
		for _, item := range data {
			model, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range includeKeys {
				if key == string(types.ListModelsParamsIncludeDeprecated) {
					continue
				}
				if _, exists := model[key]; !exists {
					model[key] = nil
				}
			}
		}
	}

	middlewares.WriteJSON(w, http.StatusOK, envelope)
}

// ListModelsHandler implements an OpenAI-compatible API endpoint
// that returns model information in the standard OpenAI format.
//
// This handler supports the OpenAI GET /v1/models endpoint specification:
// https://platform.openai.com/docs/api-reference/models/list
//
// Parameters:
//   - provider (query): Optional. When specified, returns models from only that provider.
//     If not specified, returns models from all configured providers.
//   - include (query): Optional. Comma-separated list of extra per-model metadata
//     fields to include (context_window, pricing, modalities). Keys are trimmed and
//     de-duplicated; an unknown key returns 400. Requested-but-unresolved keys are
//     returned as explicit null. When omitted, no metadata fields are added.
//     The `deprecated` key adds no field: it lists models the community status
//     table marks as deprecated, which are hidden by default.
//
// Response format:
//
//	{
//	  "object": "list",
//	  "data": [
//	   {
//	      "id": "model-id",
//	      "object": "model",
//	      "created": 1686935002,
//	      "owned_by": "provider-name",
//	      "served_by": "provider-name"
//	   },
//	   ...
//	  ]
//	}
//
// This endpoint allows applications built for OpenAI's API to work seamlessly
// with the Inference Gateway's multi-provider architecture.
func (router *RouterImpl) ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	includeKeys, err := parseIncludeParam(r.URL.Query().Get("include"))
	if err != nil {
		router.logger.Error("invalid include parameter", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	providerID := types.Provider(r.URL.Query().Get("provider"))
	if providerID != "" {
		provider, msg, err := router.buildProvider(providerID)
		if err != nil {
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		response, err := provider.ListModels(ctx)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				router.logger.Error("request timed out", err, "provider", provider.GetName())
				middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
				return
			}
			router.logger.Error("failed to list models", err, "provider", provider.GetName())
			middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to list models"})
			return
		}

		response.Data = routing.FilterModels(response.Data, router.cfg.AllowedModels, router.cfg.DisallowedModels, includesDeprecated(includeKeys))

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, response.Data)
		}

		router.renderModelsResponse(w, response, includeKeys)
	} else {
		var wg sync.WaitGroup
		providersCfg := router.cfg.Providers

		ch := make(chan types.ListModelsResponse, len(providersCfg))

		ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		for providerID := range providersCfg {
			wg.Add(1)
			go func(id types.Provider) {
				defer wg.Done()

				provider, err := router.registry.BuildProvider(id, router.client)
				if err != nil {
					router.logger.Error("failed to create provider", err, "provider", id)
					return
				}

				response, err := provider.ListModels(ctx)
				if err != nil {
					if ctx.Err() == context.DeadlineExceeded {
						router.logger.Error("request timed out", err, "provider", id)
						return
					}
					router.logger.Error("failed to list models", err, "provider", id)
					return
				}

				if response.Data == nil {
					response.Data = make([]types.Model, 0)
				}
				ch <- response
			}(providerID)
		}

		wg.Wait()
		close(ch)

		var allModels []types.Model
		for response := range ch {
			allModels = append(allModels, response.Data...)
		}

		if allModels == nil {
			allModels = make([]types.Model, 0)
		}

		allModels = routing.FilterModels(allModels, router.cfg.AllowedModels, router.cfg.DisallowedModels, includesDeprecated(includeKeys))

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, allModels)
		}

		unifiedResponse := types.ListModelsResponse{
			Object: "list",
			Data:   allModels,
		}

		router.renderModelsResponse(w, unifiedResponse, includeKeys)
	}
}
