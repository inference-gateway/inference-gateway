package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
	ChatCompletions(ctx context.Context, req CreateChatCompletionRequest) (CreateChatCompletionResponse, error)
	StreamChatCompletions(ctx context.Context, req CreateChatCompletionRequest) (<-chan []byte, error)
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

func (p *ProviderImpl) EndpointModels() string {
	return p.endpoints.Models
}

func (p *ProviderImpl) EndpointChat() string {
	return p.endpoints.Chat
}

// ListModels fetches the list of models available from the provider and returns them in OpenAI compatible format
func (p *ProviderImpl) ListModels(ctx context.Context) (ListModelsResponse, error) {
	url := "/proxy/" + p.GetID() + p.EndpointModels()

	response, err := p.client.Get(url)
	if err != nil {
		p.logger.Error("Failed to list models", err, "provider", p.GetName(), "url", url)
		return ListModelsResponse{}, err
	}

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP error: %d - Error fetching models", response.StatusCode)
		p.logger.Error("Non-200 status code when listing models", err, "provider", p.GetName(), "statusCode", response.StatusCode)
		return ListModelsResponse{}, err
	}

	var transformer ListModelsTransformer
	switch p.GetID() {
	case OllamaID:
		var resp ListModelsResponseOllama
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case CloudflareID:
		var resp ListModelsResponseCloudflare
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case AnthropicID:
		var resp ListModelsResponseAnthropic
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case CohereID:
		var resp ListModelsResponseCohere
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case GroqID:
		var resp ListModelsResponseGroq
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	default:
		var resp ListModelsResponseOpenai
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	}

	return transformer.Transform(), nil
}

// ChatCompletions generates chat completions from the provider
func (p *ProviderImpl) ChatCompletions(ctx context.Context, req CreateChatCompletionRequest) (CreateChatCompletionResponse, error) {
	proxyURL := "/proxy/" + p.GetID() + p.EndpointChat()

	reqBody, err := json.Marshal(req)
	if err != nil {
		p.logger.Error("Failed to marshal request", err, "provider", p.GetName())
		return CreateChatCompletionResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewBuffer(reqBody))
	if err != nil {
		p.logger.Error("Failed to create request", err, "provider", p.GetName(), "url", proxyURL)
		return CreateChatCompletionResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(httpReq)
	if err != nil {
		p.logger.Error("Failed to send request", err, "provider", p.GetName(), "url", proxyURL)
		return CreateChatCompletionResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP error: %d - Error generating chat completion", response.StatusCode)
		p.logger.Error("Non-200 status code", err, "provider", p.GetName(), "statusCode", response.StatusCode)
		return CreateChatCompletionResponse{}, err
	}

	var resp CreateChatCompletionResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName())
		return CreateChatCompletionResponse{}, err
	}

	return resp, nil
}

// StreamChatCompletions generates chat completions from the provider using streaming
func (p *ProviderImpl) StreamChatCompletions(ctx context.Context, req CreateChatCompletionRequest) (<-chan []byte, error) {
	proxyURL := "/proxy/" + p.GetID() + p.EndpointChat()

	// Ollama doesn't include it by default, so we enforce it here
	req.StreamOptions.IncludeUsage = true

	p.logger.Debug("Streaming chat completions", "provider", p.GetName(), "url", proxyURL, "request", req)

	reqBody, err := json.Marshal(req)
	if err != nil {
		p.logger.Error("Failed to marshal request", err, "provider", p.GetName())
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewBuffer(reqBody))
	if err != nil {
		p.logger.Error("Failed to create request", err, "provider", p.GetName(), "url", proxyURL)
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	response, err := p.client.Do(httpReq)
	if err != nil {
		p.logger.Error("Failed to send request", err, "provider", p.GetName(), "url", proxyURL)
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		err := fmt.Errorf("HTTP error: %d - Error generating streaming chat completion", response.StatusCode)
		p.logger.Error("Non-200 status code", err, "provider", p.GetName(), "statusCode", response.StatusCode)
		return nil, err
	}

	stream := make(chan []byte, 100)
	go func() {
		defer response.Body.Close()
		defer close(stream)

		reader := bufio.NewReader(response.Body)

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err.Error() != "EOF" {
					p.logger.Error("Error reading stream", err, "provider", p.GetName())
				} else {
					p.logger.Debug("Stream ended gracefully", "provider", p.GetName())
				}
				return
			}

			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			if bytes.HasPrefix(line, []byte("data: ")) {
				line = bytes.TrimPrefix(line, []byte("data: "))

				if bytes.Equal(line, []byte("[DONE]")) {
					p.logger.Debug("Stream completed", "provider", p.GetName())
					return
				}

				select {
				case stream <- line:
				case <-ctx.Done():
					p.logger.Debug("Stream context canceled", "provider", p.GetName())
					return
				}
			}
		}
	}()

	return stream, nil
}
