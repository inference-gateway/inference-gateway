package core

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
	client "github.com/inference-gateway/inference-gateway/providers/client"
	"github.com/inference-gateway/inference-gateway/providers/constants"
	transformers "github.com/inference-gateway/inference-gateway/providers/transformers"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// HTTPError represents an HTTP error with status code and message
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

type ProviderImpl struct {
	ID                 *types.Provider
	Name               string
	URL                string
	Token              string
	AuthType           string
	SupportsVisionFlag bool
	ExtraHeaders       map[string][]string
	Endpoints          types.Endpoints
	Client             client.Client
	Logger             l.Logger
}

func (p *ProviderImpl) GetID() *types.Provider {
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

func (p *ProviderImpl) EndpointModels() string {
	return p.Endpoints.Models
}

func (p *ProviderImpl) EndpointChat() string {
	return p.Endpoints.Chat
}

// Helper functions for common operations
func (p *ProviderImpl) buildProviderURL() string {
	return "/proxy/" + string(*p.GetID()) + p.EndpointChat()
}

func (p *ProviderImpl) prepareStreamingRequest(clientReq types.CreateChatCompletionRequest) types.CreateChatCompletionRequest {
	// Enforce usage tracking for streaming completions
	clientReq.StreamOptions = &types.ChatCompletionStreamOptions{
		IncludeUsage: true,
	}

	// Special case - cohere, mistral, and ollama_cloud don't like stream_options, so we don't
	// include it - probably they haven't implemented it yet in their OpenAI "compatible" API
	if *p.GetID() == constants.CohereID || *p.GetID() == constants.MistralID || *p.GetID() == constants.OllamaCloudID {
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

func (p *ProviderImpl) handleHTTPError(response *http.Response, operation string) error {
	if response.StatusCode == http.StatusOK {
		return nil
	}

	bodyBytes, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		p.Logger.Error("Failed to read error response body", readErr, "provider", p.GetName())
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
	p.Logger.Error("non-200 status code", err, "provider", p.GetName(), "statusCode", response.StatusCode, "operation", operation)
	return err
}

// ListModels fetches the list of models available from the provider and returns them in OpenAI compatible format
func (p *ProviderImpl) ListModels(ctx context.Context) (types.ListModelsResponse, error) {
	url := "/proxy/" + string(*p.GetID()) + p.EndpointModels()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		p.Logger.Error("Failed to create request", err, "provider", p.GetName(), "url", url)
		return types.ListModelsResponse{}, err
	}

	if authToken := ctx.Value("authToken"); authToken != nil {
		req.Header.Set("Authorization", "Bearer "+authToken.(string))
	}

	response, err := p.Client.Do(req)
	if err != nil {
		p.Logger.Error("Failed to list models", err, "provider", p.GetName(), "url", url)
		return types.ListModelsResponse{}, err
	}

	if err := p.handleHTTPError(response, "Error fetching models"); err != nil {
		return types.ListModelsResponse{}, err
	}

	var transformer constants.ListModelsTransformer
	switch *p.GetID() {
	case constants.OllamaID:
		var resp transformers.ListModelsResponseOllama
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.CloudflareID:
		var resp transformers.ListModelsResponseCloudflare
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.AnthropicID:
		var resp transformers.ListModelsResponseAnthropic
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.CohereID:
		var resp transformers.ListModelsResponseCohere
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.GroqID:
		var resp transformers.ListModelsResponseGroq
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.DeepseekID:
		var resp transformers.ListModelsResponseDeepseek
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.GoogleID:
		var resp transformers.ListModelsResponseGoogle
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.MistralID:
		var resp transformers.ListModelsResponseMistral
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.OllamaCloudID:
		var resp transformers.ListModelsResponseOllamaCloud
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	case constants.MoonshotID:
		var resp transformers.ListModelsResponseMoonshot
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	default:
		var resp transformers.ListModelsResponseOpenai
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName(), "url", url)
			return types.ListModelsResponse{}, err
		}
		transformer = &resp
	}

	return transformer.Transform(), nil
}

// ChatCompletions generates chat completions from the provider
func (p *ProviderImpl) ChatCompletions(ctx context.Context, clientReq types.CreateChatCompletionRequest) (types.CreateChatCompletionResponse, error) {
	url := p.buildProviderURL()

	reqBody, err := json.Marshal(clientReq)
	if err != nil {
		p.Logger.Error("Failed to marshal request", err, "provider", p.GetName())
		return types.CreateChatCompletionResponse{}, err
	}

	req, err := p.createHTTPRequest(ctx, url, reqBody)
	if err != nil {
		p.Logger.Error("Failed to create request", err, "provider", p.GetName(), "url", url)
		return types.CreateChatCompletionResponse{}, err
	}

	response, err := p.Client.Do(req)
	if err != nil {
		p.Logger.Error("Failed to send request", err, "provider", p.GetName(), "url", url)
		return types.CreateChatCompletionResponse{}, err
	}
	defer response.Body.Close()

	if err := p.handleHTTPError(response, "Error generating chat completion"); err != nil {
		return types.CreateChatCompletionResponse{}, err
	}

	var resp types.CreateChatCompletionResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		p.Logger.Error("Failed to unmarshal response", err, "provider", p.GetName())
		return types.CreateChatCompletionResponse{}, err
	}

	return resp, nil
}

// StreamChatCompletions generates chat completions from the provider using streaming
func (p *ProviderImpl) StreamChatCompletions(ctx context.Context, clientReq types.CreateChatCompletionRequest) (<-chan []byte, error) {
	url := p.buildProviderURL()

	streamReq := p.prepareStreamingRequest(clientReq)

	p.Logger.Debug("streaming chat completions", "provider", p.GetName(), "url", url, "request", streamReq)

	reqBody, err := json.Marshal(streamReq)
	if err != nil {
		p.Logger.Error("failed to marshal request", err, "provider", p.GetName())
		return nil, err
	}

	req, err := p.createHTTPRequest(ctx, url, reqBody)
	if err != nil {
		p.Logger.Error("failed to create request", err, "provider", p.GetName(), "url", url)
		return nil, err
	}

	response, err := p.Client.Do(req)
	if err != nil {
		p.Logger.Error("failed to send request", err, "provider", p.GetName(), "url", url)
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
				p.Logger.Debug("stream cancelled due to context", "provider", p.GetName())
				return
			default:
			}

			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					p.Logger.Error("error reading stream", err, "provider", p.GetName())
				} else {
					p.Logger.Debug("stream ended gracefully", "provider", p.GetName())
				}
				return
			}

			if len(line) > 0 {
				select {
				case stream <- line:
				case <-ctx.Done():
					p.Logger.Debug("stream cancelled while sending data", "provider", p.GetName())
					return
				}
			}
		}
	}()

	return stream, nil
}

// SupportsVision checks if the provider and model support vision/image processing
func (p *ProviderImpl) SupportsVision(ctx context.Context, model string) (bool, error) {
	if !p.SupportsVisionFlag {
		return false, nil
	}

	modelLower := strings.ToLower(model)

	switch *p.ID {
	case constants.OpenaiID:
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
	case constants.AnthropicID:
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
