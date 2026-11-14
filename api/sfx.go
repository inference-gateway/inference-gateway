package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// SFXHandler implements POST /v1/audio/sfx, the gateway extension that
// generates a non-speech audio clip - a sound effect or ambience - from a text
// prompt. OpenAI has no sound-effects endpoint, so the request mirrors
// /audio/speech (JSON in, raw audio bytes out) and only providers that carry an
// Sfx endpoint (currently elevenlabs and mirelo) can serve it; the rest receive
// a 400, mirroring the schema's SFXNotSupported response.
//
// The endpoint shares the AUDIO_ENABLED toggle with /audio/speech.
func (router *RouterImpl) SFXHandler(w http.ResponseWriter, r *http.Request) {
	if !router.audioEnabled(w) {
		return
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeMultipart) {
		router.serveSFXFromVideo(w, r)
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      sfxAPIName,
		exampleModel: "elevenlabs/eleven_text_to_sound_v2",
		endpointOf:   func(e types.Endpoints) *string { return e.SFX },
		notSupported: sfxNotSupportedMessage,
		translators: map[types.Provider]jsonTranslator{
			constants.ElevenlabsID: elevenlabsSFX,
			constants.MireloID:     mireloSFX,
		},
		responders: map[types.Provider]upstreamResponder{constants.MireloID: (*RouterImpl).relayMireloAudio},
	})
}

// mireloSFX rewrites a gateway CreateSFXRequest into the Mirelo text-to-sfx
// shape.
func mireloSFX(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateSFXRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	out, err := sfx.FromText(model, req)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, "", out, nil
}

// Sound-effect constants.
const (
	audioMedia                  = "audio"
	sfxAPIName                  = "Sound effect generation"
	sfxNotSupportedMessage      = "Sound effect generation is not supported by this provider yet."
	sfxVideoNotSupportedMessage = "Video scoring is not supported by this provider yet."
	sfxVideoExampleModel        = "mirelo/sfx-2.0"

	contentTypeMultipart  = "multipart/form-data"
	mireloNotReadyMessage = "The sound effect was not ready in time. Retry the request or ask for a shorter clip."
	uploadFailedMessage   = "Failed to upload the video to the provider"

	// trackArchiveMaxSize caps how much of an archive of separated audio
	// tracks is read to answer with one of them.
	trackArchiveMaxSize = 256 << 20
)

// relayMireloAudio answers a text-to-sfx request with the mix of its job.
func (router *RouterImpl) relayMireloAudio(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest) {
	router.relaySFXJob(w, r, provider, req, sfx.OutputMix)
}

// relaySFXJob turns Mirelo's asynchronous job into the raw audio bytes
// /v1/audio/sfx answers with: it creates the job, long-polls it to a final
// state and streams the wanted track of it. A create refused because the
// uploaded asset is still settling is retried within the same budget.
// shortcut: no sleep between polls, it relies on Mirelo holding ?wait= open - add a Retry-After sleep if it ever answers early.
// shortcut: a poll failure is terminal and the client re-requests - add in-budget retries only if transient poll failures show up in practice.
func (router *RouterImpl) relaySFXJob(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest, output string) {
	createPath := req.endpointPath
	plan := sfx.Plan(router.cfg.Server.ReadTimeout, router.responseHeaderTimeout())
	req.query = plan.Query
	req.accept = contentTypeJSON
	payload, err := io.ReadAll(req.body)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request body"})
		return
	}
	req.body = bytes.NewReader(payload)

	for range plan.Requests {
		router.extendWriteDeadline(w)
		job, outcome, wait := router.mireloJob(w, r, provider, req)
		switch outcome {
		case jobAnswered:
			return
		case jobAssetNotReady:
			if !pause(r, wait) {
				return
			}
			req.body = bytes.NewReader(payload)
			continue
		case jobReady:
		}
		if job.Pending() {
			req = upstreamRequest{method: http.MethodGet, endpointPath: sfx.JobPath(createPath, job.ID), query: req.query, accept: contentTypeJSON}
			continue
		}

		audioURL, contentType, err := job.Audio(output)
		if err != nil {
			router.logger.Error("mirelo job did not produce audio", err, "job_id", job.ID)
			middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
			return
		}
		router.extendWriteDeadline(w)
		router.streamDownload(w, r, audioURL, audioMedia, contentType)
		return
	}

	router.logger.Error("mirelo job still pending after the last poll", nil, "requests", plan.Requests)
	middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: mireloNotReadyMessage})
}

// responseHeaderTimeout is how long the provider client waits for response
// headers, the ceiling a long-poll must stay under. Zero when not configured.
func (router *RouterImpl) responseHeaderTimeout() time.Duration {
	if router.cfg.Client == nil {
		return 0
	}
	return router.cfg.Client.ClientResponseHeaderTimeout
}

// extendWriteDeadline gives the response one more upstream round trip plus the
// time to write it, so a job polled over several requests outlives WRITE_TIMEOUT.
func (router *RouterImpl) extendWriteDeadline(w http.ResponseWriter) {
	middlewares.ResetWriteDeadline(w, router.cfg.Server.ReadTimeout+router.cfg.Server.WriteTimeout)
}

// jobOutcome says what mireloJob did with the client response.
type jobOutcome int

const (
	jobReady jobOutcome = iota
	jobAnswered
	jobAssetNotReady
)

// mireloJob sends req to Mirelo and decodes the job it answers with. Failures
// and upstream error responses are already written to w (jobAnswered), except
// a refusal because the uploaded asset is still settling, which is reported
// with the pause Mirelo asks for so the caller can retry.
func (router *RouterImpl) mireloJob(w http.ResponseWriter, r *http.Request, provider core.IProvider, req upstreamRequest) (sfx.Job, jobOutcome, time.Duration) {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return sfx.Job{}, jobAnswered, 0
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read mirelo job response", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return sfx.Job{}, jobAnswered, 0
	}
	if sfx.AssetNotReady(resp.StatusCode, body) {
		return sfx.Job{}, jobAssetNotReady, sfx.RetryAfter(resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return sfx.Job{}, jobAnswered, 0
	}

	job, err := sfx.ParseJob(body)
	if err != nil {
		router.logger.Error("failed to decode mirelo job response", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return sfx.Job{}, jobAnswered, 0
	}
	return job, jobReady, 0
}

// pause waits for d unless the client leaves first.
func pause(r *http.Request, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-r.Context().Done():
		return false
	}
}

// serveSFXFromVideo answers a multipart /v1/audio/sfx request: the uploaded
// video is scored by the provider and the wanted track is streamed back as raw
// audio. Providers without an asset-and-job flow are served by their own
// scorer.
func (router *RouterImpl) serveSFXFromVideo(w http.ResponseWriter, r *http.Request) {
	form, ok := router.parseBoundedMultipart(w, r)
	if !ok {
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	originalModel := formValue(form, formFieldModel)
	providerID, model, msg := router.resolveProvider(r, originalModel, sfxVideoExampleModel)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}
	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}
	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}
	createPath, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.SFXVideo }, sfxAPIName, sfxVideoNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}
	videos := form.File[formFieldVideo]
	if len(videos) == 0 {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The '%s' file is required to score a video.", formFieldVideo)})
		return
	}
	if score, translated := sfxVideoScorers[providerID]; translated {
		score(router, w, r, provider, model, createPath, form)
		return
	}

	assetsPath, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.Assets }, sfxAPIName, sfxVideoNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	assetID, ok := router.uploadAsset(w, r, provider, assetsPath, videos[0])
	if !ok {
		return
	}
	body, err := sfx.FromVideo(model, form, assetID)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", sfxAPIName, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	router.relaySFXJob(w, r, provider, upstreamRequest{endpointPath: createPath, body: bytes.NewReader(body), contentType: contentTypeJSON}, formValue(form, sfx.FieldOutput))
}

// sfxVideoScorers score a video for the providers whose video-to-sfx API is
// not an asset upload followed by a job, in place of the default flow.
var sfxVideoScorers = map[types.Provider]func(router *RouterImpl, w http.ResponseWriter, r *http.Request, provider core.IProvider, model, endpointPath string, form *multipart.Form){
	constants.ElevenlabsID: (*RouterImpl).elevenlabsSFXFromVideo,
}

// elevenlabsSFXFromVideo scores a video through ElevenLabs' video-to-music
// endpoint, which takes the clips and answers the finished audio in one call.
// Asking for the sound effects alone answers an archive of separate tracks,
// which the music track is dropped from here.
// shortcut: the score must finish inside SERVER_READ_TIMEOUT, which the operator raises for long clips - give the call its own window if that proves too coarse.
func (router *RouterImpl) elevenlabsSFXFromVideo(w http.ResponseWriter, r *http.Request, provider core.IProvider, model, endpointPath string, form *multipart.Form) {
	music, err := elevenlabs.SFXFromVideo(model, form)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", sfxAPIName, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		pw.CloseWithError(writeVideoMusicForm(mw, music.Fields, form.File[formFieldVideo]))
	}()

	router.extendWriteDeadline(w)
	resp, done, failure := router.callUpstream(r, provider, upstreamRequest{endpointPath: endpointPath, query: music.Query, body: pr, contentType: mw.FormDataContentType()})
	if failure != nil {
		_ = pr.CloseWithError(io.ErrClosedPipe)
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return
	}
	defer done()
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
		if readErr != nil {
			router.logger.Error("failed to read the elevenlabs video-to-music response", readErr)
			middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
			return
		}
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return
	}
	if !music.SeparateTracks {
		writeDataFromReader(w, resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body)
		return
	}

	archive, err := io.ReadAll(io.LimitReader(resp.Body, trackArchiveMaxSize))
	if err != nil {
		router.logger.Error("failed to read the elevenlabs track archive", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return
	}
	track, err := elevenlabs.EffectsTrack(archive)
	if err != nil {
		router.logger.Error("failed to unpack the elevenlabs sound-effects track", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return
	}
	writeData(w, http.StatusOK, music.ContentType, track)
}

// writeVideoMusicForm writes the video-to-music upload: the request's fields,
// then every clip to score as a repeated part, announced with the container
// type the provider needs to decode it.
func writeVideoMusicForm(mw *multipart.Writer, fields map[string]string, videos []*multipart.FileHeader) error {
	for name, value := range fields {
		if err := mw.WriteField(name, value); err != nil {
			return err
		}
	}
	for _, video := range videos {
		content, contentType, file, err := openFormFile(video)
		if err != nil {
			return err
		}
		err = writeFilePart(mw, elevenlabs.VideoClipPart, video.Filename, sfx.VideoContentType(video.Filename, contentType), content)
		_ = file.Close()
		if err != nil {
			return err
		}
	}
	return mw.Close()
}

// uploadAsset stores the uploaded video with the provider and returns the
// asset id a generation can reference. The bytes go to the signed URL the
// provider's upload ticket names, without the API key.
func (router *RouterImpl) uploadAsset(w http.ResponseWriter, r *http.Request, provider core.IProvider, assetsPath string, video *multipart.FileHeader) (string, bool) {
	content, contentType, file, err := openFormFile(video)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read the uploaded video"})
		return "", false
	}
	defer file.Close()
	contentType = sfx.VideoContentType(video.Filename, contentType)

	ticket, ok := router.assetTicket(w, r, provider, assetsPath, contentType)
	if !ok {
		return "", false
	}
	target, usable := downloadURL(ticket.UploadURL)
	if !usable {
		router.logger.Error("provider returned an unusable upload url", nil, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "The provider returned an unusable upload URL."})
		return "", false
	}

	router.extendWriteDeadline(w)
	ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	body, formType, length, err := sfx.UploadBody(ticket, video.Filename, contentType, content, video.Size)
	if err != nil {
		router.logger.Error("failed to build the upload form", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), body)
	if err != nil {
		router.logger.Error("failed to create upload request", err, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return "", false
	}
	req.Header.Set("Content-Type", formType)
	req.ContentLength = length
	resp, err := router.client.Do(req)
	if err != nil {
		router.logger.Error("failed to upload the video to the provider", err, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: uploadFailedMessage})
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		router.logger.Error("the provider refused the video upload", nil, "status", resp.StatusCode, "url", ticket.UploadURL)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: uploadFailedMessage})
		return "", false
	}
	return ticket.ID, true
}

// assetTicket asks the provider where to upload a file of contentType.
// Failures are already written to w, reported by a false ok.
func (router *RouterImpl) assetTicket(w http.ResponseWriter, r *http.Request, provider core.IProvider, assetsPath, contentType string) (sfx.AssetTicket, bool) {
	resp, done, failure := router.callUpstream(r, provider, upstreamRequest{endpointPath: assetsPath, body: bytes.NewReader(sfx.AssetRequest(contentType)), contentType: contentTypeJSON, accept: contentTypeJSON})
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return sfx.AssetTicket{}, false
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read mirelo upload ticket", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return sfx.AssetTicket{}, false
	}
	if resp.StatusCode >= http.StatusBadRequest {
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return sfx.AssetTicket{}, false
	}
	ticket, err := sfx.ParseAssetTicket(body)
	if err != nil {
		router.logger.Error("failed to decode mirelo upload ticket", err)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return sfx.AssetTicket{}, false
	}
	return ticket, true
}

// elevenlabsSFX rewrites a gateway CreateSFXRequest into the ElevenLabs
// sound-generation shape.
func elevenlabsSFX(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateSFXRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	out, err := elevenlabs.SFX(model, req)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, "", out, nil
}
