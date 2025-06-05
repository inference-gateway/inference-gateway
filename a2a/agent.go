package a2a

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

// MaxAgentIterations limits the number of agent loop iterations
const MaxAgentIterations = 10

// Agent defines the interface for running agent operations
//
//go:generate mockgen -source=agent.go -destination=../tests/mocks/a2a/agent.go -package=a2amocks
type Agent interface {
	Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error
	RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error
	SetProvider(provider providers.IProvider)
	SetModel(model *string)
}

// Ensure agentImpl implements Agent interface at compile time
var _ Agent = (*agentImpl)(nil)

// agentImpl is the concrete implementation of the Agent interface
type agentImpl struct {
	logger    logger.Logger
	a2aClient A2AClientInterface
	provider  providers.IProvider
	model     *string
}

// NewAgent creates a new Agent instance
func NewAgent(logger logger.Logger, a2aClient A2AClientInterface) Agent {
	return &agentImpl{
		a2aClient: a2aClient,
		logger:    logger,
		provider:  nil,
		model:     nil,
	}
}

func (a *agentImpl) SetProvider(provider providers.IProvider) {
	if provider == nil {
		a.logger.Error("attempted to set nil provider", errors.New("provider is nil"))
		return
	}
	a.provider = provider
	a.logger.Debug("provider set for agent", "provider", provider.GetName())
}

func (a *agentImpl) SetModel(model *string) {
	if model == nil {
		a.logger.Error("attempted to set nil model", errors.New("model is nil"))
		return
	}
	a.model = model
	a.logger.Debug("model set for agent", "model", *model)
}

func (a *agentImpl) RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error {
	if a.provider == nil {
		return errors.New("provider is not set for agent")
	}
	if a.model == nil {
		return errors.New("model is not set for agent")
	}

	// TODO: Implement the agent run with stream logic
	return nil
}

func (a *agentImpl) Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error {
	if a.provider == nil {
		return errors.New("provider is not set for agent")
	}
	if a.model == nil {
		return errors.New("model is not set for agent")
	}

	// TODO: Implement the agent run logic
	return nil
}
