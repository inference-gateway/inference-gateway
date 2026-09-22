package tests

import (
	"encoding/base64"
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

// Fixtures shared by the ElevenLabs audio and video handler tests.
const (
	elevenlabsTestKey    = "test-elevenlabs-key"
	elevenlabsAPIKey     = "xi-api-key"
	elevenlabsVoice      = "21m00Tcm4TlvDq8ikWAM"
	elevenlabsTTSModel   = "eleven_multilingual_v2"
	elevenlabsSFXModel   = "eleven_text_to_sound_v2"
	elevenlabsMusicModel = "music_v2"
	elevenlabsVidModel   = "creatify-aurora"
	elevenlabsJobID      = "gen-abc123"

	sfxPath            = "/v1/audio/sfx"
	musicPath          = "/v1/audio/music"
	videosPath         = "/v1/videos"
	videoByIDPath      = "/v1/videos/:video_id"
	videoContentPath   = "/v1/videos/:video_id/content"
	speechPath         = "/v1/audio/speech"
	contentTypeJSONVal = "application/json"
)

func enableVideos(c *config.Config) { c.VideosEnabled = true }

// newVideosTestRouter registers all four Videos routes against upstream.
func newVideosTestRouter(t *testing.T, upstreamURL string, opts ...func(*config.Config)) *gin.Engine {
	t.Helper()
	router := newImagesTestRouter(t, upstreamURL, false, opts...)
	r := gin.New()
	r.POST(videosPath, router.VideosHandler)
	r.GET(videoByIDPath, router.RetrieveVideoHandler)
	r.GET(videoContentPath, router.DownloadVideoContentHandler)
	return r
}

// TestSpeechHandler_Elevenlabs proves the one provider whose Audio API is not
// OpenAI-compatible gets its request rewritten: voice into the path, container
// into the output_format query, and the API key into ElevenLabs' own header.
func TestSpeechHandler_Elevenlabs(t *testing.T) {
	var gotPath, gotQuery, gotKey, gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotKey = r.Header.Get(elevenlabsAPIKey)
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("FAKE-AUDIO-BYTES"))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST(speechPath, router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", speechPath, strings.NewReader(
		`{"model":"elevenlabs/`+elevenlabsTTSModel+`","input":"Hello world","voice":"`+elevenlabsVoice+`","speed":1.1}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/text-to-speech/"+elevenlabsVoice, gotPath, "the voice belongs in the URL path")
	assert.Equal(t, "output_format=mp3_44100_128", gotQuery)
	assert.Equal(t, "Hello world", gotBody["text"], "OpenAI input maps onto ElevenLabs text")
	assert.Equal(t, elevenlabsTTSModel, gotBody["model_id"], "the provider prefix must be stripped")
	assert.Equal(t, elevenlabsTestKey, gotKey, "the key goes in the provider's own auth header")
	assert.Empty(t, gotAuth, "the caller's Authorization header must never reach the provider")
	assert.Equal(t, "FAKE-AUDIO-BYTES", w.Body.String())
}

func TestSpeechHandler_ElevenlabsRejectsUnsupportedFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the request must be rejected before it reaches the provider")
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST(speechPath, router.SpeechHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", speechPath, strings.NewReader(
		`{"model":"elevenlabs/`+elevenlabsTTSModel+`","input":"Hello","voice":"`+elevenlabsVoice+`","response_format":"wav"}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "response_format", "the caller must learn which formats are servable")
}

func TestSFXHandler_HappyPath(t *testing.T) {
	var gotPath, gotKey string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get(elevenlabsAPIKey)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("FAKE-SFX-BYTES"))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST(sfxPath, router.SFXHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", sfxPath, strings.NewReader(
		`{"model":"elevenlabs/`+elevenlabsSFXModel+`","prompt":"distant thunder","duration_seconds":6,"loop":true}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/sound-generation", gotPath)
	assert.Equal(t, "distant thunder", gotBody["text"])
	assert.Equal(t, elevenlabsSFXModel, gotBody["model_id"])
	assert.Equal(t, "mp3_44100_128", gotBody["output_format"])
	assert.Equal(t, float64(6), gotBody["duration_seconds"])
	assert.Equal(t, true, gotBody["loop"])
	assert.Equal(t, elevenlabsTestKey, gotKey)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "FAKE-SFX-BYTES", w.Body.String())
}

func TestSFXHandler_DisabledReturns404(t *testing.T) {
	router := newImagesTestRouter(t, "http://127.0.0.1:0", false)
	r := gin.New()
	r.POST(sfxPath, router.SFXHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", sfxPath, strings.NewReader(`{"model":"elevenlabs/x","prompt":"thunder"}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "AUDIO_ENABLED")
}

// TestSFXHandler_ProviderWithoutSupport proves a provider that has no
// sound-effect endpoint is rejected rather than sent a request it cannot serve.
func TestSFXHandler_ProviderWithoutSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a provider without sfx support must never be called")
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST(sfxPath, router.SFXHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", sfxPath, strings.NewReader(`{"model":"openai/tts-1","prompt":"thunder"}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Sound effect generation is not supported by this provider yet.")
}

func TestMusicHandler_HappyPath(t *testing.T) {
	var gotPath, gotQuery, gotKey string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotKey = r.Header.Get(elevenlabsAPIKey)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("FAKE-MUSIC-BYTES"))
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST(musicPath, router.MusicHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", musicPath, strings.NewReader(
		`{"model":"elevenlabs/`+elevenlabsMusicModel+`","prompt":"chill lo-fi hip hop","duration_seconds":15,"instrumental":true}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/music", gotPath)
	assert.Equal(t, "output_format=mp3_44100_128", gotQuery)
	assert.Equal(t, "chill lo-fi hip hop", gotBody["prompt"])
	assert.Equal(t, elevenlabsMusicModel, gotBody["model_id"], "the provider prefix must be stripped")
	assert.Equal(t, float64(15000), gotBody["music_length_ms"])
	assert.Equal(t, true, gotBody["force_instrumental"])
	assert.Equal(t, elevenlabsTestKey, gotKey)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "FAKE-MUSIC-BYTES", w.Body.String())
}

// TestMusicHandler_ProviderWithoutSupport mirrors the SFX case: providers
// without a music endpoint are rejected before anything is sent upstream.
func TestMusicHandler_ProviderWithoutSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a provider without music support must never be called")
	}))
	defer server.Close()

	router := newImagesTestRouter(t, server.URL, false, enableAudio)
	r := gin.New()
	r.POST(musicPath, router.MusicHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", musicPath, strings.NewReader(`{"model":"openai/tts-1","prompt":"lo-fi"}`))
	req.Header.Set("Content-Type", contentTypeJSONVal)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Music generation is not supported by this provider yet.")
}

func TestVideosHandler_HappyPath(t *testing.T) {
	var gotPath, gotKey, gotContentType string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get(elevenlabsAPIKey)
		gotContentType = r.Header.Get("Content-Type")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"id":"` + elevenlabsJobID + `","status":"pending"}`))
	}))
	defer server.Close()

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "model", value: "elevenlabs/" + elevenlabsVidModel},
		{name: "prompt", value: "a slow pan across a misty valley"},
		{name: "input_reference", filename: "portrait.png", value: "PNG-PORTRAIT-BYTES"},
		{name: "audio", filename: "line.wav", value: "RIFF-fake-wav"},
		{name: "size", value: "720x1280"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", videosPath, body)
	req.Header.Set("Content-Type", contentType)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/flows/video", gotPath)
	assert.Equal(t, contentTypeJSONVal, gotContentType, "elevenlabs takes JSON, not the OpenAI multipart form")
	assert.Equal(t, elevenlabsVidModel, gotBody["model_id"], "the provider prefix must be stripped")
	assert.Nil(t, gotBody["prompt"], "avatar requests carry no prompt")
	assert.Equal(t, "720p", gotBody["resolution"])
	assert.Nil(t, gotBody["aspect_ratio"], "avatar models reject aspect_ratio")
	image := gotBody["image"].(map[string]any)
	assert.Equal(t, "inline_base64", image["type"])
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("PNG-PORTRAIT-BYTES")), image["content_base64"], "the reference image must reach the provider inline")
	audio := gotBody["audio"].(map[string]any)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("RIFF-fake-wav")), audio["content_base64"], "the driving audio must reach the provider inline")
	assert.Equal(t, elevenlabsTestKey, gotKey)

	var job map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
	assert.Equal(t, "elevenlabs:"+elevenlabsJobID, job["id"],
		"the job id must carry the provider so the follow-up calls can be routed")
	assert.Equal(t, "video", job["object"])
	assert.Equal(t, "queued", job["status"])
	assert.Equal(t, elevenlabsVidModel, job["model"], "an unechoed model falls back to the requested one")
	assert.NotZero(t, job["created_at"])
}

func TestVideosHandler_DisabledReturns404(t *testing.T) {
	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "model", value: "elevenlabs/" + elevenlabsVidModel},
		{name: "prompt", value: "a valley"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", videosPath, body)
	req.Header.Set("Content-Type", contentType)
	newVideosTestRouter(t, "http://127.0.0.1:0").ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "VIDEOS_ENABLED")
}

// TestVideosHandler_ProviderWithoutSupport proves the Videos API is refused for
// providers that do not implement it, mirroring the schema's VideosNotSupported.
func TestVideosHandler_ProviderWithoutSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a provider without video support must never be called")
	}))
	defer server.Close()

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "model", value: "openai/gpt-image-2"},
		{name: "prompt", value: "a valley"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", videosPath, body)
	req.Header.Set("Content-Type", contentType)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "The Videos API is not supported by this provider yet.")
}

// TestVideosHandler_UpstreamErrorRelayedVerbatim proves the provider's own
// explanation reaches the caller instead of being flattened into a generic error.
func TestVideosHandler_UpstreamErrorRelayedVerbatim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"resolution 1080p is not supported by this model"}`))
	}))
	defer server.Close()

	body, contentType := buildImagesMultipart(t, []imagesMultipartField{
		{name: "model", value: "elevenlabs/" + elevenlabsVidModel},
		{name: "prompt", value: "a valley"},
		{name: "size", value: "1920x1080"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", videosPath, body)
	req.Header.Set("Content-Type", contentType)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "resolution 1080p is not supported by this model")
}

func TestRetrieveVideoHandler_StripsProviderPrefix(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"generation_id":"` + elevenlabsJobID +
			`","status":"processing","progress":42,"model_id":"` + elevenlabsVidModel + `","created_at_unix":1699999999}`))
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/elevenlabs:"+elevenlabsJobID, nil)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/flows/video/"+elevenlabsJobID, gotPath, "the upstream id must be sent back without the prefix")

	var job map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
	assert.Equal(t, "elevenlabs:"+elevenlabsJobID, job["id"], "the prefix must be restored on the way out")
	assert.Equal(t, "in_progress", job["status"], "an in-flight upstream state maps onto the schema enum")
	assert.Equal(t, float64(42), job["progress"])
	assert.Equal(t, float64(1699999999), job["created_at"])
}

func TestRetrieveVideoHandler_UnroutableIDIsRejected(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/"+elevenlabsJobID, nil)
	newVideosTestRouter(t, "http://127.0.0.1:0", enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Unable to determine provider")
}

// TestRetrieveVideoHandler_ProviderQueryWins proves ?provider= overrides the
// prefix, mirroring model routing, and that the prefix is still stripped.
func TestRetrieveVideoHandler_ProviderQueryWins(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"generation_id":"` + elevenlabsJobID + `","status":"queued"}`))
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/elevenlabs:"+elevenlabsJobID+"?provider=elevenlabs", nil)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/flows/video/"+elevenlabsJobID, gotPath)
}

func TestRetrieveVideoHandler_FailedJobCarriesTheError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"generation_id":"` + elevenlabsJobID + `","status":"failed","error_message":"the portrait has no face"}`))
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/elevenlabs:"+elevenlabsJobID, nil)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var job struct {
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
	assert.Equal(t, "failed", job.Status)
	require.NotNil(t, job.Error)
	assert.Equal(t, "the portrait has no face", job.Error.Message)
}

func TestDownloadVideoContentHandler_StreamsCompletedRender(t *testing.T) {
	// The render URL is absolute and provider-supplied; renderURL is filled in
	// once the test server has an address so it can point back at itself and the
	// stream can be asserted end to end.
	var gotJobPath, gotRenderPath, renderURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".mp4") {
			gotRenderPath = r.URL.Path
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("FAKE-MP4-BYTES"))
			return
		}
		gotJobPath = r.URL.Path
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"generation_id":"` + elevenlabsJobID + `","status":"completed","content_url":"` + renderURL + `"}`))
	}))
	defer server.Close()
	renderURL = server.URL + "/renders/" + elevenlabsJobID + ".mp4"

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/elevenlabs:"+elevenlabsJobID+"/content", nil)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/flows/video/"+elevenlabsJobID, gotJobPath, "the job is polled before the bytes are fetched")
	assert.Equal(t, "/renders/"+elevenlabsJobID+".mp4", gotRenderPath)
	assert.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
	assert.Equal(t, "FAKE-MP4-BYTES", w.Body.String())
}

func TestDownloadVideoContentHandler_UnfinishedRenderReturns404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"generation_id":"` + elevenlabsJobID + `","status":"processing","progress":10}`))
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/elevenlabs:"+elevenlabsJobID+"/content", nil)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not available yet")
}

// TestDownloadVideoContentHandler_RejectsNonHTTPDownloadURL proves a malformed
// provider payload cannot make the gateway fetch a non-HTTP address.
func TestDownloadVideoContentHandler_RejectsNonHTTPDownloadURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		_, _ = w.Write([]byte(`{"generation_id":"` + elevenlabsJobID + `","status":"completed","media_url":"file:///etc/passwd"}`))
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", videosPath+"/elevenlabs:"+elevenlabsJobID+"/content", nil)
	newVideosTestRouter(t, server.URL, enableVideos).ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "unusable video download URL")
}
