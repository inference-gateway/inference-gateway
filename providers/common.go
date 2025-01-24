package providers

import (
	"encoding/json"
	"errors"
	"net/http"
)

//go:generate mockgen -source=common.go -destination=../tests/mocks/provider.go -package=mocks
type Provider interface {
	GetID() string
	GetName() string
	GetURL() string
	GetProxyURL() string
	GetToken() string
	GetAuthType() string
	GetExtraXHeaders() map[string]string
	BuildGenTokensRequest(model string, messages []GenerateMessage) interface{}
	BuildGenTokensResponse(model string, responseBody interface{}) (GenerateResponse, error)
}

type ProviderImpl struct {
	ID            string
	Name          string
	URL           string
	ProxyURL      string
	Token         string
	AuthType      string
	ExtraXHeaders map[string]string
}

type ModelsResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func FetchModels(url string, provider string) ModelsResponse {
	resp, err := http.Get(url)
	if err != nil {
		return ModelsResponse{Provider: provider, Models: []interface{}{}}
	}
	defer resp.Body.Close()

	switch provider {
	case "google":
		var response GetModelsResponseGoogle
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Models}

	case "cloudflare":
		var response GetModelsResponseCloudflare
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: provider, Models: []interface{}{}}
		}
		return ModelsResponse{Provider: provider, Models: response.Result}

	case "cohere":
		var response GetModelsResponseCohere
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

func (p *ProviderImpl) BuildGenTokensRequest(model string, messages []GenerateMessage) interface{} {
	switch p.ID {
	case "ollama":
		return GenerateRequestOllama{
			Model:  model,
			Prompt: getUserMessage(messages),
			Stream: false,
			System: getSystemMessage(messages),
		}
	case "groq":
		return GenerateRequestGroq{
			Model:    model,
			Messages: messages,
		}
	case "openai":
		return GenerateRequestOpenAI{
			Model:    model,
			Messages: messages,
		}
	case "google":
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
	case "cloudflare":
		prompt := getSystemMessage(messages) + getUserMessage(messages)
		return GenerateRequestCloudflare{
			Prompt: prompt,
		}
	case "cohere":
		return GenerateRequestCohere{
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

	switch p.ID {
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

type GenerateMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model    string            `json:"model"`
	Messages []GenerateMessage `json:"messages"`
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

func (p *ProviderImpl) GetID() string {
	return p.ID
}

func (p *ProviderImpl) GetName() string {
	return p.Name
}

func (p *ProviderImpl) GetURL() string {
	return p.URL
}

func (p *ProviderImpl) GetProxyURL() string {
	return p.ProxyURL
}

func (p *ProviderImpl) GetToken() string {
	return p.Token
}

func (p *ProviderImpl) GetAuthType() string {
	return p.AuthType
}

func (p *ProviderImpl) GetExtraXHeaders() map[string]string {
	return p.ExtraXHeaders
}

func getSystemMessage(messages []GenerateMessage) string {
	for _, message := range messages {
		if message.Role == "system" {
			return message.Content
		}
	}
	return ""
}

func getUserMessage(messages []GenerateMessage) string {
	var prompt string
	for _, message := range messages {
		if message.Role == "user" {
			prompt += message.Content
		}
	}
	return prompt
}
