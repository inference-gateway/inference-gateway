package api

import (
	"io"
	"net/http"
	"strconv"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

// writeData writes body as the response with the given status. Content-Type
// is set only when the response does not already carry one.
func writeData(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType != "" && len(w.Header()["Content-Type"]) == 0 {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeDataFromReader relays reader as the response body with the given status,
// length and content type.
func writeDataFromReader(w http.ResponseWriter, status int, contentLength int64, contentType string, reader io.Reader) {
	if contentType != "" && len(w.Header()["Content-Type"]) == 0 {
		w.Header().Set("Content-Type", contentType)
	}
	if contentLength >= 0 && w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, reader)
}

// audioEnabled reports whether the Audio API is switched on, writing the 404
// AUDIO_ENABLED response when it is not.
func (router *RouterImpl) audioEnabled(w http.ResponseWriter) bool {
	if router.cfg.AudioEnabled {
		return true
	}
	router.logger.Error("audio api not enabled", nil)
	middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Audio API is not enabled. Set AUDIO_ENABLED=true to enable it."})
	return false
}

// videosEnabled reports whether the Videos API is switched on, writing the 404
// VIDEOS_ENABLED response when it is not.
func (router *RouterImpl) videosEnabled(w http.ResponseWriter) bool {
	if router.cfg.VideosEnabled {
		return true
	}
	router.logger.Error("videos api not enabled", nil)
	middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "The Videos API is not enabled. Set VIDEOS_ENABLED=true to enable it."})
	return false
}
