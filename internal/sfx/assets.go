package sfx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	// UploadFilePart names the multipart part that carries the file, which
	// Mirelo's storage requires to come after every ticket field.
	UploadFilePart = "file"

	assetNotReadyCode = "asset_not_ready"
	retryAfterDefault = time.Second
	retryAfterMax     = 5 * time.Second
	videoTypePrefix   = "video/"
)

var videoTypesByExtension = map[string]string{
	".mov":  "video/quicktime",
	".mp4":  "video/mp4",
	".m4v":  "video/x-m4v",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
}

// AssetTicket is where Mirelo wants an upload to go: a signed URL and the
// form fields that carry its policy, sent before the file.
type AssetTicket struct {
	ID        string            `json:"id"`
	UploadURL string            `json:"upload_url"`
	Fields    map[string]string `json:"fields"`
}

// AssetRequest is the body of the upload-ticket request for a file of the
// given media type.
func AssetRequest(contentType string) []byte {
	body, _ := json.Marshal(map[string]string{"content_type": contentType})
	return body
}

// ParseAssetTicket decodes an upload ticket, rejecting one that names no
// asset or no destination.
func ParseAssetTicket(raw []byte) (AssetTicket, error) {
	var ticket AssetTicket
	if err := json.Unmarshal(raw, &ticket); err != nil {
		return AssetTicket{}, fmt.Errorf("failed to decode mirelo upload ticket: %w", err)
	}
	if ticket.ID == "" || ticket.UploadURL == "" {
		return AssetTicket{}, fmt.Errorf("mirelo upload ticket carries no asset id or upload url")
	}
	return ticket, nil
}

// UploadBody builds the multipart upload for a file of size bytes: the
// ticket's fields, then the file as the last part. Only the file streams, the
// rest is assembled up front so the body's length is known, which the storage
// behind the ticket requires (it refuses a chunked upload with 411).
func UploadBody(ticket AssetTicket, filename, contentType string, file io.Reader, size int64) (body io.Reader, formContentType string, length int64, err error) {
	head := &bytes.Buffer{}
	mw := multipart.NewWriter(head)
	for _, name := range slices.Sorted(maps.Keys(ticket.Fields)) {
		if err := mw.WriteField(name, ticket.Fields[name]); err != nil {
			return nil, "", 0, err
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, UploadFilePart, filename))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	if _, err := mw.CreatePart(header); err != nil {
		return nil, "", 0, err
	}
	trailer := "\r\n--" + mw.Boundary() + "--\r\n"
	length = int64(head.Len()) + size + int64(len(trailer))
	return io.MultiReader(head, file, strings.NewReader(trailer)), mw.FormDataContentType(), length, nil
}

// VideoContentType settles the media type of an uploaded video: the sniffed
// type when it already says video, else the type its file extension implies,
// since Go's sniffer does not know QuickTime.
func VideoContentType(filename, sniffed string) string {
	if strings.HasPrefix(sniffed, videoTypePrefix) {
		return sniffed
	}
	if byExtension, ok := videoTypesByExtension[strings.ToLower(filepath.Ext(filename))]; ok {
		return byExtension
	}
	return sniffed
}

// AssetNotReady reports whether an upstream error says the uploaded asset is
// still settling, which Mirelo documents as retryable.
func AssetNotReady(status int, body []byte) bool {
	if status != http.StatusConflict {
		return false
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	return json.Unmarshal(body, &envelope) == nil && envelope.Error.Code == assetNotReadyCode
}

// RetryAfter turns a Retry-After header into a bounded pause before the next
// attempt.
func RetryAfter(header string) time.Duration {
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds <= 0 {
		return retryAfterDefault
	}
	return min(time.Duration(seconds)*time.Second, retryAfterMax)
}
