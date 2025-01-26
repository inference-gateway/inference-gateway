package providers

import (
	"errors"
)

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
		return GenerateRequestOpenai{
			Model:    model,
			Messages: messages,
		}
	case GoogleID:
		_ = getSystemMessage(messages) + getUserMessage(messages)

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

	return nil
}

func (p *ProviderImpl) BuildGenTokensResponse(model string, responseBody interface{}) (GenerateResponse, error) {
	var role, content string

	switch p.GetID() {
	case "ollama":
		// ollamaResponse := responseBody.(*GenerateResponseOllama)
		// if ollamaResponse.Response != "" {
		// 	role = "assistant" // It's not provided by Ollama so we set it to assistant
		// 	content = ""
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from Ollama")
		// }
	case "groq":
		// groqResponse := responseBody.(*GenerateResponseGroq)
		// if len(groqResponse.Choices) > 0 && len(groqResponse.Choices[0].Message.Content) > 0 {
		// 	role = groqResponse.Choices[0].Message.Role
		// 	content = groqResponse.Choices[0].Message.Content
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from Groq")
		// }
	case "openai":
		// openAIResponse := responseBody.(*GenerateResponseOpenai)
		// if len(openAIResponse.Choices) > 0 && len(openAIResponse.Choices[0].Message.Content) > 0 {
		// 	role = openAIResponse.Choices[0].Message.Role
		// 	content = openAIResponse.Choices[0].Message.Content
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from OpenAI")
		// }
	case "google":
		// googleResponse := responseBody.(*GenerateResponseGoogle)
		// if len(googleResponse.Candidates) > 0 && len(googleResponse.Candidates[0].Content.Parts) > 0 {
		// 	role = googleResponse.Candidates[0].Content.Role
		// 	content = googleResponse.Candidates[0].Content.Parts[0].Text
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from Google")
		// }
	case "cloudflare":
		// cloudflareResponse := responseBody.(*GenerateResponseCloudflare)
		// if cloudflareResponse.Result.Response != "" {
		// 	role = "assistant"
		// 	content = cloudflareResponse.Result.Response
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from Cloudflare")
		// }
	case "cohere":
		// cohereResponse := responseBody.(*GenerateResponseCohere)
		// if len(cohereResponse.Message.Content) > 0 && cohereResponse.Message.Content[0].Text != "" {
		// 	role = cohereResponse.Message.Role
		// 	content = cohereResponse.Message.Content[0].Text
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from Cohere")
		// }
	case "anthropic":
		// anthropicResponse := responseBody.(*GenerateResponseAnthropic)
		// if len(anthropicResponse.Choices) > 0 && len(anthropicResponse.Choices[0].Message.Content) > 0 {
		// 	role = anthropicResponse.Choices[0].Message.Role
		// 	content = anthropicResponse.Choices[0].Message.Content
		// } else {
		// 	return GenerateResponse{}, errors.New("invalid response from Anthropic")
		// }
	default:
		return GenerateResponse{}, errors.New("provider not implemented")
	}

	return GenerateResponse{Provider: p.GetName(), Response: ResponseTokens{
		Role:    role,
		Model:   model,
		Content: content,
	}}, nil
}
