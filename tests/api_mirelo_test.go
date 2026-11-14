package tests

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	config "github.com/inference-gateway/inference-gateway/config"
	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
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
	// mireloTestReadTimeout mirrors the ReadTimeout newImagesTestRouter sets.
	mireloTestReadTimeout = 5 * time.Second
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
	assert.Equal(t, sfx.Plan(mireloTestReadTimeout, 0).Requests, requests, "polling must be bounded")
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

// Fixtures for the video-to-sfx handler tests.
const (
	mireloAssetID       = "ast_123"
	mireloUploadPath    = "/upload"
	mireloVideoJobPath  = constants.MireloSFXVideoEndpoint + "/" + mireloJobID
	mireloMixPath       = "/signed/mix.mp3"
	mireloSpeechPath    = "/signed/speech.wav"
	mireloMixBytes      = "FAKE-MIX-BYTES"
	mireloSpeechBytes   = "FAKE-SPEECH-BYTES"
	mireloVideoBytes    = "FAKE-VIDEO-BYTES"
	mireloVideoName     = "clip.mov"
	mireloVideoType     = "video/quicktime"
	mireloAssetNotReady = `{"error":{"code":"asset_not_ready","message":"the upload is still settling"}}`
)

func mireloTicket(uploadURL string) string {
	return `{"id":"` + mireloAssetID + `","upload_url":"` + uploadURL + `","upload_expires_at":"2026-11-15T00:00:00Z","max_bytes":1073741824,"fields":{"key":"k1","Policy":"p1"}}`
}

func mireloScoredJob(base string) string {
	return `{"id":"` + mireloJobID + `","status":"succeeded","errors":[],"result":{"outputs":[` +
		`{"index":0,"type":"mix","variants":[{"index":0,"status":"succeeded","files":{"audio":{"url":"` + base + mireloMixPath + `","format":"mp3_128"}}}]},` +
		`{"index":1,"type":"speech","variants":[{"index":0,"status":"succeeded","files":{"audio":{"url":"` + base + mireloSpeechPath + `","format":"wav"}}}]}]}}`
}

// mireloVideoUpstream is a stand-in for Mirelo's asset and video-to-sfx API
// that records what the gateway sent it.
type mireloVideoUpstream struct {
	server         *httptest.Server
	ticketType     string
	uploadAuth     string
	uploadLength   int64
	uploadOrder    []string
	uploadFileType string
	createBody     map[string]any
	creates        int
	refuseCreates  int
	refuseUpload   bool
}

func newMireloVideoUpstream(t *testing.T) *mireloVideoUpstream {
	t.Helper()
	u := &mireloVideoUpstream{}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == constants.MireloAssetsEndpoint:
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			u.ticketType = body["content_type"]
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(mireloTicket(u.server.URL + mireloUploadPath)))
		case r.Method == http.MethodPost && r.URL.Path == mireloUploadPath:
			u.uploadAuth = r.Header.Get("Authorization")
			u.uploadLength = r.ContentLength
			u.recordUpload(t, r)
			if u.refuseUpload {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == constants.MireloSFXVideoEndpoint:
			u.creates++
			require.NoError(t, json.NewDecoder(r.Body).Decode(&u.createBody))
			if u.creates <= u.refuseCreates {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(mireloAssetNotReady))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(mireloRunningJob))
		case r.Method == http.MethodGet && r.URL.Path == mireloVideoJobPath:
			_, _ = w.Write([]byte(mireloScoredJob(u.server.URL)))
		case r.Method == http.MethodGet && r.URL.Path == mireloMixPath:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(mireloMixBytes))
		case r.Method == http.MethodGet && r.URL.Path == mireloSpeechPath:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(mireloSpeechBytes))
		default:
			t.Errorf("unexpected upstream call %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (u *mireloVideoUpstream) recordUpload(t *testing.T, r *http.Request) {
	t.Helper()
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	require.NoError(t, err)
	reader := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return
		}
		require.NoError(t, err)
		u.uploadOrder = append(u.uploadOrder, part.FormName())
		if part.FormName() == sfx.UploadFilePart {
			u.uploadFileType = part.Header.Get("Content-Type")
		}
	}
}

func postMireloVideo(t *testing.T, upstreamURL string, fields []imagesMultipartField, opts ...func(*config.Config)) *httptest.ResponseRecorder {
	t.Helper()
	router := newImagesTestRouter(t, upstreamURL, false, append([]func(*config.Config){enableAudio}, opts...)...)
	r := http.NewServeMux()
	r.HandleFunc("POST "+sfxPath, router.SFXHandler)

	body, contentType := buildImagesMultipart(t, fields)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", sfxPath, body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)
	return w
}

func mireloVideoFields(extra ...imagesMultipartField) []imagesMultipartField {
	return append([]imagesMultipartField{
		{name: "model", value: "mirelo/" + mireloSFXModel},
		{name: "video", filename: mireloVideoName, value: mireloVideoBytes},
	}, extra...)
}

// TestSFXHandler_MireloScoresAVideo proves the upload goes through Mirelo's
// ticket without the API key, the job scores the asset for the rest of the
// clip, and the wanted track comes back as raw audio.
func TestSFXHandler_MireloScoresAVideo(t *testing.T) {
	u := newMireloVideoUpstream(t)

	w := postMireloVideo(t, u.server.URL, mireloVideoFields(
		imagesMultipartField{name: "prompt", value: "soft keyboard taps"},
		imagesMultipartField{name: "preserve_speech", value: "true"},
		imagesMultipartField{name: "output", value: sfx.OutputSpeech},
	))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, mireloSpeechBytes, w.Body.String(), "the speech track was asked for")
	assert.Equal(t, "audio/wav", w.Header().Get("Content-Type"))

	assert.Equal(t, mireloVideoType, u.ticketType, "a .mov is announced as QuickTime even when sniffing cannot tell")
	assert.Empty(t, u.uploadAuth, "the signed upload must not receive the provider key")
	assert.Equal(t, []string{"Policy", "key", sfx.UploadFilePart}, u.uploadOrder, "ticket fields go before the file")
	assert.Positive(t, u.uploadLength, "the upload must announce its length, S3 refuses a chunked post")
	assert.Equal(t, mireloVideoType, u.uploadFileType)

	assert.Equal(t, 1, u.creates)
	assert.Equal(t, mireloSFXModel, u.createBody["model"])
	assert.Equal(t, map[string]any{"video": map[string]any{"type": "asset", "id": mireloAssetID}, "prompt": "soft keyboard taps"}, u.createBody["input"])
	assert.Equal(t, map[string]any{"preserve_speech": true, "separate_speech": true}, u.createBody["controls"])
	assert.NotContains(t, u.createBody, "duration_ms", "no duration scores the rest of the clip")
}

func TestSFXHandler_MireloRetriesWhileTheAssetSettles(t *testing.T) {
	u := newMireloVideoUpstream(t)
	u.refuseCreates = 1

	w := postMireloVideo(t, u.server.URL, mireloVideoFields())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, mireloMixBytes, w.Body.String(), "the mix is the default track")
	assert.Equal(t, 2, u.creates, "the create is retried once the asset is ready")
}

func TestSFXHandler_MireloVideoRejections(t *testing.T) {
	tests := []struct {
		name       string
		fields     []imagesMultipartField
		opts       []func(*config.Config)
		refuse     bool
		wantStatus int
		wantBody   string
	}{
		{"a refused upload", mireloVideoFields(), nil, true, http.StatusBadGateway, "upload the video"},
		{"a missing video", []imagesMultipartField{{name: "model", value: "mirelo/" + mireloSFXModel}}, nil, false, http.StatusBadRequest, "'video' file is required"},
		{"a provider without video scoring", []imagesMultipartField{{name: "model", value: "elevenlabs/eleven_text_to_sound_v2"}, {name: "video", filename: mireloVideoName, value: mireloVideoBytes}}, nil, false, http.StatusBadRequest, "Video scoring is not supported"},
		{"a track that is not there", mireloVideoFields(imagesMultipartField{name: "output", value: sfx.OutputSFX}), nil, false, http.StatusBadGateway, "without a sfx track"},
		{"a video over the body limit", mireloVideoFields(), []func(*config.Config){func(c *config.Config) { c.Server.MaxRequestBodySize = 16 }}, false, http.StatusRequestEntityTooLarge, "too large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newMireloVideoUpstream(t)
			u.refuseUpload = tt.refuse

			w := postMireloVideo(t, u.server.URL, tt.fields, tt.opts...)

			require.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}
