package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestActiveMinimaxModelsPreservesAvailableMetadata(t *testing.T) {
	models := []types.Model{
		{ID: "minimax/MiniMax-M2"},
		{ID: "minimax/MiniMax-M2.1"},
		{ID: "minimax/MiniMax-M2.1-highspeed"},
		{ID: "minimax/MiniMax-M2.5"},
		{ID: "minimax/MiniMax-M2.5-highspeed"},
		{ID: "minimax/MiniMax-M2.7"},
		{ID: "minimax/MiniMax-M2.7-highspeed"},
		{ID: "minimax/speech-2.8-hd", OwnedBy: "media"},
		{ID: minimaxM3Model, OwnedBy: "provider"},
		{ID: "minimax/custom-model", Created: 42},
		{ID: minimaxFlashModel, ContextWindow: &types.ContextWindow{Tokens: 1000000}},
	}
	original := append([]types.Model(nil), models...)
	want := []types.Model{models[10], models[8], models[7], models[9]}
	if got := ActiveMinimaxModels(models); !reflect.DeepEqual(got, want) {
		t.Fatalf("active models = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(models, original) {
		t.Fatal("model listing mutated its input")
	}
	want = []types.Model{models[8], models[7], models[9]}
	if got := ActiveMinimaxModels(models[:10]); !reflect.DeepEqual(got, want) {
		t.Fatalf("unavailable Flash must not be added: %+v", got)
	}
	if got := ActiveMinimaxModels(nil); len(got) != 0 {
		t.Fatalf("empty upstream catalog = %+v", got)
	}
}

func TestCommunityContextOverridesMatchGeneratedTable(t *testing.T) {
	data, err := os.ReadFile("community_context_windows.overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	var overrides map[string]communityContextWindow
	if err := json.Unmarshal(data, &overrides); err != nil {
		t.Fatal(err)
	}
	for id, want := range overrides {
		if got := communityContextWindows()[id]; got != want {
			t.Errorf("generated context for %q = %+v, want override %+v", id, got, want)
		}
	}
}
