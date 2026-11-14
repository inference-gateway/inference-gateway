package elevenlabs

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	imageFieldImage      = "image"
	imageFieldImageArray = "image[]"
	imageFieldMask       = "mask"
	imageFieldPrompt     = "prompt"
	imageFieldSize       = "size"
	imageFieldQuality    = "quality"
	imageFieldCount      = "n"
	imageSizeAuto        = "auto"
	singleImage          = "1"
	aspectRatioFormat    = "%d:%d"

	generationCompleted = "completed"
	generationFailed    = "failed"

	imagePollInterval = 2 * time.Second
)

// imageQualities are the `quality` values ElevenLabs shares with OpenAI. The
// others (`auto`, `standard`) leave the model's default.
var imageQualities = []string{"low", "medium", "high"}

// rejections are the failure reasons that turn the request itself down.
var rejections = []string{"moderated", "invalid_parameters"}

// imageBody is the ElevenLabs POST /flows/image payload of an edit: a prompt
// and the reference images to edit or draw from.
type imageBody struct {
	ModelID     string         `json:"model_id"`
	Prompt      string         `json:"prompt"`
	Images      []*inlineMedia `json:"images"`
	Mask        *inlineMedia   `json:"mask,omitempty"`
	AspectRatio string         `json:"aspect_ratio,omitempty"`
	Quality     string         `json:"quality,omitempty"`
}

// ImageEdit rewrites an OpenAI Images edit form into the ElevenLabs flows JSON
// shape. Every `image` part becomes a reference image and `size` becomes an
// aspect ratio. model is the request model with the provider prefix stripped.
func ImageEdit(model string, form *multipart.Form) ([]byte, error) {
	if n := formValue(form, imageFieldCount); n != "" && n != singleImage {
		return nil, fmt.Errorf("elevenlabs generates one image per request, so 'n' must be 1")
	}

	out := imageBody{ModelID: model, Prompt: formValue(form, imageFieldPrompt)}
	for _, fh := range slices.Concat(form.File[imageFieldImage], form.File[imageFieldImageArray]) {
		image, err := inlineHeader(fh, imageFieldImage)
		if err != nil {
			return nil, err
		}
		out.Images = append(out.Images, image)
	}

	var err error
	if out.Mask, err = inlineFile(form, imageFieldMask); err != nil {
		return nil, err
	}
	if out.AspectRatio, err = aspectRatio(formValue(form, imageFieldSize)); err != nil {
		return nil, err
	}
	if quality := formValue(form, imageFieldQuality); slices.Contains(imageQualities, quality) {
		out.Quality = quality
	}
	return json.Marshal(out)
}

// aspectRatio turns an OpenAI `size` ("1024x1536") into an ElevenLabs
// aspect_ratio ("2:3"). `auto` is passed on and no size leaves the model's
// default.
func aspectRatio(size string) (string, error) {
	if size == "" || size == imageSizeAuto {
		return size, nil
	}
	w, h, ok := strings.Cut(size, sizeSeparator)
	width, werr := strconv.Atoi(w)
	height, herr := strconv.Atoi(h)
	if !ok || werr != nil || herr != nil || width <= 0 || height <= 0 {
		return "", fmt.Errorf("the 'size' field must be WIDTHxHEIGHT or auto, e.g. 1024x1536")
	}
	g := gcd(width, height)
	return fmt.Sprintf(aspectRatioFormat, width/g, height/g), nil
}

// ImageAccount is the ElevenLabs account an image is generated in. Each call
// returns the response body, or a *Refusal when ElevenLabs declines it.
type ImageAccount interface {
	CreateImage(ctx context.Context, generation []byte) ([]byte, error)
	Image(ctx context.Context, generationID string) ([]byte, error)
}

// GeneratedImage is a finished generation: the signed URL its image can be
// downloaded from for about an hour, and the image's type.
type GeneratedImage struct {
	URL      string
	MimeType string
}

// GenerationFailure is ElevenLabs failing a generation it had accepted.
type GenerationFailure struct {
	Reason  string
	Message string
}

func (f *GenerationFailure) Error() string { return f.Message }

// Rejected reports whether ElevenLabs turned the request itself down, as
// opposed to breaking while it rendered.
func (f *GenerationFailure) Rejected() bool {
	return slices.Contains(rejections, f.Reason)
}

type generation struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ContentURL      string `json:"content_url"`
	ContentMimeType string `json:"content_mime_type"`
	FailureReason   string `json:"failure_reason"`
	ErrorMessage    string `json:"error_message"`
}

// GenerateImage starts an image generation in the account and waits for its
// image. ElevenLabs renders an image as a job, where the OpenAI Images API
// answers with the image, so the wait happens here. ctx bounds it.
func GenerateImage(ctx context.Context, account ImageAccount, body []byte) (GeneratedImage, error) {
	raw, err := account.CreateImage(ctx, body)
	if err != nil {
		return GeneratedImage{}, err
	}
	var created generation
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return GeneratedImage{}, fmt.Errorf("elevenlabs did not return a generation id")
	}

	next := time.NewTicker(imagePollInterval)
	defer next.Stop()
	for {
		image, finished, err := finishedImage(ctx, account, created.ID)
		if err != nil || finished {
			return image, err
		}
		select {
		case <-ctx.Done():
			return GeneratedImage{}, ctx.Err()
		case <-next.C:
		}
	}
}

// finishedImage looks at the generation once and reports its image when it is
// complete, or its failure.
func finishedImage(ctx context.Context, account ImageAccount, generationID string) (GeneratedImage, bool, error) {
	raw, err := account.Image(ctx, generationID)
	if err != nil {
		return GeneratedImage{}, false, err
	}
	var state generation
	if err := json.Unmarshal(raw, &state); err != nil {
		return GeneratedImage{}, false, fmt.Errorf("reading the elevenlabs generation: %w", err)
	}
	switch state.Status {
	case generationCompleted:
		return GeneratedImage{URL: state.ContentURL, MimeType: state.ContentMimeType}, true, nil
	case generationFailed:
		return GeneratedImage{}, false, &GenerationFailure{Reason: state.FailureReason, Message: state.ErrorMessage}
	default:
		return GeneratedImage{}, false, nil
	}
}
