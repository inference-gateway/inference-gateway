package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	config "github.com/inference-gateway/inference-gateway/config"
	lipsync "github.com/inference-gateway/inference-gateway/internal/lipsync"
)

const (
	localVideoModel    = lipsync.ReservedModelID
	localVideoIDPrefix = "local:"
	localPortrait      = "PNG-PORTRAIT"
	localClip          = "RIFF-CLIP"
	localRendered      = "MP4 " + localClip
	localRenderWait    = 10 * time.Second
	localRenderTick    = 20 * time.Millisecond

	// A stand-in engine that writes the clip it was handed into the video.
	localEngine = `#!/bin/sh
while [ $# -gt 0 ]; do
  case "$1" in
    --audio) audio="$2"; shift ;;
    --out) out="$2"; shift ;;
  esac
  shift
done
printf 'MP4 %s' "$(cat "$audio")" > "$out"
`
)

// installLocalEngine points the gateway at a fresh home folder holding a
// stand-in lip-sync engine and its weights.
func installLocalEngine(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	tools := filepath.Join(home, lipsync.CacheToolsDir)
	require.NoError(t, os.MkdirAll(tools, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tools, lipsync.BinaryName), []byte(localEngine), 0o755))
	model := filepath.Join(home, filepath.FromSlash(lipsync.CacheModelDir))
	require.NoError(t, os.MkdirAll(model, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(model, lipsync.WeightsFile), []byte("weights"), 0o644))
}

func lipSyncLocally(t *testing.T, router http.Handler, fields ...imagesMultipartField) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := buildImagesMultipart(t, fields)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", videosPath, body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)
	return w
}

func localRender() []imagesMultipartField {
	return []imagesMultipartField{
		{name: "model", value: localVideoModel},
		{name: "input_reference", filename: "front.png", value: localPortrait},
		{name: "audio", filename: "line.wav", value: localClip},
	}
}

func getVideo(router http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func TestVideosHandler_LocalRendersAndServesTheClip(t *testing.T) {
	installLocalEngine(t)
	router := newVideosTestRouter(t, "http://unused", enableVideos)

	w := lipSyncLocally(t, router, localRender()...)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var queued map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &queued))
	id, _ := queued["id"].(string)
	require.True(t, strings.HasPrefix(id, localVideoIDPrefix), "the video id %q must say the render is local", id)
	assert.Equal(t, localVideoModel, queued["model"])
	assert.Equal(t, "video", queued["object"])

	jobPath := strings.Replace(videoByIDPath, "{video_id}", id, 1)
	var job map[string]any
	require.Eventually(t, func() bool {
		polled := getVideo(router, jobPath)
		require.Equal(t, http.StatusOK, polled.Code, polled.Body.String())
		require.NoError(t, json.Unmarshal(polled.Body.Bytes(), &job))
		return job["status"] == "completed"
	}, localRenderWait, localRenderTick)
	assert.Equal(t, id, job["id"])

	content := getVideo(router, strings.Replace(videoContentPath, "{video_id}", id, 1))
	require.Equal(t, http.StatusOK, content.Code, content.Body.String())
	assert.Equal(t, localRendered, content.Body.String())
	assert.Equal(t, "video/mp4", content.Header().Get("Content-Type"))
}

func TestVideosHandler_LocalSaysWhenTheEngineIsNotInstalled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	router := newVideosTestRouter(t, "http://unused", enableVideos)

	w := lipSyncLocally(t, router, localRender()...)

	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), "not installed")
}

func TestVideosHandler_LocalNeedsAPortraitAndAClip(t *testing.T) {
	installLocalEngine(t)
	router := newVideosTestRouter(t, "http://unused", enableVideos)

	w := lipSyncLocally(t, router,
		imagesMultipartField{name: "model", value: localVideoModel},
		imagesMultipartField{name: "input_reference", filename: "front.png", value: localPortrait},
	)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "both required")
}

func TestVideosHandler_LocalKnowsOneModel(t *testing.T) {
	installLocalEngine(t)
	router := newVideosTestRouter(t, "http://unused", enableVideos)

	w := lipSyncLocally(t, router, imagesMultipartField{name: "model", value: "local/wav2lip"})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "only local video model")
}

func TestVideosHandler_LocalHonoursTheModelDenyList(t *testing.T) {
	installLocalEngine(t)
	router := newVideosTestRouter(t, "http://unused", enableVideos, func(c *config.Config) { c.DisallowedModels = lipsync.ReservedModelID })

	w := lipSyncLocally(t, router, localRender()...)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestRetrieveVideoHandler_LocalDoesNotKnowAForgottenRender(t *testing.T) {
	installLocalEngine(t)
	router := newVideosTestRouter(t, "http://unused", enableVideos)

	job := getVideo(router, strings.Replace(videoByIDPath, "{video_id}", localVideoIDPrefix+"gone", 1))
	content := getVideo(router, strings.Replace(videoContentPath, "{video_id}", localVideoIDPrefix+"gone", 1))

	assert.Equal(t, http.StatusNotFound, job.Code)
	assert.Contains(t, job.Body.String(), "restart forgets them")
	assert.Equal(t, http.StatusNotFound, content.Code)
}

func TestVideosHandler_LocalIsOffWithTheVideosAPI(t *testing.T) {
	installLocalEngine(t)
	router := newVideosTestRouter(t, "http://unused")

	w := lipSyncLocally(t, router, localRender()...)
	job := getVideo(router, strings.Replace(videoByIDPath, "{video_id}", localVideoIDPrefix+"any", 1))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "VIDEOS_ENABLED")
	assert.Equal(t, http.StatusNotFound, job.Code)
}
