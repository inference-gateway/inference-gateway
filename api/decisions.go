package api

import (
	"net/http"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	systemone "github.com/inference-gateway/inference-gateway/internal/systemone"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// DecisionsHandler implements an OpenAI-compatible POST /v1/decisions
// endpoint: https://developers.openai.com/api/docs/guides/decisions
//
// The request body is forwarded byte-for-byte (only the `model` prefix is
// stripped); providers without a decisions endpoint receive a 400. Opt-in via
// DECISIONS_ENABLED while the upstream decision APIs are in preview.
func (router *RouterImpl) DecisionsHandler(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.DecisionsEnabled {
		router.logger.Error("decisions api not enabled", nil)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Decisions API is not enabled. Set DECISIONS_ENABLED=true to enable it."})
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:        "Decisions API",
		exampleModel:   "openai/gpt-6-luna",
		accept:         contentTypeJSON,
		endpointOf:     func(e types.Endpoints) *string { return e.Decisions },
		outputModality: types.ModalityDecisions,
		translators: map[types.Provider]jsonTranslator{
			constants.OllamaID:      systemOneDecisions,
			constants.OllamaCloudID: systemOneDecisions,
			constants.LlamacppID:    systemOneDecisions,
		},
		responseTranslators: map[types.Provider]jsonResponseTranslator{
			constants.OllamaID:      systemone.Response,
			constants.OllamaCloudID: systemone.Response,
			constants.LlamacppID:    systemone.Response,
		},
	})
}

// systemOneDecisions rewrites a CreateDecisionRequest into the System One
// request Ollama and llama.cpp serve at /systemone.
func systemOneDecisions(endpoint, model string, body []byte) (string, string, []byte, error) {
	out, err := systemone.Request(model, body)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, "", out, nil
}
