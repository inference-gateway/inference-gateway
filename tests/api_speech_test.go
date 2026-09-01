package tests

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	logger "github.com/inference-gateway/inference-gateway/logger"
)

func enableAudio(c *config.Config) { c.AudioEnabled = true }

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

// TestSpeechHandler_VoiceCloningPassthrough proves the optional
// reference_audio cloning field reaches a speech-capable provider (llamacpp)
// byte-for-byte alongside the model-prefix rewrite.
func TestSpeechHandler_VoiceCloningPassthrough(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("FAKE-CLONED-AUDIO"))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	sample := base64.StdEncoding.EncodeToString([]byte("RIFF-fake-wav-sample"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(
		`{"model":"llamacpp/qwen3-tts","input":"Hello","voice":"custom","response_format":"wav","reference_audio":"`+sample+`"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "qwen3-tts", gotBody["model"], "provider prefix should be stripped")
	assert.Equal(t, sample, gotBody["reference_audio"], "reference audio should be forwarded untouched")
	assert.Equal(t, "FAKE-CLONED-AUDIO", w.Body.String())
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
	assert.Equal(t, `{"error":"The Audio API is not enabled. Set AUDIO_ENABLED=true to enable it."}`, w.Body.String())
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

// newLocalSpeechRouter builds a router wired to an in-memory tts.Engine with
// placeholder model files so the local speech path works without the proxy
// stack (registry/client stay nil: local/ requests never touch providers).
func newLocalSpeechRouter(t *testing.T, home string, opts ...func(*config.Config)) api.Router {
	t.Helper()
	log, err := logger.NewLogger("test")
	require.NoError(t, err)
	engine := tts.NewEngine(log, tts.Config{AutoDownload: false, Home: home})
	cfg := config.Config{
		AudioEnabled: true,
		Server:       &config.ServerConfig{ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return api.NewRouter(cfg, log, nil, nil, nil, nil, nil, engine)
}

// seedLocalSpeechModels plants placeholder GGUF weights in the engine's cache.
func seedLocalSpeechModels(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, tts.CacheModelsDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, tts.BackboneGGUF), []byte("backbone"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, tts.MmprojGGUF), []byte("mmproj"), 0o644))
}

// installFakeLlamaTTS puts a sh script first on PATH that accepts the
// llama-tts CLI shape and "synthesizes" the caller-chosen body into --output.
func installFakeLlamaTTS(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake llama-tts binary is a sh script")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "llama-tts"), []byte("#!/bin/sh\n"+body), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSpeechHandler_LocalModelReady(t *testing.T) {
	home := t.TempDir()
	seedLocalSpeechModels(t, home)
	installFakeLlamaTTS(t, `out=""; prev=""
for a in "$@"; do
  [ "$prev" = "--output" ] && out="$a"
  prev="$a"
done
printf 'FAKE-LOCAL-WAV' > "$out"
`)
	router := newLocalSpeechRouter(t, home)
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(`{"model":"local/qwen3-tts","input":"Hello there"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "audio/wav", w.Header().Get("Content-Type"))
	assert.Equal(t, "FAKE-LOCAL-WAV", w.Body.String())
}

func TestSpeechHandler_LocalModelNotReadyReturns503(t *testing.T) {
	router := newLocalSpeechRouter(t, t.TempDir())
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(`{"model":"local/qwen3-tts","input":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "10", w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), "not ready")
}

func postLocalSpeech(router api.Router, body string) *httptest.ResponseRecorder {
	r := gin.New()
	r.POST("/v1/audio/speech", router.SpeechHandler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSpeechHandler_LocalUnknownModelRejected(t *testing.T) {
	home := t.TempDir()
	seedLocalSpeechModels(t, home)
	router := newLocalSpeechRouter(t, home)

	w := postLocalSpeech(router, `{"model":"local/other-tts","input":"Hello"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), tts.ReservedModelID)
}

func TestSpeechHandler_LocalNonWavFormatRejected(t *testing.T) {
	home := t.TempDir()
	seedLocalSpeechModels(t, home)
	router := newLocalSpeechRouter(t, home)

	w := postLocalSpeech(router, `{"model":"local/qwen3-tts","input":"Hello","response_format":"mp3"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "only supports response_format")
}

func TestSpeechHandler_LocalEmptyInputRejected(t *testing.T) {
	home := t.TempDir()
	seedLocalSpeechModels(t, home)
	router := newLocalSpeechRouter(t, home)

	w := postLocalSpeech(router, `{"model":"local/qwen3-tts"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "input")
}

func TestSpeechHandler_LocalModelDenied(t *testing.T) {
	home := t.TempDir()
	seedLocalSpeechModels(t, home)
	router := newLocalSpeechRouter(t, home, func(c *config.Config) { c.DisallowedModels = tts.ReservedModelID })

	w := postLocalSpeech(router, `{"model":"local/qwen3-tts","input":"Hello"}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "disallowed")
}
