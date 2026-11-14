package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Fixtures for the Mirelo dubbing tests on the Videos API.
const (
	mireloDubPath      = "/signed/dubbed.mp4"
	mireloDubbedBytes  = "FAKE-DUBBED-MP4-BYTES"
	mireloDubType      = "video/mp4"
	mireloGatewayVidID = string(constants.MireloID) + ":" + mireloJobID
)

func mireloDubbedJob(base string) string {
	return `{"id":"` + mireloJobID + `","model":"` + mireloSFXModel + `","status":"succeeded","errors":[],"result":{"output":{"type":"mix","variants":[` +
		`{"index":0,"status":"succeeded","files":{"video":{"url":"` + base + mireloDubPath + `"}}}]}}}`
}

// mireloDubUpstream stands in for Mirelo's asset and video-to-sfx API on the
// Videos path and records the create body the gateway sent.
type mireloDubUpstream struct {
	server       *httptest.Server
	createBody   map[string]any
	job          string
	refuseUpload bool
}

func newMireloDubUpstream(t *testing.T) *mireloDubUpstream {
	t.Helper()
	u := &mireloDubUpstream{}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSONVal)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == constants.MireloAssetsEndpoint:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(mireloTicket(u.server.URL + mireloUploadPath)))
		case r.Method == http.MethodPost && r.URL.Path == mireloUploadPath:
			if u.refuseUpload {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == constants.MireloVideosEndpoint:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&u.createBody))
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(mireloRunningJob))
		case r.Method == http.MethodGet && r.URL.Path == mireloVideoJobPath:
			if u.job == "" {
				u.job = mireloDubbedJob(u.server.URL)
			}
			_, _ = w.Write([]byte(u.job))
		case r.Method == http.MethodGet && r.URL.Path == mireloDubPath:
			w.Header().Set("Content-Type", mireloDubType)
			_, _ = w.Write([]byte(mireloDubbedBytes))
		default:
			t.Errorf("unexpected upstream call %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(u.server.Close)
	return u
}

func postMireloDub(t *testing.T, upstreamURL string, fields []imagesMultipartField) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := buildImagesMultipart(t, fields)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", videosPath, body)
	req.Header.Set("Content-Type", contentType)
	newVideosTestRouter(t, upstreamURL, enableVideos).ServeHTTP(w, req)
	return w
}

// TestVideosHandler_MireloDubsAVideo proves the uploaded clip is scored as a
// video-out job: the asset is uploaded, the create asks for the dubbed video,
// and the job comes back with a routable mirelo: id.
func TestVideosHandler_MireloDubsAVideo(t *testing.T) {
	u := newMireloDubUpstream(t)

	w := postMireloDub(t, u.server.URL, []imagesMultipartField{
		{name: "model", value: "mirelo/" + mireloSFXModel},
		{name: "video", filename: mireloVideoName, value: mireloVideoBytes},
		{name: "prompt", value: "soft keyboard taps"},
		{name: "seconds", value: "4"},
		{name: "preserve_speech", value: "true"},
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var job types.VideoJob
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
	assert.Equal(t, mireloGatewayVidID, job.ID, "the id has to route the follow-up calls")
	assert.Equal(t, types.VideoJobStatusInProgress, job.Status)
	assert.Equal(t, mireloSFXModel, job.Model)

	assert.Equal(t, mireloSFXModel, u.createBody["model"], "the provider prefix must be stripped")
	assert.Equal(t, map[string]any{"video": map[string]any{"type": "asset", "id": mireloAssetID}, "prompt": "soft keyboard taps"}, u.createBody["input"])
	assert.Equal(t, map[string]any{"media": "video"}, u.createBody["output"], "the clip comes back as video, not audio")
	assert.Equal(t, map[string]any{"preserve_speech": true}, u.createBody["controls"], "only the mix is laid onto the clip")
	assert.Equal(t, float64(4000), u.createBody["duration_ms"])
}

// TestVideosHandler_MireloPollAndDownload proves a client polls the dubbing job
// and downloads the re-rendered clip without the API key reaching the CDN.
func TestVideosHandler_MireloPollAndDownload(t *testing.T) {
	u := newMireloDubUpstream(t)
	router := newVideosTestRouter(t, u.server.URL, enableVideos)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", videosPath+"/"+mireloGatewayVidID, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var job types.VideoJob
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
	assert.Equal(t, mireloGatewayVidID, job.ID)
	assert.Equal(t, types.VideoJobStatusCompleted, job.Status)
	assert.Nil(t, job.Error)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", videosPath+"/"+mireloGatewayVidID+"/content", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, mireloDubbedBytes, w.Body.String())
	assert.Equal(t, mireloDubType, w.Header().Get("Content-Type"))
}

func TestVideosHandler_MireloFailedJob(t *testing.T) {
	u := newMireloDubUpstream(t)
	u.job = mireloFailedJob
	router := newVideosTestRouter(t, u.server.URL, enableVideos)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", videosPath+"/"+mireloGatewayVidID, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var job types.VideoJob
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
	assert.Equal(t, types.VideoJobStatusFailed, job.Status)
	require.NotNil(t, job.Error)
	require.NotNil(t, job.Error.Message)
	assert.Equal(t, "the prompt was blocked", *job.Error.Message)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", videosPath+"/"+mireloGatewayVidID+"/content", nil))
	assert.Equal(t, http.StatusNotFound, w.Code, "a failed job has no bytes to serve")
}

func TestVideosHandler_MireloDubRejections(t *testing.T) {
	tests := []struct {
		name       string
		fields     []imagesMultipartField
		refuse     bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "a refused upload",
			fields:     []imagesMultipartField{{name: "model", value: "mirelo/" + mireloSFXModel}, {name: "video", filename: mireloVideoName, value: mireloVideoBytes}},
			refuse:     true,
			wantStatus: http.StatusBadGateway,
			wantBody:   "upload the video",
		},
		{
			name:       "a missing video",
			fields:     []imagesMultipartField{{name: "model", value: "mirelo/" + mireloSFXModel}, {name: "prompt", value: "soft keyboard taps"}},
			wantStatus: http.StatusBadRequest,
			wantBody:   "'video' file is required",
		},
		{
			name:       "a window shorter than a second",
			fields:     []imagesMultipartField{{name: "model", value: "mirelo/" + mireloSFXModel}, {name: "video", filename: mireloVideoName, value: mireloVideoBytes}, {name: "seconds", value: "0"}},
			wantStatus: http.StatusBadRequest,
			wantBody:   "'seconds' field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newMireloDubUpstream(t)
			u.refuseUpload = tt.refuse

			w := postMireloDub(t, u.server.URL, tt.fields)

			require.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}
