package main

import (
	"encoding/json"
	"math/rand"
	"time"

	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// WeatherData represents weather information for a location
type WeatherData struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Condition   string  `json:"condition"`
	WindSpeed   float64 `json:"wind_speed"`
	Pressure    float64 `json:"pressure"`
	Timestamp   string  `json:"timestamp"`
}

// FetchWeatherParams represents parameters for fetching weather
type FetchWeatherParams struct {
	Location string `json:"location"`
}

// WeatherService provides weather-related operations
type WeatherService interface {
	FetchWeather(location string) *WeatherData
	GetAvailableConditions() []string
}

// MockWeatherService implements WeatherService with mock data
type MockWeatherService struct {
	logger *zap.Logger
}

// NewMockWeatherService creates a new mock weather service
func NewMockWeatherService(logger *zap.Logger) WeatherService {
	return &MockWeatherService{
		logger: logger,
	}
}

// FetchWeather generates mock weather data for a given location
func (w *MockWeatherService) FetchWeather(location string) *WeatherData {
	w.logger.Debug("generating weather data for location", zap.String("location", location))

	conditions := w.GetAvailableConditions()
	condition := conditions[rand.Intn(len(conditions))]

	var temp float64
	switch condition {
	case "sunny":
		temp = 20 + rand.Float64()*15 // 20-35°C
	case "partly cloudy", "cloudy":
		temp = 15 + rand.Float64()*10 // 15-25°C
	case "rainy", "stormy":
		temp = 10 + rand.Float64()*10 // 10-20°C
	case "snowy":
		temp = -5 + rand.Float64()*10 // -5-5°C
	case "foggy":
		temp = 5 + rand.Float64()*15 // 5-20°C
	default:
		temp = 15 + rand.Float64()*10
	}

	weather := &WeatherData{
		Location:    location,
		Temperature: float64(int(temp*10)) / 10, // Round to 1 decimal
		Humidity:    30 + rand.Intn(51),         // 30-80%
		Condition:   condition,
		WindSpeed:   float64(rand.Intn(31)),  // 0-30 km/h
		Pressure:    980 + rand.Float64()*50, // 980-1030 hPa
		Timestamp:   time.Now().Format("2006-01-02T15:04:05Z"),
	}

	w.logger.Debug("generated weather data",
		zap.String("location", weather.Location),
		zap.Float64("temperature", weather.Temperature),
		zap.String("condition", weather.Condition),
		zap.Int("humidity", weather.Humidity),
		zap.Float64("wind_speed", weather.WindSpeed),
		zap.Float64("pressure", weather.Pressure))

	return weather
}

// GetAvailableConditions returns list of possible weather conditions
func (w *MockWeatherService) GetAvailableConditions() []string {
	return []string{"sunny", "partly cloudy", "cloudy", "rainy", "stormy", "snowy", "foggy"}
}

// WeatherToolHandler handles weather-related tool calls
type WeatherToolHandler struct {
	weatherService WeatherService
	logger         *zap.Logger
}

// NewWeatherToolHandler creates a new weather tool handler
func NewWeatherToolHandler(weatherService WeatherService, logger *zap.Logger) *WeatherToolHandler {
	return &WeatherToolHandler{
		weatherService: weatherService,
		logger:         logger,
	}
}

// HandleToolCall processes a general tool call (ADK interface)
func (h *WeatherToolHandler) HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "fetch_weather":
		return h.HandleFetchWeather(toolCall.Function.Arguments)
	default:
		h.logger.Warn("unknown tool call", zap.String("function", toolCall.Function.Name))
		return "", NewWeatherError("unknown function: " + toolCall.Function.Name)
	}
}

// HandleFetchWeather processes a fetch_weather tool call
func (h *WeatherToolHandler) HandleFetchWeather(arguments string) (string, error) {
	var params FetchWeatherParams
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		h.logger.Error("failed to unmarshal fetch_weather parameters", zap.Error(err))
		return "", err
	}

	weatherData := h.weatherService.FetchWeather(params.Location)
	weatherJSON, err := json.Marshal(weatherData)
	if err != nil {
		h.logger.Error("failed to marshal weather data", zap.Error(err))
		return "", err
	}

	return string(weatherJSON), nil
}
