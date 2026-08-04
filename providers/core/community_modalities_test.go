package core

import (
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// TestApplyCommunityModalities exercises the embedded community table:
// provider-resolved modalities win, unresolved models fall back from the table
// (including ID variants like date pins), and models absent from the table stay
// nil.
func TestApplyCommunityModalities(t *testing.T) {
	provided := []types.ModelModalities{types.ModelModalitiesText}
	models := []types.Model{
		{ID: "openai/gpt-4o", Modalities: &provided},
		{ID: "openai/gpt-4o"},
		{ID: "anthropic/claude-opus-4-8-19990101"},
		{ID: "openai/gpt-nonexistent"},
	}

	applyCommunityModalities(models)

	if got := models[0].Modalities; got == nil || len(*got) != 1 || (*got)[0] != types.ModelModalitiesText {
		t.Errorf("provider-resolved modalities overwritten: %+v", got)
	}
	if got := models[1].Modalities; got == nil || len(*got) == 0 {
		t.Errorf("unresolved model did not fall back to community table: %+v", got)
	}
	if got := models[2].Modalities; got == nil || len(*got) == 0 {
		t.Errorf("date-pinned ID variant did not resolve in community table: %+v", got)
	}
	if got := models[3].Modalities; got != nil {
		t.Errorf("model absent from the table must keep nil modalities, got %+v", got)
	}
}
