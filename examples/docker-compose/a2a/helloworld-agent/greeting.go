package main

import (
	"encoding/json"

	uuid "github.com/google/uuid"
	adk "github.com/inference-gateway/a2a/adk"
	zap "go.uber.org/zap"
)

// GreetingData represents greeting information for a person
type GreetingData struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	Greeting  string `json:"greeting"`
	Timestamp string `json:"timestamp"`
}

// GreetParams represents parameters for greeting
type GreetParams struct {
	Language string `json:"language"`
	Name     string `json:"name"`
}

// GreetingService provides greeting-related operations
type GreetingService interface {
	Greet(language, name string) *GreetingData
	GetAvailableLanguages() []string
}

// MockGreetingService implements GreetingService with mock data
type MockGreetingService struct {
	logger *zap.Logger
}

// NewMockGreetingService creates a new mock greeting service
func NewMockGreetingService(logger *zap.Logger) GreetingService {
	return &MockGreetingService{
		logger: logger,
	}
}

// Greet generates greeting data for a given language and name
func (g *MockGreetingService) Greet(language, name string) *GreetingData {
	g.logger.Debug("generating greeting for person", zap.String("name", name), zap.String("language", language))

	var greeting string
	switch language {
	case "en":
		greeting = "Hello, " + name + "!"
	case "es":
		greeting = "¡Hola, " + name + "!"
	case "fr":
		greeting = "Bonjour, " + name + "!"
	case "de":
		greeting = "Hallo, " + name + "!"
	case "zh":
		greeting = "你好, " + name + "!"
	case "ja":
		greeting = "こんにちは, " + name + "!"
	case "ko":
		greeting = "안녕하세요, " + name + "!"
	case "it":
		greeting = "Ciao, " + name + "!"
	case "pt":
		greeting = "Olá, " + name + "!"
	case "ru":
		greeting = "Привет, " + name + "!"
	default:
		greeting = "Hello, " + name + "!"
	}

	return &GreetingData{
		Name:      name,
		Language:  language,
		Greeting:  greeting,
		Timestamp: "2025-06-11T12:00:00Z",
	}
}

// GetAvailableLanguages returns list of supported languages
func (g *MockGreetingService) GetAvailableLanguages() []string {
	return []string{"en", "es", "fr", "de", "zh", "ja", "ko", "it", "pt", "ru"}
}

// GreetingToolHandler handles greeting-related tool calls
type GreetingToolHandler struct {
	greetingService GreetingService
	logger          *zap.Logger
}

// NewGreetingToolHandler creates a new greeting tool handler
func NewGreetingToolHandler(greetingService GreetingService, logger *zap.Logger) *GreetingToolHandler {
	return &GreetingToolHandler{
		greetingService: greetingService,
		logger:          logger,
	}
}

// HandleGreetTool processes greet function calls
func (h *GreetingToolHandler) HandleGreetTool(args string) (string, error) {
	var greetParams GreetParams
	if err := json.Unmarshal([]byte(args), &greetParams); err != nil {
		h.logger.Error("failed to unmarshal greet parameters", zap.Error(err))
		return "", err
	}

	h.logger.Info("processing greet request",
		zap.String("name", greetParams.Name),
		zap.String("language", greetParams.Language))

	greetingData := h.greetingService.Greet(greetParams.Language, greetParams.Name)

	response, err := json.Marshal(greetingData)
	if err != nil {
		h.logger.Error("failed to marshal greeting response", zap.Error(err))
		return "", err
	}

	h.logger.Debug("greeting generated successfully", zap.String("greeting", greetingData.Greeting))
	return string(response), nil
}

// GreetingTaskResultProcessor implements adk.TaskResultProcessor for greeting-specific business logic
type GreetingTaskResultProcessor struct {
	logger *zap.Logger
}

// NewGreetingTaskResultProcessor creates a new greeting task result processor
func NewGreetingTaskResultProcessor(logger *zap.Logger) adk.TaskResultProcessor {
	return &GreetingTaskResultProcessor{
		logger: logger,
	}
}

// ProcessToolResult processes greeting tool call results and determines if the task should be completed
func (p *GreetingTaskResultProcessor) ProcessToolResult(toolCallResult string) *adk.Message {
	var greetingData GreetingData
	if err := json.Unmarshal([]byte(toolCallResult), &greetingData); err != nil {
		p.logger.Debug("tool result is not greeting data, continuing task processing", zap.Error(err))
		return nil
	}

	p.logger.Info("greeting task completed", zap.String("greeting", greetingData.Greeting))

	// Create completion message with the greeting text
	return &adk.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": greetingData.Greeting,
			},
		},
	}
}

// GreetingAgentInfoProvider implements adk.AgentInfoProvider for greeting-specific agent metadata
type GreetingAgentInfoProvider struct {
	logger *zap.Logger
}

// NewGreetingAgentInfoProvider creates a new greeting agent info provider
func NewGreetingAgentInfoProvider(logger *zap.Logger) adk.AgentInfoProvider {
	return &GreetingAgentInfoProvider{
		logger: logger,
	}
}

// GetAgentCard returns greeting-specific agent capabilities and metadata
func (p *GreetingAgentInfoProvider) GetAgentCard(baseConfig adk.Config) adk.AgentCard {
	return adk.AgentCard{
		Name:        baseConfig.AgentName,
		Description: baseConfig.AgentDescription,
		URL:         baseConfig.AgentURL,
		Version:     baseConfig.AgentVersion,
		Capabilities: adk.AgentCapabilities{
			Streaming:              &baseConfig.CapabilitiesConfig.Streaming,
			PushNotifications:      &baseConfig.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &baseConfig.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []adk.AgentSkill{
			{
				ID:          "greeting",
				Name:        "Personalized Greetings",
				Description: "Provide personalized greetings in multiple languages",
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain"},
			},
		},
	}
}
