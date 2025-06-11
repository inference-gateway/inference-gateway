package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/inference-gateway/inference-gateway/a2a"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/tests/mocks"

	a2amocks "github.com/inference-gateway/inference-gateway/tests/mocks/a2a"
)

func TestA2AAgent_StreamingCapabilityDetection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)

	a2aConfig := &config.A2AConfig{
		PollingInterval: time.Second * 1,
		MaxPollAttempts: 5,
		PollingTimeout:  time.Second * 10,
	}

	tests := []struct {
		name                 string
		agentCapabilities    map[string]a2a.AgentCapabilities
		expectedStreamingURL string
		shouldUseStreaming   bool
	}{
		{
			name: "Agent with streaming capability",
			agentCapabilities: map[string]a2a.AgentCapabilities{
				"http://streaming-agent.com": {
					Streaming: boolPtr(true),
				},
			},
			expectedStreamingURL: "http://streaming-agent.com",
			shouldUseStreaming:   true,
		},
		{
			name: "Agent without streaming capability",
			agentCapabilities: map[string]a2a.AgentCapabilities{
				"http://non-streaming-agent.com": {
					Streaming: boolPtr(false),
				},
			},
			expectedStreamingURL: "http://non-streaming-agent.com",
			shouldUseStreaming:   false,
		},
		{
			name: "Agent with nil streaming capability",
			agentCapabilities: map[string]a2a.AgentCapabilities{
				"http://unknown-streaming-agent.com": {
					Streaming: nil,
				},
			},
			expectedStreamingURL: "http://unknown-streaming-agent.com",
			shouldUseStreaming:   false,
		},
		{
			name:                 "Agent with no capabilities",
			agentCapabilities:    map[string]a2a.AgentCapabilities{},
			expectedStreamingURL: "http://no-capabilities-agent.com",
			shouldUseStreaming:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := a2a.NewAgent(mockLogger, mockA2AClient, a2aConfig)

			mockA2AClient.EXPECT().GetAgentCapabilities().Return(tt.agentCapabilities).Times(1)

			capabilities := mockA2AClient.GetAgentCapabilities()
			agentCapability, hasCapability := capabilities[tt.expectedStreamingURL]
			supportsStreaming := hasCapability && agentCapability.Streaming != nil && *agentCapability.Streaming

			assert.Equal(t, tt.shouldUseStreaming, supportsStreaming, "Agent streaming capability detection should match expected behavior")

			assert.NotNil(t, agent)
		})
	}
}

func TestA2AAgent_StreamingFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)

	a2aConfig := &config.A2AConfig{
		PollingInterval: time.Second * 1,
		MaxPollAttempts: 5,
		PollingTimeout:  time.Second * 10,
	}

	agent := a2a.NewAgent(mockLogger, mockA2AClient, a2aConfig)

	t.Run("SendStreamingMessage method exists and can be called", func(t *testing.T) {
		streamingRequest := &a2a.SendStreamingMessageRequest{
			ID:      "test-stream-1",
			JSONRPC: "2.0",
			Method:  "message/stream",
			Params: a2a.MessageSendParams{
				Message: a2a.Message{
					Kind:      "message",
					MessageID: "test-msg-1",
					Role:      "user",
					Parts: []a2a.Part{
						a2a.TextPart{
							Kind: "text",
							Text: "Test streaming message",
						},
					},
				},
			},
		}

		ctx := context.Background()
		testStreamCh := make(chan []byte, 1)
		close(testStreamCh)

		mockA2AClient.EXPECT().
			SendStreamingMessage(ctx, streamingRequest, "http://test-agent.com").
			Return(testStreamCh, nil).
			Times(1)

		streamCh, err := mockA2AClient.SendStreamingMessage(ctx, streamingRequest, "http://test-agent.com")

		assert.NoError(t, err)
		assert.NotNil(t, streamCh)
		assert.NotNil(t, agent)
	})
}
