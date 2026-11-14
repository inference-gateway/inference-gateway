package api

import (
	"context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ImagesHandler implements an OpenAI-compatible POST /v1/images/generations
// endpoint: https://platform.openai.com/docs/api-reference/images/create
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Images API fields pass through untouched.
//
// Only providers that natively implement the Images API are supported
// (currently OpenAI); other providers receive a 400, mirroring the schema's
// ImagesNotSupported response.
//
// The endpoint is opt-in via IMAGES_ENABLED (default off). When disabled, the
// handler returns 404.
func (router *RouterImpl) ImagesHandler(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      "Images API",
		exampleModel: "openai/gpt-image-2",
		accept:       contentTypeJSON,
		endpointOf:   func(e types.Endpoints) *string { return e.Images },
	})
}

// Multipart form field names used by the /images/edits endpoint.
const (
	imageFormFieldImage = "image"
	// imageFormFieldImageArray is the multi-image variant accepted by
	// gpt-image-1 edits.
	imageFormFieldImageArray = "image[]"
	imageFormFieldPrompt     = "prompt"

	imageFormFieldResponseFormat = "response_format"
	imageResponseFormatBase64    = "b64_json"

	// imagesAPIName labels the Images API in logs and errors.
	imagesAPIName = "Images API"

	// imageGenerationWindow is how long a provider that renders an image as a
	// job may take before the request gives up.
	imageGenerationWindow = 5 * time.Minute

	// imageMaxDownloadSize caps how much of a generated image is read to
	// answer with it as base64.
	imageMaxDownloadSize = 64 << 20
)

// ImagesEditsHandler implements POST /v1/images/edits (multipart/form-data).
func (router *RouterImpl) ImagesEditsHandler(w http.ResponseWriter, r *http.Request) {
	router.handleImagesMultipart(w, r)
}

// handleImagesMultipart proxies a multipart Images upload to the resolved
// provider. It parses the form to validate required fields and resolve the
// provider (via ?provider= or the model prefix), then re-encodes the parts and
// streams them to the upstream through an io.Pipe so the binary image/mask
// files are streamed to the upstream without a second full in-memory copy
// (ParseMultipartForm has already spilled anything over 1 MiB to temp files).
//
// Behaviour mirrors ImagesHandler: opt-in via IMAGES_ENABLED (404 when off) and
// only providers that natively implement the endpoint are supported (others
// receive a 400).
func (router *RouterImpl) handleImagesMultipart(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
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

	if len(form.File[imageFormFieldImage])+len(form.File[imageFormFieldImageArray]) == 0 {
		router.logger.Error("images request missing image file", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The 'image' file is required."})
		return
	}
	if strings.TrimSpace(formValue(form, imageFormFieldPrompt)) == "" {
		router.logger.Error("images edit request missing prompt", nil)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The 'prompt' field is required."})
		return
	}

	model := formValue(form, formFieldModel)
	originalModel := model

	providerID, model, msg := router.resolveProvider(r, model, "openai/gpt-image-2")
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

	endpoint := provider.GetEndpoints().ImagesEdits
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("images api not supported by provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The Images API is not supported by this provider yet."})
		return
	}

	if edit, translated := imageEditors[providerID]; translated {
		edit(router, w, r, provider, model, form)
		return
	}

	if model != originalModel {
		form.Value[formFieldModel] = []string{model}
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		pw.CloseWithError(writeMultipartForm(mw, form))
	}()

	if f := router.forwardUpstream(w, r, provider, upstreamRequest{
		endpointPath: *endpoint,
		body:         pr,
		contentType:  mw.FormDataContentType(),
		accept:       contentTypeJSON,
	}); f != nil {
		_ = pr.CloseWithError(io.ErrClosedPipe)
		middlewares.WriteJSON(w, f.status, ErrorResponse{Error: f.message})
		return
	}
}

// imageEditors serve an image edit for the providers that have no OpenAI-shaped
// Images API, in place of the multipart passthrough.
var imageEditors = map[types.Provider]func(router *RouterImpl, w http.ResponseWriter, r *http.Request, provider core.IProvider, model string, form *multipart.Form){
	constants.ElevenlabsID: (*RouterImpl).elevenlabsImageEdit,
}

// elevenlabsImageEdit serves an image edit through an ElevenLabs image
// generation and answers in the OpenAI Images shape. The request stays open
// while ElevenLabs renders, so the write deadline covers that wait.
func (router *RouterImpl) elevenlabsImageEdit(w http.ResponseWriter, r *http.Request, provider core.IProvider, model string, form *multipart.Form) {
	generation, err := elevenlabs.ImageEdit(model, form)
	if err != nil {
		router.logger.Error("failed to translate request for provider", err, "api", imagesAPIName, "provider", provider.GetName())
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	middlewares.ResetWriteDeadline(w, imageGenerationWindow+router.cfg.Server.WriteTimeout)
	ctx, cancel := context.WithTimeout(r.Context(), imageGenerationWindow)
	defer cancel()

	account := elevenlabsAccount{router: router, request: r, provider: provider}
	generated, err := elevenlabs.GenerateImage(ctx, account, generation)
	if err != nil {
		router.logger.Error("failed to generate the image", err, "api", imagesAPIName, "provider", provider.GetName())
		failure := elevenlabsFailure(err)
		middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
		return
	}

	image := types.Image{URL: &generated.URL}
	if formValue(form, imageFormFieldResponseFormat) == imageResponseFormatBase64 {
		encoded, failure := router.base64Image(ctx, generated.URL)
		if failure != nil {
			middlewares.WriteJSON(w, failure.status, ErrorResponse{Error: failure.message})
			return
		}
		image = types.Image{B64Json: &encoded}
	}
	middlewares.WriteJSON(w, http.StatusOK, types.ImagesResponse{Created: time.Now().Unix(), Data: []types.Image{image}})
}

// base64Image downloads a generated image from the provider-supplied URL and
// returns it base64-encoded. The URL is already signed, so no credential is
// attached.
func (router *RouterImpl) base64Image(ctx context.Context, imageURL string) (string, *upstreamFailure) {
	parsed, usable := downloadURL(imageURL)
	if !usable {
		router.logger.Error("provider returned an unusable image download url", nil, "url", imageURL)
		return "", &upstreamFailure{http.StatusBadGateway, "The provider returned an unusable image download URL."}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		router.logger.Error("failed to create image download request", err, "url", imageURL)
		return "", &upstreamFailure{http.StatusInternalServerError, "Failed to create upstream request"}
	}

	resp, err := router.client.Do(req)
	if err != nil {
		router.logger.Error("failed to download generated image", err, "url", imageURL)
		return "", &upstreamFailure{http.StatusBadGateway, "Failed to download the generated image"}
	}
	defer resp.Body.Close()

	image, err := io.ReadAll(io.LimitReader(resp.Body, imageMaxDownloadSize))
	if err != nil || resp.StatusCode != http.StatusOK {
		router.logger.Error("failed to read generated image", err, "url", imageURL, "status", resp.StatusCode)
		return "", &upstreamFailure{http.StatusBadGateway, "Failed to download the generated image"}
	}
	return base64.StdEncoding.EncodeToString(image), nil
}
