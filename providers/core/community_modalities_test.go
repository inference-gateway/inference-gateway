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
