package providers

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
