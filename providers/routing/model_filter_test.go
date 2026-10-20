package routing

import (
	"testing"

	assert "github.com/stretchr/testify/assert"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestFilterModels(t *testing.T) {
	models := []types.Model{
		{ID: "openai/gpt-4o"},
		{ID: "groq/llama-3.3-70b"},
		{ID: "ollama/phi3"},
		{ID: "minimax/MiniMax-M2"},
	}

	tests := []struct {
		name           string
		allowed        string
		disallowed     string
		showDeprecated bool
		expected       []string
	}{
		{
			name:     "no filters hides deprecated models",
			expected: []string{"openai/gpt-4o", "groq/llama-3.3-70b", "ollama/phi3"},
		},
		{
			name:           "show deprecated returns everything",
			showDeprecated: true,
			expected:       []string{"openai/gpt-4o", "groq/llama-3.3-70b", "ollama/phi3", "minimax/MiniMax-M2"},
		},
		{
			name:     "allow list keeps a deprecated model",
			allowed:  "MiniMax-M2",
			expected: []string{"minimax/MiniMax-M2"},
		},
		{
			name:           "disallow list still applies to shown deprecated models",
			disallowed:     "MiniMax-M2",
			showDeprecated: true,
			expected:       []string{"openai/gpt-4o", "groq/llama-3.3-70b", "ollama/phi3"},
		},
		{
			name:     "allow list by full id",
			allowed:  "openai/gpt-4o",
			expected: []string{"openai/gpt-4o"},
		},
		{
			name:     "allow list by bare model name case-insensitive",
			allowed:  "GPT-4o, phi3",
			expected: []string{"openai/gpt-4o", "ollama/phi3"},
		},
		{
			name:       "disallow list removes matches",
			disallowed: "phi3",
			expected:   []string{"openai/gpt-4o", "groq/llama-3.3-70b"},
		},
		{
			name:       "allow list wins over disallow list",
			allowed:    "phi3",
			disallowed: "phi3",
			expected:   []string{"ollama/phi3"},
		},
		{
			name:           "whitespace-only allow list passes everything",
			allowed:        " , ",
			showDeprecated: true,
			expected:       []string{"openai/gpt-4o", "groq/llama-3.3-70b", "ollama/phi3", "minimax/MiniMax-M2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterModels(models, tt.allowed, tt.disallowed, tt.showDeprecated)
			ids := make([]string, 0, len(filtered))
			for _, m := range filtered {
				ids = append(ids, m.ID)
			}
			assert.Equal(t, tt.expected, ids)
		})
	}
}

func TestModelMatches(t *testing.T) {
	set := ParseModelSet("gpt-4o, anthropic/claude-3")

	assert.True(t, set["gpt-4o"])
	assert.True(t, ModelMatches(set, "openai/GPT-4o"))
	assert.True(t, ModelMatches(set, "gpt-4o"))
	assert.True(t, ModelMatches(set, "Anthropic/Claude-3"))
	assert.False(t, ModelMatches(set, "openai/gpt-3.5"))
	assert.False(t, ModelMatches(set, ""))
}
