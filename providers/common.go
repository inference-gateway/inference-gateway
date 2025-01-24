package providers

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Provider defines common interface for all providers
//
//go:generate mockgen -source=common.go -destination=../tests/mocks/provider.go -package=mocks
type Provider interface {
	GetID() string
	GetName() string
	GetURL() string
	GetToken() string
	GetAuthType() string
	GetExtraHeaders() map[string][]string
	GetAPIVersion() string
}

// BaseProvider implements common provider functionality
type ProviderImpl struct {
	ID           string
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
}

func (p *ProviderImpl) GetID() string {
	return p.ID
}

func (p *ProviderImpl) GetName() string {
	return p.Name
}

func (p *ProviderImpl) GetURL() string {
	return p.URL
}

func (p *ProviderImpl) GetToken() string {
	return p.Token
}

func (p *ProviderImpl) GetAuthType() string {
	return p.AuthType
}

func (p *ProviderImpl) GetExtraHeaders() map[string][]string {
	return p.ExtraHeaders
}

type ModelsResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

var listEndpoints = map[string]string{
	"ollama":     "/v1/models",
	"groq":       "/openai/v1/models",
	"openai":     "/v1/models",
	"google":     "/v1beta/models",
	"cloudflare": "/ai/finetunes/public",
	"cohere":     "/v1/models",
	"anthropic":  "/v1/models",
}

// ListLLMsEndpoints returns the endpoints for listing models
func (p *ProviderImpl) ListLLMsEndpoints() map[string]string {
	return listEndpoints
}

var generateEndpoints = map[string]string{
	"ollama":     "/api/generate",
	"groq":       "/openai/v1/chat/completions",
	"openai":     "/v1/completions",
	"google":     "/v1beta/models/{model}:generateContent",
	"cloudflare": "/ai/run/@cf/meta/{model}",
	"cohere":     "/v2/chat",
	"anthropic":  "/v1/messages",
}

// GenTokensEndpoint returns the endpoint for generating tokens for the given provider.
func (p *ProviderImpl) GenTokensEndpoint(providerID string) string {
	return generateEndpoints[providerID]
}

func FetchModels(url string, provider string) ModelsResponse {
	resp, err := http.Get(url)
	if err != nil {
		return ModelsResponse{Provider: provider, Models: []interface{}{}}
	}
	defer resp.Body.Close()

	switch provider {
	case GoogleID:
		var response GetModelsResponseGoogle
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Models}
	case CloudflareID:
		var response GetModelsResponseCloudflare
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Result}
	case CohereID:
		var response GetModelsResponseCohere
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Models}
	case AnthropicID:
		var response GetModelsResponseAnthropic
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Models}
	default:
		var response GetModelsResponse
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Data}
	}
}

func (p *ProviderImpl) BuildGenTokensRequest(model string, messages []Message) interface{} {
	switch p.GetID() {
	case OllamaID:
		return GenerateRequestOllama{
			Model:  model,
			Prompt: getUserMessage(messages),
			Stream: false,
			System: getSystemMessage(messages),
		}
	case GroqID:
		return GenerateRequestGroq{
			Model:    model,
			Messages: messages,
		}
	case OpenaiID:
		return GenerateRequestOpenAI{
			Model:    model,
			Messages: messages,
		}
	case GoogleID:
		prompt := getSystemMessage(messages) + getUserMessage(messages)
		return GenerateRequestGoogle{
			Contents: GenerateRequestGoogleContents{
				Parts: []GenerateRequestGoogleParts{
					{
						Text: prompt,
					},
				},
			},
		}
	case CloudflareID:
		prompt := getSystemMessage(messages) + getUserMessage(messages)
		return GenerateRequestCloudflare{
			Prompt: prompt,
		}
	case CohereID:
		return GenerateRequestCohere{
			Model:    model,
			Messages: messages,
		}
	case AnthropicID:
		return GenerateRequestAnthropic{
			Model:    model,
			Messages: messages,
		}
	default:
		return GenerateRequest{
			Model:    model,
			Messages: messages,
		}
	}
}

func (p *ProviderImpl) BuildGenTokensResponse(model string, responseBody interface{}) (GenerateResponse, error) {
	var role, content string

	switch p.GetID() {
	case "ollama":
		ollamaResponse := responseBody.(*GenerateResponseOllama)
		if ollamaResponse.Response != "" {
			role = "assistant" // It's not provided by Ollama so we set it to assistant
			content = ollamaResponse.Response
		} else {
			return GenerateResponse{}, errors.New("invalid response from Ollama")
		}
	case "groq":
		groqResponse := responseBody.(*GenerateResponseGroq)
		if len(groqResponse.Choices) > 0 && len(groqResponse.Choices[0].Message.Content) > 0 {
			role = groqResponse.Choices[0].Message.Role
			content = groqResponse.Choices[0].Message.Content
		} else {
			return GenerateResponse{}, errors.New("invalid response from Groq")
		}
	case "openai":
		openAIResponse := responseBody.(*GenerateResponseOpenAI)
		if len(openAIResponse.Choices) > 0 && len(openAIResponse.Choices[0].Message.Content) > 0 {
			role = openAIResponse.Choices[0].Message.Role
			content = openAIResponse.Choices[0].Message.Content
		} else {
			return GenerateResponse{}, errors.New("invalid response from OpenAI")
		}
	case "google":
		googleResponse := responseBody.(*GenerateResponseGoogle)
		if len(googleResponse.Candidates) > 0 && len(googleResponse.Candidates[0].Content.Parts) > 0 {
			role = googleResponse.Candidates[0].Content.Role
			content = googleResponse.Candidates[0].Content.Parts[0].Text
		} else {
			return GenerateResponse{}, errors.New("invalid response from Google")
		}
	case "cloudflare":
		cloudflareResponse := responseBody.(*GenerateResponseCloudflare)
		if cloudflareResponse.Result.Response != "" {
			role = "assistant"
			content = cloudflareResponse.Result.Response
		} else {
			return GenerateResponse{}, errors.New("invalid response from Cloudflare")
		}
	case "cohere":
		cohereResponse := responseBody.(*GenerateResponseCohere)
		if len(cohereResponse.Message.Content) > 0 && cohereResponse.Message.Content[0].Text != "" {
			role = cohereResponse.Message.Role
			content = cohereResponse.Message.Content[0].Text
		} else {
			return GenerateResponse{}, errors.New("invalid response from Cohere")
		}
	case "anthropic":
		anthropicResponse := responseBody.(*GenerateResponseAnthropic)
		if len(anthropicResponse.Choices) > 0 && len(anthropicResponse.Choices[0].Message.Content) > 0 {
			role = anthropicResponse.Choices[0].Message.Role
			content = anthropicResponse.Choices[0].Message.Content
		} else {
			return GenerateResponse{}, errors.New("invalid response from Anthropic")
		}
	default:
		return GenerateResponse{}, errors.New("provider not implemented")
	}

	return GenerateResponse{Provider: p.Name, Response: ResponseTokens{
		Role:    role,
		Model:   model,
		Content: content,
	}}, nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ResponseTokens struct {
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content string `json:"content"`
}

type GetModelsResponse struct {
	Object string        `json:"object"`
	Data   []interface{} `json:"data"`
}

type GenerateResponse struct {
	Provider string         `json:"provider"`
	Response ResponseTokens `json:"response"`
}

func getSystemMessage(messages []Message) string {
	for _, message := range messages {
		if message.Role == "system" {
			return message.Content
		}
	}
	return ""
}

func getUserMessage(messages []Message) string {
	var prompt string
	for _, message := range messages {
		if message.Role == "user" {
			prompt += message.Content
		}
	}
	return prompt
}
