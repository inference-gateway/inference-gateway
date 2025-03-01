package providers

import (
	"bufio"
	"encoding/json"

	"github.com/inference-gateway/inference-gateway/logger"
)

type CohereModel struct {
	Name             string   `json:"name"`
	Endpoints        []string `json:"endpoints"`
	Finetuned        bool     `json:"finetuned"`
	ContextLength    float64  `json:"context_length"`
	TokenizerURL     string   `json:"tokenizer_url"`
	DefaultEndpoints []string `json:"default_endpoints"`
}

type ListModelsResponseCohere struct {
	Models        []CohereModel `json:"models"`
	NextPageToken string        `json:"next_page_token"`
}

func (l *ListModelsResponseCohere) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.Name,
		})
	}
	return ListModelsResponse{
		Provider: CohereID,
		Models:   models,
	}
}

type CohereResponseFormat struct {
	Type       string                 `json:"type,omitempty"`
	JsonSchema map[string]interface{} `json:"json_schema,omitempty"`
}

type CohereCitationOptions struct {
	Content interface{} `json:"content,omitempty"`
}

type CohereDocument struct {
	Content  string                 `json:"content,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type CohereTool struct {
	Description string                 `json:"description,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type GenerateRequestCohere struct {
	Messages         []Message              `json:"messages"`
	Model            string                 `json:"model"`
	Stream           bool                   `json:"stream"`
	Tools            []CohereTool           `json:"tools,omitempty"`
	Documents        []CohereDocument       `json:"documents,omitempty"`
	CitationOptions  *CohereCitationOptions `json:"citation_options,omitempty"`
	ResponseFormat   *CohereResponseFormat  `json:"response_format,omitempty"`
	SafetyMode       string                 `json:"safety_mode,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	StopSequences    []string               `json:"stop_sequences,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	K                *float64               `json:"k,omitempty"`
	P                *float64               `json:"p,omitempty"`
	LogProbs         *bool                  `json:"logprobs,omitempty"`
	ToolChoice       string                 `json:"tool_choice,omitempty"`
	StrictTools      *bool                  `json:"strict_tools,omitempty"`
}

func (r *GenerateRequest) TransformCohere() GenerateRequestCohere {
	return GenerateRequestCohere{
		Messages:    r.Messages,
		Model:       r.Model,
		Stream:      r.Stream,
		Temperature: Float64Ptr(0.3), // Default temperature as per docs
	}
}

type CohereContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CohereMessage struct {
	Role      string          `json:"role"`
	Content   []CohereContent `json:"content,omitempty"`
	ToolPlan  string          `json:"tool_plan"`
	ToolCalls []interface{}   `json:"tool_calls"`
	Citations []interface{}   `json:"citations"`
}

type CohereUsageUnits struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type CohereUsage struct {
	BilledUnits CohereUsageUnits `json:"billed_units"`
	Tokens      CohereUsageUnits `json:"tokens"`
}

type GenerateResponseCohere struct {
	ID           string        `json:"id"`
	FinishReason string        `json:"finish_reason"`
	Message      CohereMessage `json:"message"`
	Usage        CohereUsage   `json:"usage,omitempty"`
	LogProbs     []interface{} `json:"logprobs,omitempty"`
}

func (g *GenerateResponseCohere) Transform() GenerateResponse {
	if len(g.Message.Content) == 0 {
		return GenerateResponse{}
	}

	return GenerateResponse{
		Provider: CohereDisplayName,
		Response: ResponseTokens{
			Model:   "N/A",
			Content: g.Message.Content[0].Text,
			Role:    "assistant",
		},
		Usage: &Usage{
			PromptTokens:     g.Usage.BilledUnits.InputTokens,
			CompletionTokens: g.Usage.BilledUnits.OutputTokens,
			TotalTokens:      g.Usage.BilledUnits.InputTokens + g.Usage.BilledUnits.OutputTokens,
			QueueTime:        0.0, // Cohere does not provide queue time
			PromptTime:       0.0, // Cohere does not provide prompt time
			CompletionTime:   0.0, // Cohere does not provide completion time
			TotalTime:        0.0, // Cohere does not provide total time
		},
	}
}

type CohereDeltaMessage struct {
	Role      string          `json:"role,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	ToolPlan  string          `json:"tool_plan,omitempty"`
	ToolCalls []interface{}   `json:"tool_calls,omitempty"`
	Citations []interface{}   `json:"citations,omitempty"`
}

type CohereDelta struct {
	Message CohereDeltaMessage `json:"message,omitempty"`
}

type CohereStreamResponse struct {
	Id    string      `json:"id,omitempty"`
	Type  EventType   `json:"type,omitempty"`
	Delta CohereDelta `json:"delta,omitempty"`
}

type CohereContentItem struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

func (g *CohereStreamResponse) Transform() GenerateResponse {
	var content CohereContentItem
	// Unfortunatly the content is a raw message, so we need to try and unmarshal it into CohereContentItem
	// Sometimes it's array sometimes it's a structured object - weird and magical
	if err := json.Unmarshal(g.Delta.Message.Content, &content); err != nil {
		return GenerateResponse{
			Provider: CohereDisplayName,
			Response: ResponseTokens{
				Model:   "N/A",
				Content: "",
				Role:    "assistant",
			},
			EventType: g.Type,
		}
	}
	switch g.Type {
	case EventMessageStart:
		return GenerateResponse{
			Provider: CohereDisplayName,
			Response: ResponseTokens{
				Model:   "N/A",
				Content: content.Text,
				Role:    "assistant",
			},
			EventType: EventMessageStart,
		}
	case EventContentStart:
		return GenerateResponse{
			Provider: CohereDisplayName,
			Response: ResponseTokens{
				Model:   "N/A",
				Content: content.Text,
				Role:    "assistant",
			},
			EventType: EventContentStart,
		}
	case EventContentDelta:
		return GenerateResponse{
			Provider: CohereDisplayName,
			Response: ResponseTokens{
				Model:   "N/A",
				Content: content.Text,
				Role:    "assistant",
			},
			EventType: EventContentDelta,
		}
	case EventContentEnd:
		return GenerateResponse{
			Provider: CohereDisplayName,
			Response: ResponseTokens{
				Model:   "N/A",
				Content: content.Text,
				Role:    "assistant",
			},
			EventType: EventContentEnd,
		}
	case EventMessageEnd:
		return GenerateResponse{
			Provider: CohereDisplayName,
			Response: ResponseTokens{
				Model:   "N/A",
				Content: content.Text,
				Role:    "assistant",
			},
			EventType: EventMessageEnd,
		}
	default:
		return GenerateResponse{}
	}
}

type CohereStreamParser struct {
	logger logger.Logger
}

func (p *CohereStreamParser) ParseChunk(reader *bufio.Reader) (*SSEvent, error) {
	rawchunk, err := readSSEventsChunk(reader)
	if err != nil {
		return nil, err
	}

	event, err := ParseSSEvents(rawchunk)
	if err != nil {
		return nil, err
	}

	return event, nil
}
