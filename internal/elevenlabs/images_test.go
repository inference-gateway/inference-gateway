package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"testing/synctest"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const (
	testImageModel       = "gpt-image-2"
	testEditPrompt       = "the same woman seen from her left side"
	testGenerationID     = "gen-image-123"
	testImageURL         = "https://cdn.elevenlabs.io/signed/portrait.png"
	testImageMimeType    = "image/png"
	testCreated          = `{"id":"` + testGenerationID + `","status":"pending"}`
	testPending          = `{"id":"` + testGenerationID + `","status":"pending"}`
	testGenerating       = `{"id":"` + testGenerationID + `","status":"generating"}`
	testCompleted        = `{"id":"` + testGenerationID + `","status":"completed","content_url":"` + testImageURL + `","content_mime_type":"` + testImageMimeType + `"}`
	testModerated        = `{"id":"` + testGenerationID + `","status":"failed","failure_reason":"moderated","error_message":"` + testModeratedMessage + `"}`
	testModelError       = `{"id":"` + testGenerationID + `","status":"failed","failure_reason":"model_error","error_message":"the model broke"}`
	testModeratedMessage = "The image was blocked by moderation."
)

func TestImageEdit(t *testing.T) {
	inlined := map[string]any{
		"type":           inlineMediaType,
		"content_base64": base64.StdEncoding.EncodeToString([]byte(testImageBytes)),
		"mime_type":      "text/plain; charset=utf-8",
	}

	tests := []struct {
		name    string
		fields  map[string]string
		files   map[string]int
		want    map[string]any
		wantErr string
	}{
		{
			name:   "a portrait size becomes its aspect ratio",
			fields: map[string]string{imageFieldPrompt: testEditPrompt, imageFieldSize: "1024x1536", imageFieldQuality: "high"},
			files:  map[string]int{imageFieldImage: 1},
			want: map[string]any{
				"model_id": testImageModel, "prompt": testEditPrompt, "images": []any{inlined},
				"aspect_ratio": "2:3", "quality": "high",
			},
		},
		{
			name:   "an auto size lets the model follow the photo",
			fields: map[string]string{imageFieldPrompt: testEditPrompt, imageFieldSize: imageSizeAuto},
			files:  map[string]int{imageFieldImage: 1},
			want:   map[string]any{"model_id": testImageModel, "prompt": testEditPrompt, "images": []any{inlined}, "aspect_ratio": imageSizeAuto},
		},
		{
			name:   "no size and an openai-only quality leave the model defaults",
			fields: map[string]string{imageFieldPrompt: testEditPrompt, imageFieldQuality: "standard"},
			files:  map[string]int{imageFieldImage: 1},
			want:   map[string]any{"model_id": testImageModel, "prompt": testEditPrompt, "images": []any{inlined}},
		},
		{
			name:   "every image of the array form is a reference",
			fields: map[string]string{imageFieldPrompt: testEditPrompt},
			files:  map[string]int{imageFieldImageArray: 2},
			want:   map[string]any{"model_id": testImageModel, "prompt": testEditPrompt, "images": []any{inlined, inlined}},
		},
		{
			name:   "a mask marks where to edit",
			fields: map[string]string{imageFieldPrompt: testEditPrompt},
			files:  map[string]int{imageFieldImage: 1, imageFieldMask: 1},
			want:   map[string]any{"model_id": testImageModel, "prompt": testEditPrompt, "images": []any{inlined}, "mask": inlined},
		},
		{
			name:   "one image may be asked for by number",
			fields: map[string]string{imageFieldPrompt: testEditPrompt, imageFieldCount: "1"},
			files:  map[string]int{imageFieldImage: 1},
			want:   map[string]any{"model_id": testImageModel, "prompt": testEditPrompt, "images": []any{inlined}},
		},
		{
			name:    "more than one image is refused",
			fields:  map[string]string{imageFieldPrompt: testEditPrompt, imageFieldCount: "2"},
			files:   map[string]int{imageFieldImage: 1},
			wantErr: "'n' must be 1",
		},
		{
			name:    "a size that is not WIDTHxHEIGHT is refused",
			fields:  map[string]string{imageFieldPrompt: testEditPrompt, imageFieldSize: "large"},
			files:   map[string]int{imageFieldImage: 1},
			wantErr: "WIDTHxHEIGHT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := ImageEdit(testImageModel, newForm(t, tt.fields, tt.files))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			assert.Equal(t, tt.want, body)
		})
	}
}

// fakeImageAccount stands in for an ElevenLabs account: it accepts one
// generation and answers each look at it with the next scripted state.
type fakeImageAccount struct {
	created     string
	createErr   error
	states      []string
	looks       int
	generation  []byte
	lookedAtIDs []string
}

func (a *fakeImageAccount) CreateImage(_ context.Context, generation []byte) ([]byte, error) {
	a.generation = generation
	return []byte(a.created), a.createErr
}

func (a *fakeImageAccount) Image(_ context.Context, generationID string) ([]byte, error) {
	a.lookedAtIDs = append(a.lookedAtIDs, generationID)
	state := a.states[min(a.looks, len(a.states)-1)]
	a.looks++
	return []byte(state), nil
}

func TestGenerateImage_WaitsForTheGenerationToComplete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		account := &fakeImageAccount{created: testCreated, states: []string{testPending, testGenerating, testCompleted}}

		image, err := GenerateImage(t.Context(), account, []byte(`{"prompt":"p"}`))

		require.NoError(t, err)
		assert.Equal(t, GeneratedImage{URL: testImageURL, MimeType: testImageMimeType}, image)
		assert.JSONEq(t, `{"prompt":"p"}`, string(account.generation))
		assert.Equal(t, []string{testGenerationID, testGenerationID, testGenerationID}, account.lookedAtIDs)
	})
}

func TestGenerateImage_ReportsAFailedGeneration(t *testing.T) {
	tests := []struct {
		name         string
		state        string
		wantRejected bool
		wantMessage  string
	}{
		{name: "moderation turns the request down", state: testModerated, wantRejected: true, wantMessage: testModeratedMessage},
		{name: "a model error is elevenlabs' own", state: testModelError, wantRejected: false, wantMessage: "the model broke"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &fakeImageAccount{created: testCreated, states: []string{tt.state}}

			_, err := GenerateImage(t.Context(), account, nil)

			var failure *GenerationFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, tt.wantRejected, failure.Rejected())
			assert.Equal(t, tt.wantMessage, failure.Error())
		})
	}
}

func TestGenerateImage_RelaysARefusedCreate(t *testing.T) {
	refusal := NewRefusal(402, []byte(`{"detail":{"message":"paid_plan_required"}}`))
	account := &fakeImageAccount{createErr: refusal}

	_, err := GenerateImage(t.Context(), account, nil)

	require.ErrorIs(t, err, refusal)
	assert.Zero(t, account.looks, "a refused generation is never looked at")
}

func TestGenerateImage_NeedsAGenerationID(t *testing.T) {
	account := &fakeImageAccount{created: `{"status":"pending"}`}

	_, err := GenerateImage(t.Context(), account, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generation id")
}

func TestGenerateImage_StopsWaitingWhenTheCallerGivesUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		account := &fakeImageAccount{created: testCreated, states: []string{testPending}}
		ctx, cancel := context.WithTimeout(t.Context(), 3*imagePollInterval)
		defer cancel()

		_, err := GenerateImage(ctx, account, nil)

		require.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
		assert.Greater(t, account.looks, 1, "the generation is looked at again while it is pending")
	})
}
