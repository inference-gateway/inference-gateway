package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

//go:generate mockgen -source=management.go -destination=../tests/mocks/provider.go -package=mocks
type Provider interface {
	GetID() string
	GetName() string
	GetURL() string
	GetToken() string
	GetAuthType() string
	GetExtraHeaders() map[string][]string

	ListModels() ModelsResponse
	GenerateTokens(model string, messages []Message, client http.Client) (GenerateResponse, error)
}

type ProviderImpl struct {
	ID           string
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    struct {
		List     string
		Generate string
	}
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

func (p *ProviderImpl) EndpointList() string {
	return p.Endpoints.List
}

func (p *ProviderImpl) EndpointGenerate() string {
	return p.Endpoints.Generate
}

func (p *ProviderImpl) ListModels() ModelsResponse {
	// resp, err := http.Get(p.GetListURL())
	resp, err := http.Get("")
	if err != nil {
		return ModelsResponse{Provider: p.GetName(), Models: []interface{}{}}
	}
	defer resp.Body.Close()

	switch p.GetID() {
	case GoogleID:
		var response GetModelsResponseGoogle
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: p.GetName(), Models: []interface{}{}}
		}
		return ModelsResponse{Provider: p.GetName(), Models: response.Models}
	case CloudflareID:
		var response GetModelsResponseCloudflare
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: p.GetName(), Models: []interface{}{}}
		}
		return ModelsResponse{Provider: p.GetName(), Models: response.Result}
	case CohereID:
		var response GetModelsResponseCohere
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: p.GetName(), Models: []interface{}{}}
		}
		return ModelsResponse{Provider: p.GetName(), Models: response.Models}
	case AnthropicID:
		var response GetModelsResponseAnthropic
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: p.GetName(), Models: []interface{}{}}
		}
		return ModelsResponse{Provider: p.GetName(), Models: response.Models}
	default:
		var response GetModelsResponse
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsResponse{Provider: p.GetName(), Models: []interface{}{}}
		}
		return ModelsResponse{Provider: p.GetName(), Models: response.Data}
	}
}

func (p *ProviderImpl) GenerateTokens(model string, messages []Message, client http.Client) (GenerateResponse, error) {
	if p == nil {
		return GenerateResponse{}, errors.New("provider cannot be nil")
	}

	// TODO - build provider generate URL
	var url string

	providerName := p.GetName()
	if providerName == "Google" || providerName == "Cloudflare" {
		// providerGenTokensURL = strings.Replace(providerGenTokensURL, "{model}", req.Model, 1)
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

	resp, err := client.Do(req)
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
