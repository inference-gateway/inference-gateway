package providers

import (
	"context"

	l "github.com/inference-gateway/inference-gateway/logger"
)

// DeepseekProvider implements the Provider interface for deepseek.
type DeepseekProvider struct {
	ProviderImpl
}

// NewDeepseekProvider creates a new instance of deepseek provider.
func NewDeepseekProvider(cfg *Config, logger l.Logger, client Client) Provider {
	return &DeepseekProvider{
		ProviderImpl: ProviderImpl{
			id:           cfg.ID,
			name:         "Deepseek",
			url:          cfg.URL,
			token:        cfg.Token,
			authType:     cfg.AuthType,
			extraHeaders: cfg.ExtraHeaders,
			endpoints:    cfg.Endpoints,
			client:       client,
			logger:       logger,
		},
	}
}

// GenerateTokens returns a dummy response concatenating all message contents.
func (p *DeepseekProvider) GenerateTokens(ctx context.Context, model string, messages []Message) (GenerateResponse, error) {
	content := ""
	for _, m := range messages {
		content += m.Content + " "
	}
	resp := GenerateResponse{
		Provider: p.GetID(),
		Response: ResponseTokens{
			Content: content,
			Model:   model,
			Role:    "assistant",
		},
	}
	return resp, nil
}

// StreamTokens starts a streaming response by sending a single dummy token.
func (p *DeepseekProvider) StreamTokens(ctx context.Context, model string, messages []Message) (<-chan GenerateResponse, error) {
	ch := make(chan GenerateResponse)
	go func() {
		defer close(ch)
		resp, err := p.GenerateTokens(ctx, model, messages)
		if err != nil {
			p.logger.Error("failed to generate tokens", err)
			return
		}
		ch <- resp
	}()
	return ch, nil
}
