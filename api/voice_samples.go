package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	maxVoiceResponseBytes = 8 << 20
	voicesAPIName         = "Voices API"
)

// elevenlabsSampleVoice swaps a speech request's reference_audio for the
// ElevenLabs voice cloned from it, since ElevenLabs takes no sample with a
// speech request. A request without a sample passes through untouched.
func (router *RouterImpl) elevenlabsSampleVoice(r *http.Request, provider core.IProvider, body []byte) ([]byte, *upstreamFailure) {
	var req types.CreateSpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode speech request", err)
		return nil, &upstreamFailure{http.StatusBadRequest, "Failed to decode request"}
	}
	if req.ReferenceAudio == nil || len(*req.ReferenceAudio) == 0 {
		return body, nil
	}

	voice, failure := router.findOrCloneVoice(r, provider, *req.ReferenceAudio)
	if failure != nil {
		return nil, failure
	}

	req.Voice, req.ReferenceAudio = voice, nil
	prepared, err := json.Marshal(req)
	if err != nil {
		router.logger.Error("failed to encode speech request", err)
		return nil, &upstreamFailure{http.StatusInternalServerError, "Failed to encode request"}
	}
	return prepared, nil
}

// findOrCloneVoice returns the ElevenLabs voice of a sample, cloning it when
// the account has none yet. One request at a time gets through, so two
// sentences in a new voice never clone the same sample twice. The account is
// asked every time, so a voice deleted there is cloned again.
func (router *RouterImpl) findOrCloneVoice(r *http.Request, provider core.IProvider, sample []byte) (string, *upstreamFailure) {
	listPath, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.Voices }, voicesAPIName, "")
	if msg != "" {
		return "", &upstreamFailure{http.StatusBadRequest, msg}
	}
	createPath, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.VoicesCreate }, voicesAPIName, "")
	if msg != "" {
		return "", &upstreamFailure{http.StatusBadRequest, msg}
	}

	router.voiceCloning.Lock()
	defer router.voiceCloning.Unlock()

	name := elevenlabs.ClonedVoiceName(sample)
	listed, failure := router.elevenlabsJSON(r, provider, upstreamRequest{method: http.MethodGet, endpointPath: listPath}, "list its voices")
	if failure != nil {
		return "", failure
	}
	id, err := elevenlabs.FindVoice(listed, name)
	if err != nil {
		router.logger.Error("failed to read elevenlabs voices", err)
		return "", &upstreamFailure{http.StatusBadGateway, err.Error()}
	}
	if id != "" {
		return id, nil
	}

	contentType, form, err := elevenlabs.CreateVoiceForm(name, sample)
	if err != nil {
		router.logger.Error("failed to build the voice clone request", err)
		return "", &upstreamFailure{http.StatusInternalServerError, "Failed to build the voice clone request"}
	}
	created, failure := router.elevenlabsJSON(r, provider, upstreamRequest{
		endpointPath: createPath,
		body:         bytes.NewReader(form),
		contentType:  contentType,
	}, "clone the voice sample")
	if failure != nil {
		return "", failure
	}
	id, err = elevenlabs.CreatedVoiceID(created)
	if err != nil {
		router.logger.Error("failed to read the cloned elevenlabs voice", err)
		return "", &upstreamFailure{http.StatusBadGateway, err.Error()}
	}
	router.logger.Info("cloned a voice sample at elevenlabs", "voice", name)
	return id, nil
}

// elevenlabsJSON calls ElevenLabs and returns a successful response body. A
// refusal comes back with its status and ElevenLabs' own message, so a key
// without the voices permissions says so.
func (router *RouterImpl) elevenlabsJSON(r *http.Request, provider core.IProvider, req upstreamRequest, action string) ([]byte, *upstreamFailure) {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		return nil, failure
	}
	defer done()
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxVoiceResponseBytes))
	if err != nil {
		router.logger.Error("failed to read elevenlabs response", err)
		return nil, &upstreamFailure{http.StatusBadGateway, "Failed to read upstream response"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &upstreamFailure{resp.StatusCode, fmt.Sprintf("elevenlabs could not %s: %s", action, elevenlabs.ErrorMessage(raw))}
	}
	return raw, nil
}
