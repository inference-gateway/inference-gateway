package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const maxVoiceResponseBytes = 8 << 20

// clonedVoices remembers the ElevenLabs voice cloned from each sample. Its
// lock lets one request at a time look a voice up or create it, so two
// sentences in a new voice never clone the same sample twice.
//
// shortcut: a voice deleted at ElevenLabs stays remembered until the gateway
// restarts, look it up again on a missing voice if that bites.
type clonedVoices struct {
	mu  sync.Mutex
	ids map[string]string
}

// elevenlabsClonedVoice swaps a speech request's reference_audio for the
// ElevenLabs voice cloned from it, creating that voice on first use. A request
// without a sample passes through untouched.
func (router *RouterImpl) elevenlabsClonedVoice(r *http.Request, provider core.IProvider, body []byte) ([]byte, *upstreamFailure) {
	var req types.CreateSpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode speech request", err)
		return nil, &upstreamFailure{http.StatusBadRequest, "Failed to decode request"}
	}
	if req.ReferenceAudio == nil || len(*req.ReferenceAudio) == 0 {
		return body, nil
	}

	voice, failure := router.clonedVoiceID(r, provider, *req.ReferenceAudio)
	if failure != nil {
		return nil, failure
	}

	req.Voice, req.ReferenceAudio = voice, nil
	out, err := json.Marshal(req)
	if err != nil {
		router.logger.Error("failed to encode speech request", err)
		return nil, &upstreamFailure{http.StatusInternalServerError, "Failed to encode request"}
	}
	return out, nil
}

func (router *RouterImpl) clonedVoiceID(r *http.Request, provider core.IProvider, sample []byte) (string, *upstreamFailure) {
	name := elevenlabs.ClonedVoiceName(sample)

	router.clonedVoices.mu.Lock()
	defer router.clonedVoices.mu.Unlock()

	if id, ok := router.clonedVoices.ids[name]; ok {
		return id, nil
	}
	id, failure := router.findOrCloneVoice(r, provider, name, sample)
	if failure != nil {
		return "", failure
	}
	if router.clonedVoices.ids == nil {
		router.clonedVoices.ids = map[string]string{}
	}
	router.clonedVoices.ids[name] = id
	return id, nil
}

func (router *RouterImpl) findOrCloneVoice(r *http.Request, provider core.IProvider, name string, sample []byte) (string, *upstreamFailure) {
	listed, failure := router.elevenlabsJSON(r, provider, upstreamRequest{
		method:       http.MethodGet,
		endpointPath: elevenlabs.VoicesEndpoint,
	}, "list its voices")
	if failure != nil {
		return "", failure
	}
	id, found, err := elevenlabs.FindVoice(listed, name)
	if err != nil {
		router.logger.Error("failed to read elevenlabs voices", err)
		return "", &upstreamFailure{http.StatusBadGateway, err.Error()}
	}
	if found {
		return id, nil
	}

	contentType, form, err := elevenlabs.AddVoiceForm(name, sample)
	if err != nil {
		router.logger.Error("failed to build the voice clone request", err)
		return "", &upstreamFailure{http.StatusInternalServerError, "Failed to build the voice clone request"}
	}
	added, failure := router.elevenlabsJSON(r, provider, upstreamRequest{
		endpointPath: elevenlabs.AddVoiceEndpoint,
		body:         bytes.NewReader(form),
		contentType:  contentType,
	}, "clone the voice sample")
	if failure != nil {
		return "", failure
	}
	id, err = elevenlabs.AddedVoiceID(added)
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
