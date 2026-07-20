package core

import (
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// TestApplyCommunityContextWindows exercises the embedded community table:
// provider-resolved windows win, unresolved models fall back with source
// community (including ID variants like date pins), and models absent from
// the table stay nil.
func TestApplyCommunityContextWindows(t *testing.T) {
	models := []types.Model{
		{ID: "openai/gpt-4", ContextWindow: &types.ModelContextWindow{Tokens: 4096, Source: types.ContextWindowSourceProvider}},
		{ID: "openai/gpt-4"},
		{ID: "anthropic/claude-sonnet-4-5-19990101"},
		{ID: "openai/gpt-nonexistent"},
	}

	applyCommunityContextWindows(models)

	if got := models[0].ContextWindow; got.Tokens != 4096 || got.Source != types.ContextWindowSourceProvider {
		t.Errorf("provider-resolved window overwritten: %+v", got)
	}
	if got := models[1].ContextWindow; got == nil || got.Tokens <= 0 || got.Source != types.ContextWindowSourceCommunity {
		t.Errorf("unresolved model did not fall back to community table: %+v", got)
	}
	if got := models[2].ContextWindow; got == nil || got.Tokens <= 0 || got.Source != types.ContextWindowSourceCommunity {
		t.Errorf("date-pinned ID variant did not resolve in community table: %+v", got)
	}
	if got := models[3].ContextWindow; got != nil {
		t.Errorf("model absent from the table must keep a nil window, got %+v", got)
	}
}
