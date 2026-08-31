package core

import (
	"slices"
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// TestApplyCommunityModalities exercises the embedded community table:
// provider-resolved modalities win, unresolved models fall back from the table
// (including ID variants like date pins), and models absent from the table stay
// nil.
func TestApplyCommunityModalities(t *testing.T) {
	provided := types.ModelModalities{
		Input:  []types.Modality{types.ModalityText},
		Output: []types.Modality{types.ModalityText},
	}
	models := []types.Model{
		{ID: "openai/gpt-4o", Modalities: &provided},
		{ID: "openai/gpt-4o"},
		{ID: "anthropic/claude-opus-4-8-19990101"},
		{ID: "openai/gpt-nonexistent"},
	}

	applyCommunityModalities(models)

	if got := models[0].Modalities; got == nil || len(got.Input) != 1 || got.Input[0] != types.ModalityText {
		t.Errorf("provider-resolved modalities overwritten: %+v", got)
	}
	if got := models[1].Modalities; got == nil || len(got.Input) == 0 || len(got.Output) == 0 {
		t.Errorf("unresolved model did not fall back to community table: %+v", got)
	}
	if got := models[2].Modalities; got == nil || len(got.Input) == 0 {
		t.Errorf("date-pinned ID variant did not resolve in community table: %+v", got)
	}
	if got := models[3].Modalities; got != nil {
		t.Errorf("model absent from the table must keep nil modalities, got %+v", got)
	}
}

// TestCommunityModalitiesTableDistinguishesImageGen verifies the embedded
// table carries the output side: gpt-image-2 must be image-out without
// text-out (image-generation), while gpt-4o outputs text (chat).
func TestCommunityModalitiesTableDistinguishesImageGen(t *testing.T) {
	models := []types.Model{
		{ID: "openai/gpt-image-2"},
		{ID: "openai/gpt-4o"},
	}

	applyCommunityModalities(models)

	imageGen := models[0].Modalities
	if imageGen == nil {
		t.Fatal("gpt-image-2 missing from community table")
	}
	if len(imageGen.Output) == 0 || slices.Contains(imageGen.Output, types.ModalityText) {
		t.Errorf("gpt-image-2 output must be image-only, got %+v", imageGen.Output)
	}
	chat := models[1].Modalities
	if chat == nil || !slices.Contains(chat.Output, types.ModalityText) {
		t.Errorf("gpt-4o must output text, got %+v", chat)
	}
}

// TestCommunityModalitiesOverridesCoverSpeechModels verifies the hand-curated
// overlay survives a re-sync into the embedded table: speech models absent
// from models.dev must resolve with the correct audio side.
func TestCommunityModalitiesOverridesCoverSpeechModels(t *testing.T) {
	models := []types.Model{
		{ID: "openai/tts-1"},
		{ID: "openai/whisper-1"},
		{ID: "groq/playai-tts"},
	}

	applyCommunityModalities(models)

	tts := models[0].Modalities
	if tts == nil || !slices.Contains(tts.Output, types.ModalityAudio) || slices.Contains(tts.Output, types.ModalityText) {
		t.Errorf("tts-1 must be audio-out without text-out, got %+v", tts)
	}
	stt := models[1].Modalities
	if stt == nil || !slices.Contains(stt.Input, types.ModalityAudio) {
		t.Errorf("whisper-1 must be audio-in, got %+v", stt)
	}
	if models[2].Modalities == nil {
		t.Error("playai-tts missing from community table")
	}
}

// TestModelAcceptsImages verifies the per-model vision gate: table-confirmed
// vision models pass (including date-pinned IDs), table-confirmed text-only
// models fail, and unknown models pass so requests are never silently stripped.
func TestModelAcceptsImages(t *testing.T) {
	tests := []struct {
		name     string
		provider types.Provider
		model    string
		want     bool
	}{
		{"vision model with date pin", "anthropic", "claude-haiku-4-5-20251001", true},
		{"vision model", "anthropic", "claude-haiku-4-5", true},
		{"text-only model", "deepseek", "deepseek-v4-flash", false},
		{"unknown model is permissive", "openai", "gpt-nonexistent", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ModelAcceptsImages(tt.provider, tt.model); got != tt.want {
				t.Errorf("ModelAcceptsImages(%s, %s) = %v, want %v", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}
