package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	a2a "github.com/inference-gateway/inference-gateway/a2a"
)

type WeatherData struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Condition   string  `json:"condition"`
	WindSpeed   float64 `json:"wind_speed"`
	Pressure    float64 `json:"pressure"`
	Timestamp   string  `json:"timestamp"`
}

type ForecastData struct {
	Date        string  `json:"date"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Condition   string  `json:"condition"`
	Humidity    int     `json:"humidity"`
	WindSpeed   float64 `json:"wind_speed"`
	Probability int     `json:"rain_probability"`
}

type WeatherResponse struct {
	Weather WeatherData `json:"weather"`
	Agent   string      `json:"agent"`
}

type ForecastResponse struct {
	Location string         `json:"location"`
	Forecast []ForecastData `json:"forecast"`
	Days     int            `json:"days"`
	Agent    string         `json:"agent"`
}

type WeatherConditions struct {
	AirQuality   string  `json:"air_quality"`
	UVIndex      int     `json:"uv_index"`
	VisibilityKm float64 `json:"visibility_km"`
	Sunrise      string  `json:"sunrise"`
	Sunset       string  `json:"sunset"`
	MoonPhase    string  `json:"moon_phase"`
	FeelsLike    float64 `json:"feels_like"`
	DewPoint     float64 `json:"dew_point"`
}

type ConditionsResponse struct {
	Location   string            `json:"location"`
	Conditions WeatherConditions `json:"conditions"`
	Agent      string            `json:"agent"`
}

type WeatherAlert struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
}

type AlertsResponse struct {
	Location string         `json:"location"`
	Alerts   []WeatherAlert `json:"alerts"`
	Agent    string         `json:"agent"`
}

type StreamingConnectionMessage struct {
	Type     string `json:"type"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Location string `json:"location"`
}

type StreamingWeatherUpdate struct {
	Type    string      `json:"type"`
	Weather WeatherData `json:"weather"`
	Agent   string      `json:"agent"`
}

func main() {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleJSONRPCRequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		streaming := true
		pushNotifications := false
		stateTransitionHistory := false

		info := a2a.AgentCard{
			Name:        "weather-agent",
			Description: "A weather information agent that provides current weather and forecasts",
			URL:         "http://localhost:8083",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              &streaming,
				PushNotifications:      &pushNotifications,
				StateTransitionHistory: &stateTransitionHistory,
			},
			DefaultInputModes:  []string{"text"},
			DefaultOutputModes: []string{"text"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "current",
					Name:        "current",
					Description: "Get current weather for a location",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "forecast",
					Name:        "forecast",
					Description: "Get weather forecast for a location",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "conditions",
					Name:        "conditions",
					Description: "Get detailed weather conditions",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "alerts",
					Name:        "alerts",
					Description: "Get weather alerts for a location",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
			},
		}
		c.JSON(http.StatusOK, info)
	})

	log.Println("weather-agent starting on port 8083...")
	if err := r.Run(":8083"); err != nil {
		log.Fatal("failed to start server:", err)
	}
}

func handleJSONRPCRequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	if req.ID == nil {
		id := interface{}(uuid.New().String())
		req.ID = &id
	}

	switch req.Method {
	case "message/send":
		handleMessageSend(c, req)
	case "message/stream":
		handleJSONRPCStreamRequest(c)
	default:
		sendError(c, req.ID, -32601, "method not found")
	}
}

// handleMessageSend handles A2A message/send requests
func handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest) {
	paramsMap, ok := req.Params["message"].(map[string]interface{})
	if !ok {
		sendError(c, req.ID, -32602, "invalid params: missing message")
		return
	}

	partsArray, ok := paramsMap["parts"].([]interface{})
	if !ok {
		sendError(c, req.ID, -32602, "invalid params: missing message parts")
		return
	}

	var messageText string
	for _, partInterface := range partsArray {
		part, ok := partInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if partKind, exists := part["kind"]; exists && partKind == "text" {
			if text, textExists := part["text"].(string); textExists {
				messageText = text
				break
			}
		}
	}

	metadata, ok := req.Params["metadata"].(map[string]interface{})
	var skill string
	var arguments map[string]interface{}

	if ok {
		if skillVal, exists := metadata["skill"].(string); exists {
			skill = skillVal
		}
		if argsVal, exists := metadata["arguments"].(map[string]interface{}); exists {
			arguments = argsVal
		}
	}

	if skill == "" {
		text := strings.ToLower(messageText)
		if strings.Contains(text, "forecast") {
			skill = "forecast"
		} else if strings.Contains(text, "conditions") {
			skill = "conditions"
		} else if strings.Contains(text, "alerts") {
			skill = "alerts"
		} else {
			skill = "current"
		}
	}

	var location string
	if arguments != nil {
		if loc, ok := arguments["location"].(string); ok {
			location = loc
		} else if req, ok := arguments["request"].(string); ok {
			location = req
		}
	}

	if location == "" {
		location = messageText
	}

	var weatherResult interface{}
	switch skill {
	case "current":
		weather := generateWeatherData(location)
		weatherResult = WeatherResponse{
			Weather: weather,
			Agent:   "weather-agent",
		}
	case "forecast":
		days := 5
		if arguments != nil {
			if d, ok := arguments["days"].(float64); ok {
				days = int(d)
				if days > 7 {
					days = 7
				}
				if days < 1 {
					days = 1
				}
			}
		}
		forecast := generateForecast(location, days)
		weatherResult = ForecastResponse{
			Location: location,
			Forecast: forecast,
			Days:     days,
			Agent:    "weather-agent",
		}
	case "conditions":
		conditions := generateConditions(location)
		weatherResult = ConditionsResponse{
			Location:   location,
			Conditions: conditions,
			Agent:      "weather-agent",
		}
	case "alerts":
		alerts := generateAlerts(location)
		weatherResult = AlertsResponse{
			Location: location,
			Alerts:   alerts,
			Agent:    "weather-agent",
		}
	default:
		weather := generateWeatherData(location)
		weatherResult = WeatherResponse{
			Weather: weather,
			Agent:   "weather-agent",
		}
	}

	resultJSON, _ := json.Marshal(weatherResult)

	taskId := uuid.New().String()
	contextId := uuid.New().String()
	messageId := uuid.New().String()
	timestamp := time.Now().Format(time.RFC3339)
	artifactName := fmt.Sprintf("%s_weather.json", skill)

	responseMessage := a2a.Message{
		Role:      "assistant",
		MessageID: messageId,
		ContextID: &contextId,
		TaskID:    &taskId,
		Kind:      "message",
		Parts: []a2a.Part{
			a2a.TextPart{
				Kind: "text",
				Text: string(resultJSON),
			},
		},
	}

	task := a2a.Task{
		ID:        taskId,
		ContextID: contextId,
		Kind:      "task",
		Status: a2a.TaskStatus{
			State:     "completed",
			Timestamp: &timestamp,
			Message:   &responseMessage,
		},
		Artifacts: []a2a.Artifact{
			{
				ArtifactID: uuid.New().String(),
				Name:       &artifactName,
				Parts: []a2a.Part{
					a2a.TextPart{
						Kind: "text",
						Text: string(resultJSON),
					},
				},
			},
		},
		History: []a2a.Message{
			{
				Role:      "user",
				MessageID: getStringFromMap(paramsMap, "messageId", uuid.New().String()),
				ContextID: &contextId,
				TaskID:    &taskId,
				Kind:      "message",
				Parts: []a2a.Part{
					a2a.TextPart{
						Kind: "text",
						Text: messageText,
					},
				},
			},
			responseMessage,
		},
	}

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result:  task,
	}

	c.JSON(http.StatusOK, response)
}

// Helper function to extract string values from maps
func getStringFromMap(m map[string]interface{}, key string, defaultValue string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func generateWeatherData(location string) WeatherData {
	conditions := []string{"sunny", "partly cloudy", "cloudy", "rainy", "stormy", "snowy", "foggy"}
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

	return WeatherData{
		Location:    location,
		Temperature: float64(int(temp*10)) / 10, // Round to 1 decimal
		Humidity:    30 + rand.Intn(51),         // 30-80%
		Condition:   condition,
		WindSpeed:   float64(rand.Intn(31)),  // 0-30 km/h
		Pressure:    980 + rand.Float64()*50, // 980-1030 hPa
		Timestamp:   time.Now().Format("2006-01-02T15:04:05Z"),
	}
}

func generateForecast(location string, days int) []ForecastData {
	forecast := make([]ForecastData, days)
	conditions := []string{"sunny", "partly cloudy", "cloudy", "rainy", "stormy"}

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, i+1).Format("2006-01-02")
		condition := conditions[rand.Intn(len(conditions))]

		var baseTemp float64
		switch condition {
		case "sunny":
			baseTemp = 25
		case "partly cloudy":
			baseTemp = 22
		case "cloudy":
			baseTemp = 18
		case "rainy":
			baseTemp = 15
		case "stormy":
			baseTemp = 12
		}

		variation := rand.Float64()*10 - 5 // ±5 degrees
		high := baseTemp + 3 + variation
		low := baseTemp - 3 + variation

		forecast[i] = ForecastData{
			Date:        date,
			High:        float64(int(high*10)) / 10,
			Low:         float64(int(low*10)) / 10,
			Condition:   condition,
			Humidity:    40 + rand.Intn(41), // 40-80%
			WindSpeed:   float64(rand.Intn(26)),
			Probability: rand.Intn(101), // 0-100%
		}
	}

	return forecast
}

func generateConditions(location string) WeatherConditions {
	locationLower := strings.ToLower(location)

	var airQuality string
	var uvIndex int
	var visibility float64

	if strings.Contains(locationLower, "city") || strings.Contains(locationLower, "urban") {
		airQuality = "moderate"
		uvIndex = 6
		visibility = 8.0
	} else if strings.Contains(locationLower, "mountain") || strings.Contains(locationLower, "rural") {
		airQuality = "good"
		uvIndex = 8
		visibility = 15.0
	} else {
		airQualities := []string{"good", "moderate", "unhealthy for sensitive groups"}
		airQuality = airQualities[rand.Intn(len(airQualities))]
		uvIndex = 3 + rand.Intn(8) // 3-10
		visibility = 5.0 + rand.Float64()*10.0
	}

	return WeatherConditions{
		AirQuality:   airQuality,
		UVIndex:      uvIndex,
		VisibilityKm: float64(int(visibility*10)) / 10,
		Sunrise:      "06:30",
		Sunset:       "18:45",
		MoonPhase:    getMoonPhase(),
		FeelsLike:    generateFeelsLike(),
		DewPoint:     float64(rand.Intn(21)), // 0-20°C
	}
}

func generateAlerts(location string) []WeatherAlert {
	if rand.Float64() < 0.3 { // 30% chance of having alerts
		return []WeatherAlert{}
	}

	alertTypes := []string{
		"Thunderstorm Warning",
		"Heat Advisory",
		"Flood Watch",
		"High Wind Warning",
		"Winter Storm Warning",
		"Air Quality Alert",
	}

	severities := []string{"Minor", "Moderate", "Severe", "Extreme"}

	alertType := alertTypes[rand.Intn(len(alertTypes))]
	severity := severities[rand.Intn(len(severities))]

	alert := WeatherAlert{
		Type:        alertType,
		Severity:    severity,
		Description: fmt.Sprintf("%s in effect for %s area", alertType, location),
		StartTime:   time.Now().Format("2006-01-02T15:04:05Z"),
		EndTime:     time.Now().Add(6 * time.Hour).Format("2006-01-02T15:04:05Z"),
	}

	return []WeatherAlert{alert}
}

func getMoonPhase() string {
	phases := []string{"New Moon", "Waxing Crescent", "First Quarter", "Waxing Gibbous", "Full Moon", "Waning Gibbous", "Last Quarter", "Waning Crescent"}
	return phases[rand.Intn(len(phases))]
}

func generateFeelsLike() float64 {
	base := 15 + rand.Float64()*15 // 15-30°C
	return float64(int(base*10)) / 10
}

func sendError(c *gin.Context, id interface{}, code int, message string) {
	response := a2a.JSONRPCErrorResponse{
		ID:      id,
		JSONRPC: "2.0",
		Error: a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, response)
}

func handleJSONRPCStreamRequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	if req.ID == nil {
		id := interface{}(uuid.New().String())
		req.ID = &id
	}

	if req.Method != "message/stream" {
		sendError(c, req.ID, -32601, "method not found - use message/stream for streaming")
		return
	}

	messageData, ok := req.Params["message"]
	if !ok {
		sendError(c, req.ID, -32602, "missing 'message' parameter")
		return
	}

	messageBytes, err := json.Marshal(messageData)
	if err != nil {
		sendError(c, req.ID, -32602, "invalid message format")
		return
	}

	var message struct {
		Role  string `json:"role"`
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
		MessageID string `json:"messageId"`
	}

	if err := json.Unmarshal(messageBytes, &message); err != nil {
		sendError(c, req.ID, -32602, "invalid message structure")
		return
	}

	var location string
	var weatherAction string
	for _, part := range message.Parts {
		if part.Type == "text" {
			text := strings.ToLower(part.Text)
			if strings.Contains(text, "current") || strings.Contains(text, "weather") {
				weatherAction = "current"
			} else if strings.Contains(text, "forecast") {
				weatherAction = "forecast"
			} else if strings.Contains(text, "conditions") {
				weatherAction = "conditions"
			} else if strings.Contains(text, "alerts") {
				weatherAction = "alerts"
			}

			words := strings.Fields(part.Text)
			for i, word := range words {
				if (strings.Contains(strings.ToLower(word), "in") ||
					strings.Contains(strings.ToLower(word), "for") ||
					strings.Contains(strings.ToLower(word), "at")) && i+1 < len(words) {
					location = words[i+1]
					break
				}
			}

			if location == "" && len(words) > 0 {
				location = words[len(words)-1]
			}
		}
	}

	if location == "" {
		location = "Berlin"
	}
	if weatherAction == "" {
		weatherAction = "current"
	}

	streamWeatherDataA2A(c, req, location, weatherAction, message.MessageID)
}

func streamWeatherDataA2A(c *gin.Context, req a2a.JSONRPCRequest, location string, action string, messageID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	taskID := uuid.New().String()
	contextID := uuid.New().String()
	artifactID := uuid.New().String()

	taskStatus := a2a.TaskStatus{
		State:     a2a.TaskStateSubmitted,
		Timestamp: stringPtr(time.Now().Format("2006-01-02T15:04:05.000Z07:00")),
	}

	task := a2a.Task{
		ID:        taskID,
		ContextID: contextID,
		Status:    taskStatus,
		History: []a2a.Message{
			{
				Role: "user",
				Parts: []a2a.Part{
					a2a.TextPart{
						Kind: "text",
						Text: fmt.Sprintf("Get %s weather for %s", action, location),
					},
				},
				MessageID: messageID,
				TaskID:    &taskID,
				ContextID: &contextID,
				Kind:      "message",
			},
		},
		Kind:     "task",
		Metadata: map[string]interface{}{},
	}

	submissionResponse := a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  task,
	}

	data, _ := json.Marshal(submissionResponse)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()

	time.Sleep(500 * time.Millisecond)

	var weatherData interface{}
	var artifactName string

	switch action {
	case "current":
		weatherData = generateWeatherData(location)
		artifactName = "current_weather.json"
	case "forecast":
		weatherData = generateForecast(location, 5)
		artifactName = "weather_forecast.json"
	case "conditions":
		weatherData = generateConditions(location)
		artifactName = "weather_conditions.json"
	case "alerts":
		weatherData = generateAlerts(location)
		artifactName = "weather_alerts.json"
	default:
		weatherData = generateWeatherData(location)
		artifactName = "weather_data.json"
	}

	weatherJSON, _ := json.Marshal(weatherData)
	artifactUpdateEvent := a2a.TaskArtifactUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Artifact: a2a.Artifact{
			ArtifactID: artifactID,
			Name:       &artifactName,
			Parts: []a2a.Part{
				a2a.TextPart{
					Kind: "text",
					Text: string(weatherJSON),
				},
			},
		},
		Append:    boolPtr(false),
		LastChunk: boolPtr(true),
		Kind:      "artifact-update",
	}

	artifactResponse := a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  artifactUpdateEvent,
	}

	data, _ = json.Marshal(artifactResponse)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()

	time.Sleep(200 * time.Millisecond)

	completionStatus := a2a.TaskStatus{
		State:     a2a.TaskStateCompleted,
		Timestamp: stringPtr(time.Now().Format("2006-01-02T15:04:05.000Z07:00")),
	}

	statusUpdateEvent := a2a.TaskStatusUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Status:    completionStatus,
		Final:     true,
		Kind:      "status-update",
	}

	completionResponse := a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  statusUpdateEvent,
	}

	data, _ = json.Marshal(completionResponse)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
