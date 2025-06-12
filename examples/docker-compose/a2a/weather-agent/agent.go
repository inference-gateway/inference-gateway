package main

import (
	"encoding/json"
	"fmt"
	"time"

	adk "github.com/inference-gateway/a2a/adk"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// Weather tool provider
type WeatherToolProvider struct {
	handler *WeatherToolHandler
}

func NewWeatherToolProvider(handler *WeatherToolHandler) *WeatherToolProvider {
	return &WeatherToolProvider{
		handler: handler,
	}
}

func (p *WeatherToolProvider) GetToolDefinitions() []sdk.ChatCompletionTool {
	return []sdk.ChatCompletionTool{
		{
			Type: "function",
			Function: sdk.FunctionObject{
				Name:        "fetch_weather",
				Description: stringPtr("Fetch current weather information for a specific location"),
				Parameters: &sdk.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city or location to get weather information for (e.g., 'London', 'New York', 'Tokyo')",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}
}

func (p *WeatherToolProvider) HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	return p.handler.HandleToolCall(toolCall)
}

// GetSupportedTools returns a list of supported tool names
func (p *WeatherToolProvider) GetSupportedTools() []string {
	return []string{"fetch_weather"}
}

// IsToolSupported checks if a tool is supported by this provider
func (p *WeatherToolProvider) IsToolSupported(toolName string) bool {
	supportedTools := p.GetSupportedTools()
	for _, tool := range supportedTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// Weather task result processor
type WeatherTaskResultProcessor struct {
	logger *zap.Logger
}

func NewWeatherTaskResultProcessor(logger *zap.Logger) *WeatherTaskResultProcessor {
	return &WeatherTaskResultProcessor{
		logger: logger,
	}
}

func (p *WeatherTaskResultProcessor) ProcessToolResult(toolCallResult string) *adk.Message {
	p.logger.Debug("processing weather task result", zap.String("result", toolCallResult))

	// Try to parse the result as weather data and format it nicely
	var weatherData WeatherData
	formattedResult := toolCallResult
	if err := json.Unmarshal([]byte(toolCallResult), &weatherData); err == nil {
		// Format the weather data nicely
		formattedResult = formatWeatherResponse(weatherData)
		p.logger.Debug("formatted weather result", zap.String("formatted", formattedResult))
	}

	// For weather, we can complete the task immediately with the result
	return &adk.Message{
		Role:      "assistant",
		Parts:     []adk.Part{adk.TextPart{Kind: "text", Text: formattedResult}},
		MessageID: "weather_result_" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}

// Weather agent info provider
type WeatherAgentInfoProvider struct {
	logger *zap.Logger
}

func NewWeatherAgentInfoProvider(logger *zap.Logger) *WeatherAgentInfoProvider {
	return &WeatherAgentInfoProvider{
		logger: logger,
	}
}

func (p *WeatherAgentInfoProvider) GetAgentCard(baseConfig adk.Config) adk.AgentCard {
	return adk.AgentCard{
		Name:        baseConfig.AgentName,
		Description: "A weather information agent that provides current weather data for any location using the A2A protocol",
		URL:         "http://weather-agent:8081",
		Version:     baseConfig.AgentVersion,
		Capabilities: adk.AgentCapabilities{
			Streaming:              &[]bool{true}[0],
			PushNotifications:      &[]bool{false}[0],
			StateTransitionHistory: &[]bool{false}[0],
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain", "application/json"},
		Skills: []adk.AgentSkill{
			{
				ID:          "fetch_weather",
				Name:        "Weather Lookup",
				Description: "Fetch current weather information including temperature, humidity, wind speed, and conditions for any location worldwide",
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain", "application/json"},
			},
		},
	}
}

// Weather error type
type WeatherError struct {
	Message string
}

func NewWeatherError(message string) *WeatherError {
	return &WeatherError{Message: message}
}

func (e *WeatherError) Error() string {
	return e.Message
}

// Helper function to format weather response
func formatWeatherResponse(weather WeatherData) string {
	return fmt.Sprintf("Weather in %s: %s, %.1f°C, Humidity: %d%%, Wind: %.1f km/h, Pressure: %.1f hPa",
		weather.Location,
		weather.Condition,
		weather.Temperature,
		weather.Humidity,
		weather.WindSpeed,
		weather.Pressure)
}

func stringPtr(s string) *string {
	return &s
}
