package providers

import (
	"bufio"
	"bytes"

	"github.com/inference-gateway/inference-gateway/logger"
)

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
		Provider: OllamaID,
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
	if len(r.Messages) > 1 && r.Messages[0].Role == MessageRoleSystem {
		systemPrompt = r.Messages[0].Content
	}

	return GenerateRequestOllama{
		Model:  r.Model,
		Prompt: lastMessage,
		System: systemPrompt,
		Stream: r.Stream,
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
	DoneReason         string `json:"done_reason,omitempty"`
	Context            []int  `json:"context,omitempty"`
	TotalDuration      int64  `json:"total_duration,omitempty"`
	LoadDuration       int64  `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int64  `json:"eval_duration,omitempty"`
}

func (g *GenerateResponseOllama) Transform() GenerateResponse {
	event := EventContentDelta
	if g.Done {
		event = EventStreamEnd
	}

	return GenerateResponse{
		Provider: OllamaDisplayName,
		Response: ResponseTokens{
			Content: g.Response,
			Model:   g.Model,
			Role:    "assistant",
		},
		EventType: event,
	}
}

type OllamaStreamParser struct {
	logger logger.Logger
}

func (p *OllamaStreamParser) ParseChunk(reader *bufio.Reader) (*SSEvent, error) {
	// It's good that they kept it simple, raw bytes
	// so no need to pass it through parseSSEvents
	rawchunk, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	event := EventStreamStart

	// It's a weird API where we have to check for "done" to determine if it's a delta or end event
	// Better would be if there were some metadata in the stream, so we don't have to search in the entire json for "done"
	// But it is what it is - hopefully they improve it in the future
	if bytes.Contains(rawchunk, []byte(`"done":false`)) {
		event = EventContentDelta
	}

	if bytes.Contains(rawchunk, []byte(`"done":true`)) {
		event = EventStreamEnd
	}

	return &SSEvent{
		EventType: event,
		Data:      rawchunk,
	}, nil
}
