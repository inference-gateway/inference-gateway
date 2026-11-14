package sfx

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestParseAssetTicket(t *testing.T) {
	ticket, err := ParseAssetTicket([]byte(`{"id":"ast_1","upload_url":"https://bucket/upload","max_bytes":1073741824,"fields":{"key":"k","Policy":"p"}}`))
	require.NoError(t, err)
	assert.Equal(t, AssetTicket{ID: "ast_1", UploadURL: "https://bucket/upload", Fields: map[string]string{"key": "k", "Policy": "p"}}, ticket)

	_, err = ParseAssetTicket([]byte(`{"id":"ast_1"}`))
	require.Error(t, err)
	_, err = ParseAssetTicket([]byte(`not json`))
	require.Error(t, err)
}

func TestUploadBodyIsSizedAndOrdered(t *testing.T) {
	ticket := AssetTicket{Fields: map[string]string{"key": "k", "Policy": "p", "X-Amz-Algorithm": "a"}}
	video := "FAKE-VIDEO-BYTES"

	body, contentType, length, err := UploadBody(ticket, "clip.mov", "video/quicktime", strings.NewReader(video), int64(len(video)))
	require.NoError(t, err)

	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, int64(len(raw)), length, "the announced length must match the bytes sent")

	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	assert.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
	var order []string
	var fileType, fileBytes string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		order = append(order, part.FormName())
		if part.FormName() == UploadFilePart {
			fileType = part.Header.Get("Content-Type")
			content, err := io.ReadAll(part)
			require.NoError(t, err)
			fileBytes = string(content)
		}
	}
	assert.Equal(t, []string{"Policy", "X-Amz-Algorithm", "key", UploadFilePart}, order, "ticket fields go before the file")
	assert.Equal(t, "video/quicktime", fileType)
	assert.Equal(t, video, fileBytes)
}

func TestVideoContentType(t *testing.T) {
	assert.Equal(t, "video/mp4", VideoContentType("clip.bin", "video/mp4"), "a sniffed video type wins")
	assert.Equal(t, "video/quicktime", VideoContentType("Clip.MOV", "application/octet-stream"), "QuickTime is known by extension only")
	assert.Equal(t, "text/plain; charset=utf-8", VideoContentType("notes.txt", "text/plain; charset=utf-8"), "anything else is left to the provider to refuse")
}

func TestAssetNotReady(t *testing.T) {
	assert.True(t, AssetNotReady(http.StatusConflict, []byte(`{"error":{"code":"asset_not_ready","message":"still uploading"}}`)))
	assert.False(t, AssetNotReady(http.StatusConflict, []byte(`{"error":{"code":"invalid_asset"}}`)))
	assert.False(t, AssetNotReady(http.StatusBadRequest, []byte(`{"error":{"code":"asset_not_ready"}}`)))
}

func TestRetryAfter(t *testing.T) {
	assert.Equal(t, time.Second, RetryAfter(""))
	assert.Equal(t, 2*time.Second, RetryAfter("2"))
	assert.Equal(t, retryAfterMax, RetryAfter("60"))
	assert.Equal(t, time.Second, RetryAfter("soon"))
}
