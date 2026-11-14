package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
)

// parseBoundedMultipart parses the request's multipart/form-data body within the
// configured max request body size and returns the form. It writes the client
// response and reports false when the body is too large or malformed; the caller
// still owns removing the form's temp files.
func (router *RouterImpl) parseBoundedMultipart(w http.ResponseWriter, r *http.Request) (*multipart.Form, bool) {
	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBodySize))
	if err := r.ParseMultipartForm(multipartMaxMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
			return nil, false
		}
		router.logger.Error("failed to parse multipart form", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to parse multipart/form-data request"})
		return nil, false
	}
	return r.MultipartForm, true
}

// formValue returns the first value for key in a parsed multipart form,
// or "" when absent.
func formValue(form *multipart.Form, key string) string {
	if vs := form.Value[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// writeMultipartForm re-encodes a parsed multipart form onto mw, copying
// each uploaded file straight through (preserving an explicit Content-Type,
// sniffing a missing or octet-stream one) so the
// payload streams to the upstream without a second full in-memory copy. It
// runs in its own goroutine writing to the io.Pipe. Shared by the Images and
// Videos multipart endpoints.
func writeMultipartForm(mw *multipart.Writer, form *multipart.Form) error {
	for field, values := range form.Value {
		for _, v := range values {
			if err := mw.WriteField(field, v); err != nil {
				return err
			}
		}
	}
	for field, headers := range form.File {
		for _, fh := range headers {
			if err := copyFormFile(mw, field, fh); err != nil {
				return err
			}
		}
	}
	return mw.Close()
}

// Multipart file part content types.
const (
	contentTypeOctetStream = "application/octet-stream"
	// contentSniffLen is how many leading bytes http.DetectContentType reads.
	contentSniffLen = 512
)

// copyFormFile copies one uploaded file into mw. A part without a type or
// typed application/octet-stream (what Go's multipart.CreateFormFile and many
// clients send) is labelled from its leading bytes, because providers such as
// OpenAI's /images/edits reject octet-stream uploads; an explicit type is kept.
func copyFormFile(mw *multipart.Writer, field string, fh *multipart.FileHeader) error {
	body, contentType, file, err := openFormFile(fh)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeFilePart(mw, field, fh.Filename, contentType, body)
}

// openFormFile opens an uploaded part and settles its media type, sniffing
// the leading bytes when the client sent none or octet-stream. The caller
// closes the returned file once the reader is drained.
func openFormFile(fh *multipart.FileHeader) (io.Reader, string, io.Closer, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, "", nil, err
	}

	var body io.Reader = src
	ct := fh.Header.Get("Content-Type")
	if ct == "" || ct == contentTypeOctetStream {
		head := make([]byte, contentSniffLen)
		n, err := io.ReadFull(src, head)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			_ = src.Close()
			return nil, "", nil, err
		}
		if n > 0 {
			ct = http.DetectContentType(head[:n])
		}
		body = io.MultiReader(bytes.NewReader(head[:n]), src)
	}
	return body, ct, src, nil
}

// writeFilePart adds body to mw as the file part named field.
func writeFilePart(mw *multipart.Writer, field, filename, contentType string, body io.Reader) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	dst, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, body)
	return err
}

// Multipart form field names shared by the multipart endpoints.
const (
	// formFieldModel is the model field of a multipart upload.
	formFieldModel = "model"

	// formFieldVideo is the uploaded video part served to /audio/sfx and
	// /videos.
	formFieldVideo = "video"

	// multipartMaxMemory caps how much of a multipart upload is kept in
	// memory; parts above it spill to temp files instead.
	multipartMaxMemory = 1 << 20
)
