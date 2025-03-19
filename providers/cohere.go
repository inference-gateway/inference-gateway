package providers

import "time"

type ModelCohere struct {
	Name             string   `json:"name,omitempty"`
	Endpoints        []string `json:"endpoints,omitempty"`
	Finetuned        bool     `json:"finetuned,omitempty"`
	ContextLenght    int32    `json:"context_length,omitempty"`
	TokenizerURL     string   `json:"tokenizer_url,omitempty"`
	SupportsVision   bool     `json:"supports_vision,omitempty"`
	DefaultEndpoints []string `json:"default_endpoints,omitempty"`
}

type ListModelsResponseCohere struct {
	NextPageToken string         `json:"next_page_token,omitempty"`
	Models        []*ModelCohere `json:"models,omitempty"`
}

func (l *ListModelsResponseCohere) Transform() ListModelsResponse {
	models := make([]*Model, len(l.Models))
	created := time.Now().Unix()
	for i, model := range l.Models {
		models[i].ID = model.Name
		models[i].Object = "model"
		models[i].Created = created // Cohere does not provide creation time
		models[i].OwnedBy = CohereID
		models[i].ServedBy = CohereID
	}

	return ListModelsResponse{
		Provider: CohereID,
		Object:   "list",
		Data:     models,
	}
}
