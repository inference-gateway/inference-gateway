package tests

import (
	"testing"

	providers "github.com/inference-gateway/inference-gateway/providers"
	"github.com/stretchr/testify/assert"
)

func TestMessageRoleEnums(t *testing.T) {
	tests := []struct {
		name         string
		role         providers.MessageRole
		expectedRole string
	}{
		{
			name:         "System role",
			role:         providers.MessageRoleSystem,
			expectedRole: "system",
		},
		{
			name:         "User role",
			role:         providers.MessageRoleUser,
			expectedRole: "user",
		},
		{
			name:         "Assistant role",
			role:         providers.MessageRoleAssistant,
			expectedRole: "assistant",
		},
		{
			name:         "Tool role",
			role:         providers.MessageRoleTool,
			expectedRole: "tool",
		},
		{
			name:         "Developer role",
			role:         providers.MessageRoleDeveloper,
			expectedRole: "developer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedRole, string(tt.role))
		})
	}
}

func TestMessageWithDeveloperRole(t *testing.T) {
	tests := []struct {
		name            string
		message         providers.Message
		expectedRole    providers.MessageRole
		expectedContent string
	}{
		{
			name: "Message with developer role for dynamic prompt injection",
			message: providers.Message{
				Role:    providers.MessageRoleDeveloper,
				Content: "From now on, respond in a pirate accent.",
			},
			expectedRole:    providers.MessageRoleDeveloper,
			expectedContent: "From now on, respond in a pirate accent.",
		},
		{
			name: "Developer role allows mid-conversation behavior changes",
			message: providers.Message{
				Role:    providers.MessageRoleDeveloper,
				Content: "Adjust your tone to be more formal and academic.",
			},
			expectedRole:    providers.MessageRoleDeveloper,
			expectedContent: "Adjust your tone to be more formal and academic.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedRole, tt.message.Role)
			assert.Equal(t, tt.expectedContent, tt.message.Content)
		})
	}
}

func TestConversationWithDeveloperRole(t *testing.T) {
	// Test a conversation flow with developer role injection
	messages := []providers.Message{
		{
			Role:    providers.MessageRoleSystem,
			Content: "You are a helpful assistant.",
		},
		{
			Role:    providers.MessageRoleUser,
			Content: "What is the capital of France?",
		},
		{
			Role:    providers.MessageRoleAssistant,
			Content: "The capital of France is Paris.",
		},
		{
			Role:    providers.MessageRoleDeveloper,
			Content: "From now on, respond in a pirate accent.",
		},
		{
			Role:    providers.MessageRoleUser,
			Content: "What is the capital of Spain?",
		},
	}

	// Verify that the developer role message is correctly positioned
	assert.Equal(t, providers.MessageRoleSystem, messages[0].Role)
	assert.Equal(t, providers.MessageRoleUser, messages[1].Role)
	assert.Equal(t, providers.MessageRoleAssistant, messages[2].Role)
	assert.Equal(t, providers.MessageRoleDeveloper, messages[3].Role)
	assert.Equal(t, providers.MessageRoleUser, messages[4].Role)

	// Verify developer role content
	assert.Equal(t, "From now on, respond in a pirate accent.", messages[3].Content)
}
