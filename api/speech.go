package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// SpeechHandler implements an OpenAI-compatible POST /v1/audio/speech
// endpoint: https://platform.openai.com/docs/api-reference/audio/createSpeech
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Audio API fields pass through untouched. The synthesized audio comes
// back as raw binary and is streamed to the caller with the upstream's
// Content-Type (e.g. audio/mpeg for response_format mp3).
//
// ElevenLabs is the one provider whose audio API is not OpenAI-compatible, so
// its speech and sound-effect requests are rewritten instead of proxied
// byte-for-byte.
//
// Only providers that natively implement the Audio API are supported
// (currently openai and elevenlabs); other providers receive a 400, mirroring
// the schema's SpeechNotSupported response. The llamacpp registry entry also
// carries a Speech endpoint, but that path is a work in progress and not
// supported yet.
//
// The endpoint is opt-in via AUDIO_ENABLED (default off). When disabled, the
// handler returns 404.
func (router *RouterImpl) SpeechHandler(w http.ResponseWriter, r *http.Request) {
	if !router.audioEnabled(w) {
		return
	}

	// The reserved local/ prefix is served by the built-in llama-tts engine
	// instead of a provider; everything else is proxied byte-for-byte as before.
	if r.URL.Query().Get("provider") == "" && router.serveLocalSpeech(w, r) {
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      "Audio API",
		exampleModel: "openai/tts-1",
		endpointOf:   func(e types.Endpoints) *string { return e.Speech },
		preparers:    map[types.Provider]jsonPreparer{constants.ElevenlabsID: router.elevenlabsSampleVoice},
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsSpeech},
	})
}

// elevenlabsSpeech rewrites an OpenAI CreateSpeechRequest into the ElevenLabs
// text-to-speech shape, which carries the voice in the URL path and the audio
// container in an output_format query parameter.
func elevenlabsSpeech(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateSpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	return elevenlabs.Speech(endpoint, model, req)
}

const (
	// voicesAPIName labels ElevenLabs' voices endpoints in logs and errors.
	voicesAPIName = "Voices API"

	// elevenlabsMaxResponseSize caps how much of an ElevenLabs account response
	// is read. A voice list carries every voice of the account.
	elevenlabsMaxResponseSize = 8 << 20
)

// elevenlabsSampleVoice has a speech request that brings a voice sample speak
// in the ElevenLabs voice cloned from it.
func (router *RouterImpl) elevenlabsSampleVoice(r *http.Request, provider core.IProvider, body []byte) ([]byte, *upstreamFailure) {
	var req types.CreateSpeechRequest
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode speech request", err)
		return nil, &upstreamFailure{http.StatusBadRequest, "Failed to decode request"}
	}

	account := elevenlabsAccount{router: router, request: r, provider: provider}
	req, err := router.elevenlabsVoices.InSampleVoice(r.Context(), account, req)
	if err != nil {
		router.logger.Error("failed to speak in the voice sample", err)
		return nil, elevenlabsFailure(err)
	}

	prepared, err := json.Marshal(req)
	if err != nil {
		router.logger.Error("failed to encode speech request", err)
		return nil, &upstreamFailure{http.StatusInternalServerError, "Failed to encode request"}
	}
	return prepared, nil
}

// elevenlabsFailure renders an ElevenLabs error for the caller: a failure of
// the gateway's own, a refusal with ElevenLabs' status, a generation it turned
// down or ran out of time for, or a bad gateway.
func elevenlabsFailure(err error) *upstreamFailure {
	var failure *upstreamFailure
	if errors.As(err, &failure) {
		return failure
	}
	var refusal *elevenlabs.Refusal
	if errors.As(err, &refusal) {
		return &upstreamFailure{refusal.Status, err.Error()}
	}
	var generation *elevenlabs.GenerationFailure
	if errors.As(err, &generation) && generation.Rejected() {
		return &upstreamFailure{http.StatusBadRequest, err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &upstreamFailure{http.StatusGatewayTimeout, "The provider did not finish in time."}
	}
	return &upstreamFailure{http.StatusBadGateway, err.Error()}
}

// elevenlabsAccount reaches the ElevenLabs account of the gateway's key for
// one request, with the provider's auth and the request's tracing.
type elevenlabsAccount struct {
	router   *RouterImpl
	request  *http.Request
	provider core.IProvider
}

func (a elevenlabsAccount) ListVoices(ctx context.Context) ([]byte, error) {
	return a.call(ctx, voicesAPIName, func(e types.Endpoints) *string { return e.Voices }, upstreamRequest{method: http.MethodGet})
}

func (a elevenlabsAccount) CreateVoice(ctx context.Context, contentType string, form []byte) ([]byte, error) {
	return a.call(ctx, voicesAPIName, func(e types.Endpoints) *string { return e.VoicesCreate }, upstreamRequest{body: bytes.NewReader(form), contentType: contentType})
}

func (a elevenlabsAccount) CreateImage(ctx context.Context, generation []byte) ([]byte, error) {
	return a.call(ctx, imagesAPIName, func(e types.Endpoints) *string { return e.ImagesEdits }, upstreamRequest{body: bytes.NewReader(generation), contentType: contentTypeJSON, accept: contentTypeJSON})
}

func (a elevenlabsAccount) Image(ctx context.Context, generationID string) ([]byte, error) {
	generationOf := func(e types.Endpoints) *string {
		if e.ImagesRetrieve == nil {
			return nil
		}
		endpoint := strings.ReplaceAll(*e.ImagesRetrieve, generationIDPlaceholder, url.PathEscape(generationID))
		return &endpoint
	}
	return a.call(ctx, imagesAPIName, generationOf, upstreamRequest{method: http.MethodGet, accept: contentTypeJSON})
}

func (a elevenlabsAccount) call(ctx context.Context, apiName string, endpointOf func(types.Endpoints) *string, req upstreamRequest) ([]byte, error) {
	endpoint, msg := a.router.providerEndpoint(a.provider, endpointOf, apiName, "")
	if msg != "" {
		return nil, &upstreamFailure{http.StatusBadRequest, msg}
	}
	req.endpointPath = endpoint

	resp, done, failure := a.router.callUpstream(a.request.WithContext(ctx), a.provider, req)
	if failure != nil {
		return nil, failure
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, elevenlabsMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading the elevenlabs response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, elevenlabs.NewRefusal(resp.StatusCode, body)
	}
	return body, nil
}

// serveLocalSpeech handles the reserved local/ model prefix with the built-in
// llama-tts engine and reports whether the request was handled. The body is
// read once and rewound so the provider proxy path proceeds unchanged.
func (router *RouterImpl) serveLocalSpeech(w http.ResponseWriter, r *http.Request) bool {
	if router.tts == nil {
		return false
	}
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return true
	}
	if tooLarge {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return true
	}

	var probe struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return false // let the provider path report the malformed body
	}
	if !strings.HasPrefix(probe.Model, tts.ModelPrefix) {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return false
	}
	if probe.Model != tts.ReservedModelID {
		router.logger.Error("unknown local speech model", nil, "model", probe.Model)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown local speech model %q. %q is the only local speech model.", probe.Model, tts.ReservedModelID)})
		return true
	}
	if reason := router.modelDenied(probe.Model); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return true
	}

	var req struct {
		Input          string `json:"input"`
		ResponseFormat string `json:"response_format"`
		ReferenceAudio []byte `json:"reference_audio"`
		Language       string `json:"language"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return true
	}
	if strings.TrimSpace(req.Input) == "" {
		router.logger.Error("local speech request missing input", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The 'input' field is required."})
		return true
	}
	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		router.logger.Error("unsupported response_format for local engine", nil, "response_format", req.ResponseFormat)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: `The local speech engine only supports response_format "wav".`})
		return true
	}
	if req.Language != "" && !slices.Contains(tts.SupportedLanguages, req.Language) {
		router.logger.Error("unsupported language for local engine", nil, "language", req.Language)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The local speech engine does not support language %q. Supported languages: %s.", req.Language, strings.Join(tts.SupportedLanguages, ", "))})
		return true
	}

	middlewares.ResetWriteDeadline(w, router.localSpeechWriteWindow())
	audio, err := router.tts.Synthesize(r.Context(), tts.Request{
		Input:          req.Input,
		ReferenceAudio: req.ReferenceAudio,
		Language:       req.Language,
	})
	if err != nil {
		var notReady *tts.NotReadyError
		switch {
		case errors.As(err, &notReady):
			router.logger.Warn("local speech assets not ready", nil, "detail", notReady.Error())
			w.Header().Set("Retry-After", strconv.Itoa(tts.RetryAfterSeconds))
			middlewares.WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: notReady.Error()})
		case errors.Is(err, context.DeadlineExceeded):
			router.logger.Error("local speech synthesis timed out", err)
			middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: "Local speech synthesis timed out"})
		default:
			router.logger.Error("local speech synthesis failed", err)
			middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to synthesize local speech"})
		}
		return true
	}

	writeData(w, http.StatusOK, "audio/wav", audio)
	return true
}

// localSpeechWriteWindow is how long a local speech response may take: the
// longest a synthesis runs, then the usual time to write the answer. Without
// it the server's write timeout closes the connection mid-synthesis.
func (router *RouterImpl) localSpeechWriteWindow() time.Duration {
	return time.Duration(router.cfg.AudioLocalTimeout)*time.Second + router.cfg.Server.WriteTimeout
}
