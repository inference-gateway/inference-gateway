package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	l "github.com/inference-gateway/inference-gateway/logger"
)

//go:generate mockgen -source=management.go -destination=../tests/mocks/provider.go -package=mocks
type Provider interface {
	// Getters
	GetID() string
	GetName() string
	GetURL() string
	GetToken() string
	GetAuthType() string
	GetExtraHeaders() map[string][]string

	// Fetchers
	ListModels(ctx context.Context) (ListModelsResponse, error)
	GenerateTokens(ctx context.Context, model string, messages []Message, tools []Tool, maxTokens int) (GenerateResponse, error)
	StreamTokens(ctx context.Context, model string, messages []Message) (<-chan GenerateResponse, error)
}

type ProviderImpl struct {
	id           string
	name         string
	url          string
	token        string
	authType     string
	extraHeaders map[string][]string
	endpoints    Endpoints
	client       Client
	logger       l.Logger
}

func (p *ProviderImpl) GetID() string {
	return p.id
}

func (p *ProviderImpl) GetName() string {
	return p.name
}

func (p *ProviderImpl) GetURL() string {
	return p.url
}

func (p *ProviderImpl) GetToken() string {
	return p.token
}

func (p *ProviderImpl) GetAuthType() string {
	return p.authType
}

func (p *ProviderImpl) GetExtraHeaders() map[string][]string {
	return p.extraHeaders
}

func (p *ProviderImpl) EndpointList() string {
	return p.endpoints.List
}

func (p *ProviderImpl) EndpointGenerate() string {
	return p.endpoints.Generate
}

func (p *ProviderImpl) ListModels(ctx context.Context) (ListModelsResponse, error) {
	baseURL, err := url.Parse(p.GetURL())
	if err != nil {
		p.logger.Error("failed to parse base URL", err)
		return ListModelsResponse{}, fmt.Errorf("failed to parse base URL: %v", err)
	}

	url := "/proxy/" + p.GetID() + baseURL.Path + p.EndpointList()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		p.logger.Error("failed to create request", err)
		return ListModelsResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("failed to make request", err, "provider", p.GetName())
		return ListModelsResponse{
			Provider: p.GetID(),
			Models:   make([]Model, 0),
		}, fmt.Errorf("failed to reach provider %s: %w", p.GetName(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ListModelsResponse{
			Provider: p.GetID(),
			Models:   make([]Model, 0),
		}, fmt.Errorf("failed with status code: %d", resp.StatusCode)
	}

	switch p.GetID() {
	case OllamaID:
		var response ListModelsResponseOllama
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case GroqID:
		var response ListModelsResponseGroq
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case OpenaiID:
		var response ListModelsResponseOpenai
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case CloudflareID:
		var response ListModelsResponseCloudflare
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case CohereID:
		var response ListModelsResponseCohere
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case AnthropicID:
		var response ListModelsResponseAnthropic
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	default:
		p.logger.Error("provider not found", nil, "provider", p.GetName())
		return ListModelsResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}
}

func (p *ProviderImpl) GenerateTokens(ctx context.Context, model string, messages []Message, tools []Tool, maxTokens int) (GenerateResponse, error) {
	if p == nil {
		return GenerateResponse{}, errors.New("provider cannot be nil")
	}

	baseURL, err := url.Parse(p.GetURL())
	if err != nil {
		p.logger.Error("failed to parse base URL", err)
		return GenerateResponse{}, fmt.Errorf("failed to parse base URL: %v", err)
	}

	url := "/proxy/" + p.GetID() + baseURL.Path + p.EndpointGenerate()
	if p.GetID() == CloudflareID {
		url = strings.Replace(url, "{model}", model, 1)
	}

	genRequest := GenerateRequest{
		Model:     model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: maxTokens,
	}

	switch p.GetID() {
	case OllamaID:
		// Request
		payload := genRequest.TransformOllama()
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal request", err)
			return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := fetchTokens(ctx, p.client, url, payloadBytes, p.logger)
		if err != nil {
			p.logger.Error("failed to make request", err)
			return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
		}

		// Response
		var response GenerateResponseOllama
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err)
			return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case GroqID:
		// Request
		payload := genRequest.TransformGroq()
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal request", err)
			return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := fetchTokens(ctx, p.client, url, payloadBytes, p.logger)
		if err != nil {
			p.logger.Error("failed to make request", err)
			return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
		}

		// Response
		var response GenerateResponseGroq
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err)
			return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case OpenaiID:
		// Request
		payload := genRequest.TransformOpenai()
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal request", err)
			return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := fetchTokens(ctx, p.client, url, payloadBytes, p.logger)
		if err != nil {
			p.logger.Error("failed to make request", err)
			return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
		}

		// Response
		var response GenerateResponseOpenai
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case CloudflareID:
		// Request
		payload := genRequest.TransformCloudflare()
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal request", err)
			return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := fetchTokens(ctx, p.client, url, payloadBytes, p.logger)
		if err != nil {
			p.logger.Error("failed to make request", err)
			return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
		}

		// Response
		var response GenerateResponseCloudflare
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case CohereID:
		// Request
		payload := genRequest.TransformCohere()
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal request", err)
			return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := fetchTokens(ctx, p.client, url, payloadBytes, p.logger)
		if err != nil {
			p.logger.Error("failed to make request", err)
			return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
		}

		// Response
		var response GenerateResponseCohere
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	case AnthropicID:
		// Request
		payload := genRequest.TransformAnthropic()
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal request", err)
			return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := fetchTokens(ctx, p.client, url, payloadBytes, p.logger)
		if err != nil {
			p.logger.Error("failed to make request", err)
			return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
		}

		// Response
		var response GenerateResponseAnthropic
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			p.logger.Error("failed to decode response", err, "provider", p.GetName())
			return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response.Transform(), nil
	default:
		p.logger.Error("unsupported provider", nil)
		return GenerateResponse{}, fmt.Errorf("unsupported provider: %s", p.GetID())
	}
}

func (p *ProviderImpl) StreamTokens(ctx context.Context, model string, messages []Message) (<-chan GenerateResponse, error) {
	if p == nil {
		return nil, errors.New("provider cannot be nil")
	}

	baseURL, err := url.Parse(p.GetURL())
	if err != nil {
		p.logger.Error("failed to parse base URL", err)
		return nil, fmt.Errorf("failed to parse base URL: %v", err)
	}

	url := "/proxy/" + p.GetID() + baseURL.Path + p.EndpointGenerate()
	if p.GetID() == CloudflareID {
		url = strings.Replace(url, "{model}", model, 1)
	}

	streamCh := make(chan GenerateResponse)

	genRequest := GenerateRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	// Transform request based on provider
	var payloadBytes []byte
	switch p.GetID() {
	case OllamaID:
		payload := genRequest.TransformOllama()
		payloadBytes, err = json.Marshal(payload)
	case OpenaiID:
		payload := genRequest.TransformOpenai()
		payloadBytes, err = json.Marshal(payload)
	case GroqID:
		payload := genRequest.TransformGroq()
		payloadBytes, err = json.Marshal(payload)
	case CloudflareID:
		if genRequest.Stream {
			p.logger.Error("streaming not supported for Cloudflare provider", nil)
			return nil, fmt.Errorf("streaming is not supported for Cloudflare provider")
		}
		payload := genRequest.TransformCloudflare()
		payloadBytes, err = json.Marshal(payload)
	case CohereID:
		payload := genRequest.TransformCohere()
		payloadBytes, err = json.Marshal(payload)
	case AnthropicID:
		payload := genRequest.TransformAnthropic()
		payloadBytes, err = json.Marshal(payload)
	default:
		p.logger.Error("unsupported provider", nil)
		return nil, fmt.Errorf("unsupported provider")
	}

	if err != nil {
		p.logger.Error("failed to marshal request", err)
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		p.logger.Error("failed to create request", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("failed to make request", err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	reader := bufio.NewReader(resp.Body)

	go func() {
		defer resp.Body.Close()
		defer close(streamCh)

		for {
			streamParser, err := NewStreamParser(p.logger, p.GetID())
			if err != nil {
				p.logger.Error("failed to create stream parser", err)
				return
			}

			event, err := streamParser.ParseChunk(reader)
			if err != nil {
				if err == io.EOF {
					return
				}
				p.logger.Error("failed to read chunk", err)
				return
			}

			if event.EventType == EventStreamEnd {
				// Close the channel by returning
				return
			}

			var chunk interface{}
			switch p.GetID() {
			case OllamaID:
				var response GenerateResponseOllama
				if err := json.Unmarshal(event.Data, &response); err != nil {
					p.logger.Error("failed to unmarshal chunk", err)
					continue
				}
				chunk = response
			case OpenaiID:
				var response GenerateResponseOpenai
				if err := json.Unmarshal(event.Data, &response); err != nil {
					p.logger.Error("failed to unmarshal chunk", err)
					continue
				}
				chunk = response
			case GroqID:
				var response GenerateResponseGroq
				if err := json.Unmarshal(event.Data, &response); err != nil {
					p.logger.Error("failed to unmarshal chunk", err)
					continue
				}
				chunk = response
			case CloudflareID:
				var response GenerateResponseCloudflare
				if err := json.Unmarshal(event.Data, &response); err != nil {
					p.logger.Error("failed to unmarshal chunk", err)
					continue
				}
				chunk = response
			case CohereID:
				var response CohereStreamResponse
				if err := json.Unmarshal(event.Data, &response); err != nil {
					p.logger.Error("failed to unmarshal chunk", err)
					return
				}
				chunk = response
			case AnthropicID:
				var response GenerateResponseAnthropic
				if err := json.Unmarshal(event.Data, &response); err != nil {
					p.logger.Error("failed to unmarshal chunk", err)
					continue
				}
				chunk = response
			default:
				p.logger.Error("unsupported provider for streaming", nil)
				return
			}

			// Transform and send chunk
			select {
			case <-ctx.Done():
				return
			default:
				switch v := chunk.(type) {
				case GenerateResponseOllama:
					streamCh <- v.Transform()
				case GenerateResponseOpenai:
					streamCh <- v.Transform()
				case GenerateResponseGroq:
					streamCh <- v.Transform()
				case GenerateResponseCloudflare:
					streamCh <- v.Transform()
				case CohereStreamResponse:
					streamCh <- v.Transform()
				case GenerateResponseAnthropic:
					streamCh <- v.Transform()
				default:
					p.logger.Error("unsupported response type", nil)
					return
				}
			}
		}
	}()

	return streamCh, nil
}

func fetchTokens(ctx context.Context, client Client, url string, payload []byte, logger l.Logger) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		logger.Error("failed to create request", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to make request", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("request failed", fmt.Errorf("status code: %d", resp.StatusCode))
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	return resp, nil
}
