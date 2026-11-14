package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	lipsync "github.com/inference-gateway/inference-gateway/internal/lipsync"
	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Videos API constants.
const (
	// videosExampleModel is the routing hint used in client-facing Videos errors.
	videosExampleModel = "elevenlabs/creatify-aurora"

	// videosNotSupportedMessage mirrors the schema's VideosNotSupported response.
	videosNotSupportedMessage = "The Videos API is not supported by this provider yet."

	// videosAPIName labels the Videos API in logs and errors.
	videosAPIName = "Videos API"

	// videoMedia names the media kind in download errors.
	videoMedia = "video"

	// videoJobIDSeparator joins the provider to the upstream job id in the video
	// id handed back to clients. Models use "provider/model", but a video id has
	// to survive as a single path segment of GET /v1/videos/{video_id}, so this
	// one cannot be a slash.
	videoJobIDSeparator = ":"

	// videoJobIDHint explains how to route a per-job Videos request.
	videoJobIDHint = "Please specify a provider using the ?provider= query parameter or pass the video id returned by POST /v1/videos (e.g., elevenlabs" +
		videoJobIDSeparator + "gen_abc123)."
)

// videoJobMapper maps a provider's job payload onto the schema's VideoJob and
// returns the URL the rendered video can be downloaded from, empty while the
// render is unfinished. createdAt is the fallback timestamp for payloads that do
// not carry one and fallbackModel the model to report when it is not echoed back.
type videoJobMapper func(raw []byte, fallbackModel string, createdAt int) (types.VideoJob, string, error)

// videoJobMappers holds the per-provider response mappings. Every provider that
// carries a Videos endpoint in the registry needs an entry here; there is no
// OpenAI-shaped passthrough because no provider answers with a schema-shaped
// VideoJob yet.
var videoJobMappers = map[types.Provider]videoJobMapper{
	constants.ElevenlabsID: elevenlabs.Job,
	constants.MireloID:     sfx.VideoJob,
}

// videoRequestTranslators rewrite the OpenAI Videos multipart form into each
// provider's native create-job payload. No provider speaks the OpenAI shape,
// so every provider that carries a Videos endpoint must have an entry here.
// assetID is the uploaded clip a dubbing provider renders from, empty for the
// providers that render from the form alone.
var videoRequestTranslators = map[types.Provider]func(model string, form *multipart.Form, assetID string) ([]byte, error){
	constants.ElevenlabsID: func(model string, form *multipart.Form, _ string) ([]byte, error) {
		return elevenlabs.Video(model, form)
	},
	constants.MireloID: sfx.Dub,
}

// VideosHandler implements POST /v1/videos, the OpenAI-compatible Videos API:
// https://platform.openai.com/docs/api-reference/videos/create
//
// Video generation is asynchronous at every provider, so the multipart request
// (prompt plus optional reference image and driving audio) is forwarded to the
// provider and the created job is returned immediately. Clients poll
// GET /v1/videos/{video_id} until the status is completed, then download the
// bytes from GET /v1/videos/{video_id}/content.
//
// The returned job id is prefixed with the provider that created it
// ("elevenlabs:abc123", see videoJobIDSeparator), because the gateway keeps no
// job state and has to route the follow-up calls somewhere.
//
// Only providers that carry a Videos endpoint (currently elevenlabs) can serve
// the request; the rest receive a 400, mirroring the schema's VideosNotSupported
// response. The endpoint is opt-in via VIDEOS_ENABLED (default off).
func (router *RouterImpl) VideosHandler(w http.ResponseWriter, r *http.Request) {
	if !router.videosEnabled(w) {
		return
	}

	form, ok := router.parseBoundedMultipart(w, r)
	if !ok {
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	model := formValue(form, formFieldModel)
	originalModel := model

	if strings.HasPrefix(model, tts.ModelPrefix) {
		router.serveLocalVideo(w, model, form)
		return
	}

	providerID, model, msg := router.resolveProvider(r, model, videosExampleModel)
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

	endpoint, msg := router.providerEndpoint(provider, func(e types.Endpoints) *string { return e.Videos }, videosAPIName, videosNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	assetID, ok := router.videoAsset(w, r, provider, form)
	if !ok {
		return
	}

	body, err := videoRequestTranslators[providerID](model, form, assetID)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", videosAPIName, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	job, _, ok := router.videoJob(w, r, provider, providerID, model, upstreamRequest{
		endpointPath: endpoint,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	job.ID = string(providerID) + videoJobIDSeparator + job.ID
	middlewares.WriteJSON(w, http.StatusOK, job)
}

// videoAsset uploads the clip of a video-in, video-out request as a provider
// asset the create job references. Providers whose registry entry carries no
// Assets endpoint render from the form alone and get an empty id. Failures are
// already written to w, reported by a false ok.
// shortcut: a create refused while the fresh asset settles is relayed as the provider's own status and the client retries - retry in the handler if that shows up in practice.
func (router *RouterImpl) videoAsset(w http.ResponseWriter, r *http.Request, provider core.IProvider, form *multipart.Form) (string, bool) {
	assets := provider.GetEndpoints().Assets
	if assets == nil || *assets == "" {
		return "", true
	}

	videos := form.File[formFieldVideo]
	if len(videos) == 0 {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The '%s' file is required to dub a video.", formFieldVideo)})
		return "", false
	}

	return router.uploadAsset(w, r, provider, *assets, videos[0])
}

// RetrieveVideoHandler implements GET /v1/videos/{video_id}. The gateway keeps
// no job state, so the request is forwarded to the provider encoded in the job
// id (or named by ?provider=) and the upstream payload is mapped back onto a
// VideoJob.
func (router *RouterImpl) RetrieveVideoHandler(w http.ResponseWriter, r *http.Request) {
	if id, local := strings.CutPrefix(r.PathValue("video_id"), localVideoIDPrefix); local {
		router.serveLocalVideoJob(w, id)
		return
	}

	provider, providerID, videoID, endpoint, ok := router.resolveVideoTarget(w, r, func(e types.Endpoints) *string { return e.VideosRetrieve })
	if !ok {
		return
	}

	job, _, ok := router.videoJob(w, r, provider, providerID, "", upstreamRequest{
		method:       http.MethodGet,
		endpointPath: strings.ReplaceAll(endpoint, generationIDPlaceholder, url.PathEscape(videoID)),
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	job.ID = string(providerID) + videoJobIDSeparator + job.ID
	middlewares.WriteJSON(w, http.StatusOK, job)
}

// DownloadVideoContentHandler implements GET /v1/videos/{video_id}/content. No
// provider serves the rendered bytes from its own API - the render lands on a
// signed CDN URL - so the job is retrieved first and the video is streamed from
// the download URL it carries. A job that is still queued, in progress or failed
// has no bytes to serve and returns 404, as the schema requires.
func (router *RouterImpl) DownloadVideoContentHandler(w http.ResponseWriter, r *http.Request) {
	if id, local := strings.CutPrefix(r.PathValue("video_id"), localVideoIDPrefix); local {
		router.serveLocalVideoContent(w, r, id)
		return
	}

	provider, providerID, videoID, endpoint, ok := router.resolveVideoTarget(w, r, func(e types.Endpoints) *string { return e.VideosRetrieve })
	if !ok {
		return
	}

	job, downloadURL, ok := router.videoJob(w, r, provider, providerID, "", upstreamRequest{
		method:       http.MethodGet,
		endpointPath: strings.ReplaceAll(endpoint, generationIDPlaceholder, url.PathEscape(videoID)),
		accept:       contentTypeJSON,
	})
	if !ok {
		return
	}

	if job.Status != types.VideoJobStatusCompleted || downloadURL == "" {
		router.logger.Warn("rendered video not available", "video_id", videoID, "status", job.Status)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: videoNotReadyMessage})
		return
	}

	router.streamDownload(w, r, downloadURL, videoMedia, "")
}

// resolveVideoTarget resolves the provider, upstream job id and endpoint for a
// per-job Videos request, writing the client response and reporting false on
// failure.
func (router *RouterImpl) resolveVideoTarget(w http.ResponseWriter, r *http.Request, endpointOf func(types.Endpoints) *string) (core.IProvider, types.Provider, string, string, bool) {
	if !router.videosEnabled(w) {
		return nil, "", "", "", false
	}

	providerID, videoID, msg := router.resolveVideoJobID(r, r.PathValue("video_id"))
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	endpoint, msg := router.providerEndpoint(provider, endpointOf, videosAPIName, videosNotSupportedMessage)
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return nil, "", "", "", false
	}

	return provider, providerID, videoID, endpoint, true
}

// resolveVideoJobID splits the gateway's "provider:job_id" video id into the
// provider and the upstream id that must be sent back verbatim. An explicit
// ?provider= wins over the prefix, mirroring model routing; the prefix is
// stripped either way. It returns the client-facing message ("" on success) when
// neither names a provider.
func (router *RouterImpl) resolveVideoJobID(r *http.Request, videoID string) (types.Provider, string, string) {
	providerID := types.Provider(r.URL.Query().Get("provider"))

	if prefix, rest, found := strings.Cut(videoID, videoJobIDSeparator); found {
		if detected := types.Provider(strings.ToLower(prefix)); registry.Registry[detected] != nil {
			videoID = rest
			if providerID == "" {
				providerID = detected
			}
		}
	}

	if providerID == "" {
		router.logger.Error("unable to determine provider for video job", nil, "video_id", videoID)
		return "", "", "Unable to determine provider for this video id. " + videoJobIDHint
	}
	if videoID == "" {
		router.logger.Error("video job request carries no id", nil)
		return "", "", "The video id is required."
	}

	return providerID, videoID, ""
}

// videoJob sends req to the provider and maps the payload onto a VideoJob,
// returning it alongside the URL its rendered video can be downloaded from
// (empty until the render completes). Failures and upstream error responses are
// already written to the response, reported by a false ok.
func (router *RouterImpl) videoJob(w http.ResponseWriter, r *http.Request, provider core.IProvider, providerID types.Provider, fallbackModel string, req upstreamRequest) (types.VideoJob, string, bool) {
	resp, done, failure := router.callUpstream(r, provider, req)
	if failure != nil {
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return types.VideoJob{}, "", false
	}
	defer done()
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jobMaxResponseSize))
	if err != nil {
		router.logger.Error("failed to read video job response", err, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to read the upstream response"})
		return types.VideoJob{}, "", false
	}

	// Upstream errors are relayed verbatim so the provider's own explanation
	// (quota, unsupported size, unknown job) reaches the caller unaltered.
	if resp.StatusCode >= http.StatusBadRequest {
		writeData(w, resp.StatusCode, resp.Header.Get("Content-Type"), body)
		return types.VideoJob{}, "", false
	}

	mapper := videoJobMappers[providerID]
	if mapper == nil {
		router.logger.Error("no video job mapping for provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: videosNotSupportedMessage})
		return types.VideoJob{}, "", false
	}

	job, downloadURL, err := mapper(body, fallbackModel, int(time.Now().Unix()))
	if err != nil {
		router.logger.Error("failed to map video job response", err, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return types.VideoJob{}, "", false
	}

	return job, downloadURL, true
}

// Local video constants.
const (
	// localVideoIDPrefix marks the video id of a render the built-in engine
	// holds, where a provider's id carries the provider.
	localVideoIDPrefix = "local" + videoJobIDSeparator

	videoFormFieldReference = "input_reference"
	videoFormFieldAudio     = "audio"

	videoNotReadyMessage = "The rendered video is not available yet. Poll GET /v1/videos/{video_id} until the status is completed."
)

// serveLocalVideo queues a lip-sync render on the built-in engine for a
// request that names a local/ model, and answers with its job.
func (router *RouterImpl) serveLocalVideo(w http.ResponseWriter, model string, form *multipart.Form) {
	if model != lipsync.ReservedModelID {
		router.logger.Error("unknown local video model", nil, "model", model)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown local video model %q. %q is the only local video model.", model, lipsync.ReservedModelID)})
		return
	}
	if reason := router.modelDenied(model); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	portraits, clips := form.File[videoFormFieldReference], form.File[videoFormFieldAudio]
	if len(portraits) == 0 || len(clips) == 0 {
		router.logger.Error("local video request missing its portrait or clip", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("%s lip-syncs a portrait to a clip: the '%s' image and the '%s' clip are both required.", lipsync.ReservedModelID, videoFormFieldReference, videoFormFieldAudio)})
		return
	}
	portrait, err := portraits[0].Open()
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read the uploaded portrait"})
		return
	}
	defer portrait.Close()
	clip, err := clips[0].Open()
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read the uploaded clip"})
		return
	}
	defer clip.Close()

	job, err := router.lipsync.Submit(
		lipsync.Upload{Name: portraits[0].Filename, Data: portrait},
		lipsync.Upload{Name: clips[0].Filename, Data: clip},
	)
	if err != nil {
		var notReady *lipsync.NotReadyError
		if errors.As(err, &notReady) {
			router.logger.Warn("local lip-sync engine not ready", nil, "detail", notReady.Error())
			w.Header().Set("Retry-After", strconv.Itoa(lipsync.RetryAfterSeconds))
			middlewares.WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: notReady.Error()})
			return
		}
		router.logger.Error("failed to queue the local render", err)
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to queue the local render"})
		return
	}

	job.ID = localVideoIDPrefix + job.ID
	middlewares.WriteJSON(w, http.StatusOK, job)
}

// localVideo returns a job the built-in engine holds with the path of its
// video, writing the 404 for an id it does not know. The engine keeps its jobs
// in memory, so a restart forgets them.
func (router *RouterImpl) localVideo(w http.ResponseWriter, id string) (types.VideoJob, string, bool) {
	if !router.videosEnabled(w) {
		return types.VideoJob{}, "", false
	}
	job, clip, known := router.lipsync.Job(id)
	if !known {
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Unknown local video id. Local renders are kept in memory, so a gateway restart forgets them."})
		return types.VideoJob{}, "", false
	}
	job.ID = localVideoIDPrefix + job.ID
	return job, clip, true
}

func (router *RouterImpl) serveLocalVideoJob(w http.ResponseWriter, id string) {
	if job, _, ok := router.localVideo(w, id); ok {
		middlewares.WriteJSON(w, http.StatusOK, job)
	}
}

func (router *RouterImpl) serveLocalVideoContent(w http.ResponseWriter, r *http.Request, id string) {
	job, clip, ok := router.localVideo(w, id)
	if !ok {
		return
	}
	if job.Status != types.VideoJobStatusCompleted || clip == "" {
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: videoNotReadyMessage})
		return
	}
	http.ServeFile(w, r, clip)
}
