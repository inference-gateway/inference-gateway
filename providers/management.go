package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	l "github.com/inference-gateway/inference-gateway/logger"
)

//go:generate mockgen -source=management.go -destination=../tests/mocks/provider.go -package=mocks
type Provider interface {
	GetID() string
	GetName() string
	GetURL() string
	GetToken() string
	GetAuthType() string
	GetExtraHeaders() map[string][]string
	GetClient() Client

	ListModels() (ListModelsResponse, error)
	GenerateTokens(model string, messages []Message) (GenerateResponse, error)
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

func NewProvider(cfg map[string]*Config, id string, logger *l.Logger, client *Client) (Provider, error) {
	provider, ok := cfg[id]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", id)
	}

	if provider.AuthType != AuthTypeNone && provider.Token == "" {
		return nil, fmt.Errorf("provider %s token not configured", id)
	}

	return &ProviderImpl{
		id:           provider.ID,
		name:         provider.Name,
		url:          provider.URL,
		token:        provider.Token,
		authType:     provider.AuthType,
		extraHeaders: provider.ExtraHeaders,
		endpoints:    provider.Endpoints,
		client:       *client,
		logger:       *logger,
	}, nil
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

func (p *ProviderImpl) SetClient(client Client) {
	p.client = client
}

func (p *ProviderImpl) GetClient() Client {
	return p.client
}

func (p *ProviderImpl) ListModels() (ListModelsResponse, error) {
	url := "/proxy/" + p.GetID() + p.EndpointList()

	p.logger.Debug("list models", "url", url)
	resp, err := p.client.Get(url)
	if err != nil {
		p.logger.Error("failed to make request", err, "provider", p.GetName())
		return ListModelsResponse{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

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
	case GoogleID:
		var response ListModelsResponseGoogle
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

func (p *ProviderImpl) GenerateTokens(model string, messages []Message) (GenerateResponse, error) {
	if p == nil {
		return GenerateResponse{}, errors.New("provider cannot be nil")
	}

	url := "/proxy/" + p.GetID() + p.EndpointGenerate()
	providerName := p.GetName()
	if providerName == "Google" || providerName == "Cloudflare" {
		url = strings.Replace(url, "{model}", model, 1)
	}

	payload := p.BuildGenTokensRequest(model, messages)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GenerateResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	result, err := p.BuildGenTokensResponse(model, response)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("failed to build response: %w", err)
	}

	return result, nil
}
