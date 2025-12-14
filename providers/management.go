package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	l "github.com/inference-gateway/inference-gateway/logger"
)

// Helper functions for common operations
func (p *ProviderImpl) buildProviderURL() string {
	return "/proxy/" + string(*p.GetID()) + p.EndpointChat()
}

func (p *ProviderImpl) prepareStreamingRequest(clientReq CreateChatCompletionRequest) CreateChatCompletionRequest {
	// Enforce usage tracking for streaming completions
	clientReq.StreamOptions = &ChatCompletionStreamOptions{
		IncludeUsage: true,
	}

	// Special case - cohere, mistral, and ollama_cloud don't like stream_options, so we don't
	// include it - probably they haven't implemented it yet in their OpenAI "compatible" API
	if *p.GetID() == CohereID || *p.GetID() == MistralID || *p.GetID() == OllamaCloudID {
		clientReq.StreamOptions = nil
	}

	return clientReq
}

func (p *ProviderImpl) createHTTPRequest(ctx context.Context, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	return req, nil
}

// HTTPError represents an HTTP error with status code and message
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

func (p *ProviderImpl) handleHTTPError(response *http.Response, operation string) error {
	if response.StatusCode == http.StatusOK {
		return nil
	}

	bodyBytes, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		p.logger.Error("Failed to read error response body", readErr, "provider", p.GetName())
		return &HTTPError{
			StatusCode: response.StatusCode,
			Message:    fmt.Sprintf("failed to read response body (status %d)", response.StatusCode),
		}
	}

	errorMsg := string(bodyBytes)
	err := &HTTPError{
		StatusCode: response.StatusCode,
		Message:    errorMsg,
	}
	p.logger.Error("Non-200 status code", err, "provider", p.GetName(), "statusCode", response.StatusCode, "operation", operation)
	return err
}

//go:generate mockgen -source=management.go -destination=../tests/mocks/providers/management.go -package=providersmocks
type IProvider interface {
	// Getters
	GetID() *Provider
	GetName() string
	GetURL() string
	GetToken() string
	GetAuthType() string
	GetExtraHeaders() map[string][]string

	// Fetchers
	ListModels(ctx context.Context) (ListModelsResponse, error)
	ChatCompletions(ctx context.Context, clientReq CreateChatCompletionRequest) (CreateChatCompletionResponse, error)
	StreamChatCompletions(ctx context.Context, clientReq CreateChatCompletionRequest) (<-chan []byte, error)
	SupportsVision(ctx context.Context, model string) (bool, error)
}

type ProviderImpl struct {
	id             *Provider
	name           string
	url            string
	token          string
	authType       string
	supportsVision bool
	extraHeaders   map[string][]string
	endpoints      Endpoints
	client         Client
	logger         l.Logger
}

func (p *ProviderImpl) GetID() *Provider {
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
	url := "/proxy/" + string(*p.GetID()) + p.EndpointModels()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		p.logger.Error("Failed to create request", err, "provider", p.GetName(), "url", url)
		return ListModelsResponse{}, err
	}

	if authToken := ctx.Value("authToken"); authToken != nil {
		req.Header.Set("Authorization", "Bearer "+authToken.(string))
	}

	response, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("Failed to list models", err, "provider", p.GetName(), "url", url)
		return ListModelsResponse{}, err
	}

	if err := p.handleHTTPError(response, "Error fetching models"); err != nil {
		return ListModelsResponse{}, err
	}

	var transformer ListModelsTransformer
	switch *p.GetID() {
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
	case DeepseekID:
		var resp ListModelsResponseDeepseek
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case GoogleID:
		var resp ListModelsResponseGoogle
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case MistralID:
		var resp ListModelsResponseMistral
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return ListModelsResponse{}, err
		}
		transformer = &resp
	case OllamaCloudID:
		var resp ListModelsResponseOllamaCloud
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
func (p *ProviderImpl) ChatCompletions(ctx context.Context, clientReq CreateChatCompletionRequest) (CreateChatCompletionResponse, error) {
	url := p.buildProviderURL()

	reqBody, err := json.Marshal(clientReq)
	if err != nil {
		p.logger.Error("Failed to marshal request", err, "provider", p.GetName())
		return CreateChatCompletionResponse{}, err
	}

	req, err := p.createHTTPRequest(ctx, url, reqBody)
	if err != nil {
		p.logger.Error("Failed to create request", err, "provider", p.GetName(), "url", url)
		return CreateChatCompletionResponse{}, err
	}

	response, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("Failed to send request", err, "provider", p.GetName(), "url", url)
		return CreateChatCompletionResponse{}, err
	}
	defer response.Body.Close()

	if err := p.handleHTTPError(response, "Error generating chat completion"); err != nil {
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
func (p *ProviderImpl) StreamChatCompletions(ctx context.Context, clientReq CreateChatCompletionRequest) (<-chan []byte, error) {
	url := p.buildProviderURL()

	streamReq := p.prepareStreamingRequest(clientReq)

	p.logger.Debug("streaming chat completions", "provider", p.GetName(), "url", url, "request", streamReq)

	reqBody, err := json.Marshal(streamReq)
	if err != nil {
		p.logger.Error("failed to marshal request", err, "provider", p.GetName())
		return nil, err
	}

	req, err := p.createHTTPRequest(ctx, url, reqBody)
	if err != nil {
		p.logger.Error("failed to create request", err, "provider", p.GetName(), "url", url)
		return nil, err
	}

	response, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("failed to send request", err, "provider", p.GetName(), "url", url)
		return nil, err
	}

	if err := p.handleHTTPError(response, "Error generating streaming chat completion"); err != nil {
		response.Body.Close()
		return nil, err
	}

	stream := make(chan []byte, 100)
	go func() {
		defer response.Body.Close()
		defer close(stream)

		reader := bufio.NewReaderSize(response.Body, 4096)

		for {
			select {
			case <-ctx.Done():
				p.logger.Debug("stream cancelled due to context", "provider", p.GetName())
				return
			default:
			}

			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					p.logger.Error("error reading stream", err, "provider", p.GetName())
				} else {
					p.logger.Debug("stream ended gracefully", "provider", p.GetName())
				}
				return
			}

			if len(line) > 0 {
				select {
				case stream <- line:
				case <-ctx.Done():
					p.logger.Debug("stream cancelled while sending data", "provider", p.GetName())
					return
				}
			}
		}
	}()

	return stream, nil
}

// SupportsVision checks if the provider and model support vision/image processing
func (p *ProviderImpl) SupportsVision(ctx context.Context, model string) (bool, error) {
	if !p.supportsVision {
		return false, nil
	}

	modelLower := strings.ToLower(model)

	switch *p.id {
	case OpenaiID:
		if strings.Contains(modelLower, "gpt-5") {
			return true, nil
		}

		if strings.Contains(modelLower, "gpt-4.1") {
			return true, nil
		}

		if strings.Contains(modelLower, "gpt-4") &&
			(strings.Contains(modelLower, "vision") ||
				strings.Contains(modelLower, "turbo") ||
				strings.Contains(modelLower, "gpt-4o")) {
			return true, nil
		}
		return false, nil
	case AnthropicID:
		return strings.Contains(modelLower, "claude-3") ||
			strings.Contains(modelLower, "opus-4") ||
			strings.Contains(modelLower, "sonnet-4") ||
			strings.Contains(modelLower, "haiku-4"), nil
	default:
		return strings.Contains(modelLower, "vision") ||
			strings.Contains(modelLower, "multimodal") ||
			strings.Contains(modelLower, "-vl") ||
			strings.Contains(modelLower, "qwen") && strings.Contains(modelLower, "vl"), nil
	}
}
