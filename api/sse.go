package api

import (
	"bufio"
	"bytes"
	"io"
	"net/http"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

const (
	// sseReaderBufferSize is the bufio buffer used when relaying upstream SSE lines.
	sseReaderBufferSize = 4096
	// sseChunkLogMinBytes is the line size above which development mode logs a chunk preview.
	sseChunkLogMinBytes = 512
	// sseChunkPreviewBytes caps the logged preview length.
	sseChunkPreviewBytes = 200
	// sseChunkSampleEvery samples smaller /proxy chunks for the development preview.
	sseChunkSampleEvery = 10
)

// relaySSE streams upstream SSE lines to the client, resetting the write
// deadline per line. A partial trailing line is written before the read error
// is inspected so nothing buffered is dropped. The upstream request carries
// the client's context, so cancellation surfaces here as a read error - no
// separate ctx.Done() check is needed.
func (router *RouterImpl) relaySSE(w http.ResponseWriter, r *http.Request, upstream io.Reader, logURL string) {
	reader := bufio.NewReaderSize(upstream, sseReaderBufferSize)
	middlewares.StreamResponse(w, func(_ io.Writer) bool {
		middlewares.ResetWriteDeadline(w, router.cfg.Server.WriteTimeout)

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			router.logStreamChunk(r, line)
			if _, werr := w.Write(line); werr != nil {
				router.logger.Error("failed to write chunk", werr, "bytes", len(line))
				return false
			}
		}
		if err != nil {
			if err != io.EOF {
				router.logger.Error("failed to read stream", err, "url", logURL, "method", r.Method)
			}
			return false
		}
		return true
	})
}

// logStreamChunk emits a development-only preview of large (or sampled
// /proxy) SSE lines.
func (router *RouterImpl) logStreamChunk(r *http.Request, line []byte) {
	if router.cfg.Environment != constants.EnvironmentDevelopment {
		return
	}
	providerParam := r.PathValue("provider")
	if len(line) <= sseChunkLogMinBytes && (providerParam == "" || len(line)%sseChunkSampleEvery != 0) {
		return
	}
	preview := string(bytes.TrimSpace(line))
	if len(preview) > sseChunkPreviewBytes {
		preview = preview[:sseChunkPreviewBytes] + "... (truncated)"
	}
	router.logger.Debug("stream chunk", "provider", providerParam, "bytes", len(line), "data_preview", preview)
}
