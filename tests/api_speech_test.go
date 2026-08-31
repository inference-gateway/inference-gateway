package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"

	config "github.com/inference-gateway/inference-gateway/config"
)

func enableAudio(c *config.Config) { c.EnableAudio = true }

func TestSpeechHandler_HappyPath(t *testing.T) {
	var gotPath, gotAuth, gotRequestContentType string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotRequestContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("FAKE-AUDIO-BYTES"))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(`{"model":"openai/tts-1","input":"Hello world","voice":"alloy","speed":1.25}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/audio/speech", gotPath)
	assert.Equal(t, "tts-1", gotBody["model"], "provider prefix should be stripped")
	assert.Equal(t, "Hello world", gotBody["input"], "input should be forwarded untouched")
	assert.Equal(t, "alloy", gotBody["voice"])
	assert.Equal(t, float64(1.25), gotBody["speed"])
	assert.Equal(t, "Bearer test-openai-key", gotAuth, "provider token should be applied")
	assert.Equal(t, "application/json", gotRequestContentType)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"), "audio should be streamed with the upstream content type")
	assert.Equal(t, "FAKE-AUDIO-BYTES", w.Body.String())
}

func TestSpeechHandler_DisabledByDefault(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", false)
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(`{"model":"openai/tts-1","input":"Hello","voice":"alloy"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, `{"error":"The Audio API is not enabled. Set ENABLE_AUDIO=true to enable it."}`, w.Body.String())
}

func TestSpeechHandler_UnsupportedProvider(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", false, enableAudio)
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech?provider=cohere", strings.NewReader(`{"model":"tts-1","input":"Hello","voice":"alloy"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, `{"error":"The Audio API is not supported by this provider yet."}`, w.Body.String())
}

func TestSpeechHandler_NoProvider(t *testing.T) {
	router := newImagesTestRouter(t, "http://unused", false, enableAudio)
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(`{"input":"Hello","voice":"alloy"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "No provider specified")
}
