package providers

type ListModelsResponseGoogle struct {
	Models []Model `json:"models"`
}

func (l *ListModelsResponseGoogle) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.Name,
		})
	}
	return ListModelsResponse{
		Provider: GoogleDisplayName,
		Models:   models,
	}
}

type GenerateRequestGoogle struct {
	Contents struct{} `json:"contents"`
}

type GenerateResponseGoogle struct {
	Candidates []struct{} `json:"candidates"`
}
