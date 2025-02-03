package providers

// HuggingFaceModel represents the model details returned from the HuggingFace API.
type HuggingfaceModel struct {
	_ID           string   `json:"_id"`
	ID            string   `json:"id"`
	Likes         int      `json:"likes"`
	TrendingScore int      `json:"trending_score"`
	Private       bool     `json:"private"`
	Downloads     int      `json:"downloads"`
	Tags          []string `json:"tags"`
	PipelineTag   string   `json:"pipeline_tag"`
	LibraryName   string   `json:"library_name"`
	CreatedAt     string   `json:"created_at"`
	ModelID       string   `json:"model_id"`
}

// ListModelsResponseHuggingface wraps the API response for listing models.
type ListModelsResponseHuggingface []HuggingfaceModel

// Transform converts the provider-specific response to the common ListModelsResponse.
func (l *ListModelsResponseHuggingface) Transform() ListModelsResponse {
	var models []Model
	for _, m := range *l {
		models = append(models, Model{
			Name: m.ID,
		})
	}
	return ListModelsResponse{
		Provider: HuggingfaceID,
		Models:   models,
	}
}

// GenerateRequestHuggingface models the request body for generating text.
type GenerateRequestHuggingface struct {
	Inputs     string                 `json:"inputs"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// TransformHuggingface converts a generic GenerateRequest to a HuggingFace-specific request.
// Here we use the first message's content as input.
func (r *GenerateRequest) TransformHuggingface() GenerateRequestHuggingface {
	input := ""
	if len(r.Messages) > 0 {
		input = r.Messages[0].Content
	}
	return GenerateRequestHuggingface{
		Inputs:     input,
		Parameters: map[string]interface{}{},
		Options:    map[string]interface{}{},
	}
}

// GenerateResponseHuggingface models the response body from the HuggingFace generate endpoint.
type GenerateResponseHuggingface struct {
	GeneratedText string `json:"generated_text"`
}

// Transform converts the HuggingFace-specific response to the common GenerateResponse.
func (r *GenerateResponseHuggingface) Transform() GenerateResponse {
	response := ResponseTokens{
		Content: r.GeneratedText,
		Model:   "", // Set model name if needed
		Role:    MessageRoleAssistant,
	}

	return GenerateResponse{
		Provider:  HuggingfaceDisplayName,
		Response:  response,
		EventType: EventContentDelta,
	}
}
