package providers

type GoogleModel struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                float64  `json:"temperature"`
	MaxTemperature             float64  `json:"maxTemperature"`
	TopP                       float64  `json:"topP"`
	TopK                       int      `json:"topK"`
}

type ListModelsResponseGoogle struct {
	Models []GoogleModel `json:"models"`
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

type GooglePart struct {
	Text string `json:"text"`
}

type GoogleContent struct {
	Parts []GooglePart `json:"parts"`
	Role  string       `json:"role"`
}

type GenerateRequestGoogle struct {
	Contents []GoogleContent `json:"contents"`
}

func (r *GenerateRequest) TransformGoogle() GenerateRequestGoogle {
	contents := make([]GoogleContent, len(r.Messages))
	for i, msg := range r.Messages {
		contents[i] = GoogleContent{
			Role: msg.Role,
			Parts: []GooglePart{
				{Text: msg.Content},
			},
		}
	}
	return GenerateRequestGoogle{
		Contents: contents,
	}
}

type GoogleCandidate struct {
	Content       GoogleContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	Index         int           `json:"index"`
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings"`
}

type GooglePromptFeedback struct {
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings"`
	BlockReason string `json:"blockReason,omitempty"`
}

type GoogleUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type GenerateResponseGoogle struct {
	Candidates     []GoogleCandidate    `json:"candidates"`
	PromptFeedback GooglePromptFeedback `json:"promptFeedback"`
	UsageMetadata  GoogleUsageMetadata  `json:"usageMetadata"`
	ModelVersion   string               `json:"modelVersion"`
}

func (g *GenerateResponseGoogle) Transform() GenerateResponse {
	if len(g.Candidates) == 0 {
		return GenerateResponse{}
	}

	return GenerateResponse{
		Provider: GoogleDisplayName,
		Response: ResponseTokens{
			Role:    g.Candidates[0].Content.Role,
			Content: g.Candidates[0].Content.Parts[0].Text,
			Model:   g.ModelVersion,
		},
	}
}
