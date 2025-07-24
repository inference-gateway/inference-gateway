package providers

import (
	"strings"
	"time"
)

type AnthropicModel struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

type ListModelsResponseAnthropic struct {
	Data    []AnthropicModel `json:"data"`
	HasMore bool             `json:"has_more"`
	FirstID string           `json:"first_id"`
	LastID  string           `json:"last_id"`
}

func (l *ListModelsResponseAnthropic) Transform() ListModelsResponse {
	provider := AnthropicID
	var models []Model
	for _, model := range l.Data {
		t, err := time.Parse(time.RFC3339, model.CreatedAt)
		var created int64
		if err != nil {
			created = 0
		} else {
			created = t.Unix()
		}

		modelID := model.ID
		if !strings.Contains(modelID, "/") {
			modelID = string(provider) + "/" + modelID
		}
		models = append(models, Model{
			ID:       modelID,
			Object:   "model",
			Created:  created,
			OwnedBy:  string(provider),
			ServedBy: provider,
		})
	}
	return ListModelsResponse{
		Object:   "list",
		Provider: &provider,
		Data:     models,
	}
}
