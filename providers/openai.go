package providers

type OpenaiPermission struct {
	ID                 string `json:"id"`
	Object             string `json:"object"`
	Created            int64  `json:"created"`
	AllowCreateEngine  bool   `json:"allow_create_engine"`
	AllowSampling      bool   `json:"allow_sampling"`
	AllowLogprobs      bool   `json:"allow_logprobs"`
	AllowSearchIndices bool   `json:"allow_search_indices"`
	AllowView          bool   `json:"allow_view"`
	AllowFineTuning    bool   `json:"allow_fine_tuning"`
}

type OpenaiModel struct {
	ID         string             `json:"id"`
	Object     string             `json:"object"`
	Created    int64              `json:"created"`
	OwnedBy    string             `json:"owned_by"`
	Permission []OpenaiPermission `json:"permission"`
	Root       string             `json:"root"`
	Parent     string             `json:"parent"`
}

type ListModelsResponseOpenai struct {
	Object string        `json:"object"`
	Data   []OpenaiModel `json:"data"`
}

func (l *ListModelsResponseOpenai) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		models = append(models, Model{
			Name: model.ID,
		})
	}
	return ListModelsResponse{
		Provider: OpenaiDisplayName,
		Models:   models,
	}
}

type GenerateRequestOpenai struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseOpenai struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
