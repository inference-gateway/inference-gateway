package providers

type OllamaDetails struct {
	Format            string      `json:"format"`
	Family            string      `json:"family"`
	Families          interface{} `json:"families"`
	ParameterSize     string      `json:"parameter_size"`
	QuantizationLevel string      `json:"quantization_level"`
}

type OllamaModel struct {
	Name       string        `json:"name"`
	ModifiedAt string        `json:"modified_at"`
	Size       int           `json:"size"`
	Digest     string        `json:"digest"`
	Details    OllamaDetails `json:"details"`
}

type ListModelsResponseOllama struct {
	Models []OllamaModel `json:"models"`
}

func (l *ListModelsResponseOllama) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.Name,
		})
	}
	return ListModelsResponse{
		Provider: OllamaDisplayName,
		Models:   models,
	}
}

type GenerateRequestOllama struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	System string `json:"system"`
}

type GenerateResponseOllama struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	DoneReason         string `json:"done_reason"`
	Context            []int  `json:"context"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

func (g *GenerateResponseOllama) Transform() GenerateResponse {
	return GenerateResponse{
		Provider: OllamaDisplayName,
		Response: ResponseTokens{
			Content: g.Response,
			Model:   g.Model,
			Role:    "assistant",
		},
	}
}
