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

// Advanced options for Ollama model generation
type OllamaOptions struct {
	NumKeep          *int     `json:"num_keep,omitempty"`
	Seed             *int     `json:"seed,omitempty"`
	NumPredict       *int     `json:"num_predict,omitempty"`
	TopK             *int     `json:"top_k,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	MinP             *float64 `json:"min_p,omitempty"`
	TypicalP         *float64 `json:"typical_p,omitempty"`
	RepeatLastN      *int     `json:"repeat_last_n,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	RepeatPenalty    *float64 `json:"repeat_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	Mirostat         *int     `json:"mirostat,omitempty"`
	MirostatTau      *float64 `json:"mirostat_tau,omitempty"`
	MirostatEta      *float64 `json:"mirostat_eta,omitempty"`
	PenalizeNewline  *bool    `json:"penalize_newline,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	NumCtx           *int     `json:"num_ctx,omitempty"`
	NumBatch         *int     `json:"num_batch,omitempty"`
	NumGPU           *int     `json:"num_gpu,omitempty"`
	MainGPU          *int     `json:"main_gpu,omitempty"`
	LowVRAM          *bool    `json:"low_vram,omitempty"`
	VocabOnly        *bool    `json:"vocab_only,omitempty"`
	UseMMap          *bool    `json:"use_mmap,omitempty"`
	UseMlock         *bool    `json:"use_mlock,omitempty"`
	NumThread        *int     `json:"num_thread,omitempty"`
}

type GenerateRequestOllama struct {
	Model     string         `json:"model"`
	Prompt    string         `json:"prompt"`
	System    string         `json:"system,omitempty"`
	Template  string         `json:"template,omitempty"`
	Context   []int          `json:"context,omitempty"`
	Stream    bool           `json:"stream"`
	Raw       bool           `json:"raw,omitempty"`
	Format    interface{}    `json:"format,omitempty"`
	Options   *OllamaOptions `json:"options,omitempty"`
	Images    []string       `json:"images,omitempty"`
	KeepAlive string         `json:"keep_alive,omitempty"`
}

func (r *GenerateRequest) TransformOllama() GenerateRequestOllama {
	// Get the last message content as prompt since Ollama expects a single prompt
	lastMessage := r.Messages[len(r.Messages)-1].Content

	// Use first message as system prompt if it exists and is a system message
	var systemPrompt string
	if len(r.Messages) > 1 && r.Messages[0].Role == RoleSystem {
		systemPrompt = r.Messages[0].Content
	}

	return GenerateRequestOllama{
		Model:  r.Model,
		Prompt: lastMessage,
		System: systemPrompt,
		Stream: false, // Default to non-streaming
		Options: &OllamaOptions{
			Temperature: float64Ptr(0.7), // Default temperature
		},
	}
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
