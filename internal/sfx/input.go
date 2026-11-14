package sfx

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

const (
	dataURIPrefix   = "data:"
	dataURIBase64   = ";base64"
	dataURIComma    = ","
	paramSeparator  = ";"
	httpsScheme     = "https"
	inlineVideoName = "video"
)

var errInvalidVideo = errors.New("the '" + FieldVideo + "' field must be an https URL or a base64 data:video/... URI")

// Video is the clip a video-to-sfx request scores: a URL the provider fetches
// itself, or the decoded bytes of an inline data URI with their media type.
type Video struct {
	URL         string
	ContentType string
	Content     []byte
}

// ParseVideo reads a request's `video` field, an https URL or a base64
// data:video/... URI, decoding the inline bytes.
func ParseVideo(raw string) (Video, error) {
	if rest, inline := strings.CutPrefix(raw, dataURIPrefix); inline {
		return parseDataURI(rest)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != httpsScheme || u.Host == "" {
		return Video{}, errInvalidVideo
	}
	return Video{URL: raw}, nil
}

func parseDataURI(rest string) (Video, error) {
	meta, payload, found := strings.Cut(rest, dataURIComma)
	mediaType, isBase64 := strings.CutSuffix(meta, dataURIBase64)
	contentType, _, _ := strings.Cut(mediaType, paramSeparator)
	if !found || !isBase64 || !strings.HasPrefix(contentType, videoTypePrefix) {
		return Video{}, errInvalidVideo
	}
	content, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(content) == 0 {
		return Video{}, errInvalidVideo
	}
	return Video{ContentType: contentType, Content: content}, nil
}

// Filename names an inline clip after its media type, since a data URI
// carries no file name, so the uploaded part keeps a matching extension.
func (v Video) Filename() string {
	for extension, videoType := range videoTypesByExtension {
		if videoType == v.ContentType {
			return inlineVideoName + extension
		}
	}
	return inlineVideoName
}
