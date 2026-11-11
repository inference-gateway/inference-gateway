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

	mirelo "github.com/inference-gateway/inference-gateway/internal/mirelo"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

// Fixtures shared by the Mirelo sound-effect handler tests.
const (
	mireloTestKey     = "test-mirelo-key"
	mireloSFXModel    = "sfx-2.0"
	mireloJobID       = "gen_abc123"
	mireloJobPath     = constants.MireloSFXEndpoint + "/" + mireloJobID
	mireloAudioPath   = "/signed/gen_abc123.mp3"
	mireloAudioBytes  = "FAKE-MIRELO-SFX-BYTES"
	mireloSFXRequest  = `{"model":"mirelo/` + mireloSFXModel + `","prompt":"distant thunder","duration_seconds":2,"loop":true}`
	mireloRunningJob  = `{"id":"` + mireloJobID + `","status":"running","errors":[],"result":null}`
	mireloFailedJob   = `{"id":"` + mireloJobID + `","status":"failed","errors":[{"code":"moderation_blocked","message":"the prompt was blocked"}],"result":null}`
	mireloAudioFormat = "mp3_128"
)

func mireloSucceededJob(audioURL string) string {
	return `{"id":"` + mireloJobID + `","status":"succeeded","errors":[],"result":{"output":{"variants":[` +
		`{"index":0,"status":"succeeded","files":{"audio":{"url":"` + audioURL + `","format":"` + mireloAudioFormat + `"}}}]}}}`
}

func postMireloSFX(t *testing.T, upstreamURL, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := newImagesTestRouter(t, upstreamURL, false, enableAudio)
	r := http.NewServeMux()
	r.HandleFunc("POST "+sfxPath, router.SFXHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", sfxPath, strings.NewReader(body))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)
	return w
}

// TestSFXHandler_Mirelo proves the asynchronous Mirelo job is turned into the
// raw audio bytes the endpoint promises: create, poll by id, then download the
// signed file without leaking the API key to it.
func TestSFXHandler_Mirelo(t *testing.T) {
	var createBody map[string]any
	var createQuery, createAuth, pollQuery, downloadAuth string
	polls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == constants.MireloSFXEndpoint:
			createQuery, createAuth = r.URL.RawQuery, r.Header.Get("Authorization")
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &createBody))
			w.Header().Set("Content-Type", contentTypeJSONVal)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(mireloRunningJob))
		case r.Method == http.MethodGet && r.URL.Path == mireloJobPath:
			polls++
			pollQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", contentTypeJSONVal)
			_, _ = w.Write([]byte(mireloSucceededJob(server.URL + mireloAudioPath)))
		case r.Method == http.MethodGet && r.URL.Path == mireloAudioPath:
			downloadAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(mireloAudioBytes))
		default:
			t.Errorf("unexpected upstream call %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	w := postMireloSFX(t, server.URL, mireloSFXRequest)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, mireloAudioBytes, w.Body.String())
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"), "the job's file format decides the Content-Type")
	assert.Equal(t, mireloSFXModel, createBody["model"], "the provider prefix must be stripped")
	assert.Equal(t, float64(2000), createBody["duration_ms"])
	assert.Equal(t, map[string]any{"prompt": "distant thunder"}, createBody["input"])
	assert.Equal(t, map[string]any{"loop": true}, createBody["controls"])
	assert.Equal(t, map[string]any{"format": mireloAudioFormat}, createBody["output"])
	assert.Equal(t, "Bearer "+mireloTestKey, createAuth)
	assert.True(t, strings.HasPrefix(createQuery, "wait="), createQuery)
	assert.Equal(t, createQuery, pollQuery)
	assert.Equal(t, 1, polls)
	assert.Empty(t, downloadAuth, "the signed download must not receive the provider key")
}

func TestSFXHandler_MireloFailedJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(mireloFailedJob))
	}))
	defer server.Close()

	w := postMireloSFX(t, server.URL, mireloSFXRequest)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "the prompt was blocked")
}

func TestSFXHandler_MireloJobNeverFinishes(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(mireloRunningJob))
	}))
	defer server.Close()

	w := postMireloSFX(t, server.URL, mireloSFXRequest)

	require.Equal(t, http.StatusGatewayTimeout, w.Code)
	assert.Equal(t, mirelo.MaxJobRequests, requests, "polling must be bounded")
}

func TestSFXHandler_MireloRelaysUpstreamError(t *testing.T) {
	const upstreamError = `{"error":{"code":"insufficient_credits","message":"not enough credits","retryable":false}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(upstreamError))
	}))
	defer server.Close()

	w := postMireloSFX(t, server.URL, mireloSFXRequest)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.JSONEq(t, upstreamError, w.Body.String())
}

func TestSFXHandler_MireloRejectsUntranslatableRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the request must be rejected before it reaches the provider")
	}))
	defer server.Close()

	for field, body := range map[string]string{
		"response_format":  `{"model":"mirelo/` + mireloSFXModel + `","prompt":"thunder","response_format":"pcm"}`,
		"prompt_influence": `{"model":"mirelo/` + mireloSFXModel + `","prompt":"thunder","prompt_influence":0.5}`,
	} {
		w := postMireloSFX(t, server.URL, body)
		require.Equal(t, http.StatusBadRequest, w.Code, field)
		assert.Contains(t, w.Body.String(), field)
	}
}
