package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	gin "github.com/gin-gonic/gin"

	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func newImagesTestRouter(t *testing.T, upstreamURL string, enableImages bool) api.Router {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockClient := providersmocks.NewMockClient(ctrl)
	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}).
		AnyTimes()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:        constants.OpenaiID,
			Name:      constants.OpenaiDisplayName,
			URL:       upstreamURL,
			Token:     "test-openai-key",
			AuthType:  constants.AuthTypeBearer,
			Endpoints: registry.Registry[constants.OpenaiID].Endpoints,
		},
		constants.CohereID: {
			ID:       constants.CohereID,
			Name:     constants.CohereDisplayName,
			URL:      upstreamURL,
			Token:    "test-cohere-key",
			AuthType: constants.AuthTypeBearer,
		},
	}

	cfg := config.Config{
		EnableImages: enableImages,
		Server: &config.ServerConfig{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		Providers: providerCfg,
	}

	return api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), mockClient, nil, nil, nil)
}

// imagesMultipartField is one part of a multipart image request; a non-empty
// filename makes it a file part.
type imagesMultipartField struct {
	name     string
	filename string
	value    string
}

func buildImagesMultipart(t *testing.T, fields []imagesMultipartField) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for _, f := range fields {
		if f.filename != "" {
			w, err := mw.CreateFormFile(f.name, f.filename)
			require.NoError(t, err)
			_, err = io.WriteString(w, f.value)
			require.NoError(t, err)
			continue
		}
		require.NoError(t, mw.WriteField(f.name, f.value))
	}
	require.NoError(t, mw.Close())
	return body, mw.FormDataContentType()
}

func TestImagesEditsHandler_HappyPath(t *testing.T) {
	var gotPath string
	var gotModel, gotPrompt, gotImage, gotMask string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotModel = r.FormValue("model")
		gotPrompt = r.FormValue("prompt")
		gotImage = readUploadedFile(t, r, "image")
		gotMask = readUploadedFile(t, r, "mask")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1730000000,"data":[{"url":"https://example.com/generated.png"}]}`))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, true)
	r := gin.New()
	r.POST("/v1/images/edits", router.ImagesEditsHandler)

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "image", filename: "sunset.png", value: "PNG-IMAGE-BYTES"},
		{name: "mask", filename: "mask.png", value: "PNG-MASK-BYTES"},
		{name: "prompt", value: "Add a flock of birds"},
		{name: "model", value: "openai/gpt-image-1"},
		{name: "n", value: "1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/images/edits", gotPath)
	assert.Equal(t, "gpt-image-1", gotModel, "provider prefix should be stripped")
	assert.Equal(t, "Add a flock of birds", gotPrompt)
	assert.Equal(t, "PNG-IMAGE-BYTES", gotImage, "image file should be forwarded")
	assert.Equal(t, "PNG-MASK-BYTES", gotMask, "mask file should be forwarded")
	assert.Equal(t, "Bearer test-openai-key", gotAuth, "provider token should be applied")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1730000000), resp["created"])
}

func TestImagesVariationsHandler_HappyPath(t *testing.T) {
	var gotPath, gotModel, gotImage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotModel = r.FormValue("model")
		gotImage = readUploadedFile(t, r, "image")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1730000000,"data":[{"b64_json":"aGk="}]}`))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, true)
	r := gin.New()
	r.POST("/v1/images/variations", router.ImagesVariationsHandler)

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "image", filename: "sunset.png", value: "PNG-IMAGE-BYTES"},
		{name: "model", value: "openai/dall-e-2"},
		{name: "n", value: "2"},
		{name: "response_format", value: "b64_json"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/images/variations", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/images/variations", gotPath)
	assert.Equal(t, "dall-e-2", gotModel)
	assert.Equal(t, "PNG-IMAGE-BYTES", gotImage)
}

func TestImagesEditsHandler_MissingImage(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", true)
	r := gin.New()
	r.POST("/v1/images/edits", router.ImagesEditsHandler)

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "prompt", value: "Add a flock of birds"},
		{name: "model", value: "openai/gpt-image-1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/images/edits?provider=openai", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "image")
}

func TestImagesEditsHandler_MissingPrompt(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", true)
	r := gin.New()
	r.POST("/v1/images/edits", router.ImagesEditsHandler)

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "image", filename: "sunset.png", value: "PNG-IMAGE-BYTES"},
		{name: "model", value: "openai/gpt-image-1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "prompt")
}

func TestImagesVariationsHandler_UnsupportedProvider(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", true)
	r := gin.New()
	r.POST("/v1/images/variations", router.ImagesVariationsHandler)

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "image", filename: "sunset.png", value: "PNG-IMAGE-BYTES"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/images/variations?provider=cohere", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not supported")
}

func TestImagesEditsHandler_Disabled(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", false)
	r := gin.New()
	r.POST("/v1/images/edits", router.ImagesEditsHandler)

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "image", filename: "sunset.png", value: "PNG-IMAGE-BYTES"},
		{name: "prompt", value: "Add a flock of birds"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/images/edits?provider=openai", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func readUploadedFile(t *testing.T, r *http.Request, field string) string {
	t.Helper()
	f, _, err := r.FormFile(field)
	if err == http.ErrMissingFile {
		return ""
	}
	require.NoError(t, err)
	defer f.Close()
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(b)
}
