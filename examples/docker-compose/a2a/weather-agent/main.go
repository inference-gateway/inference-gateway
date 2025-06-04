package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	a2a "weather-agent/a2a"
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

func main() {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleJSONRPCRequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		info := a2a.AgentCard{
			Name:        "weather-agent",
			Description: "A weather information agent that provides current weather and forecasts",
			URL:         "http://localhost:8083",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              false,
				Pushnotifications:      false,
				Statetransitionhistory: false,
			},
			Defaultinputmodes:  []string{"text"},
			Defaultoutputmodes: []string{"text"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "current",
					Name:        "current",
					Description: "Get current weather for a location",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "forecast",
					Name:        "forecast",
					Description: "Get weather forecast for a location",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "conditions",
					Name:        "conditions",
					Description: "Get detailed weather conditions",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "alerts",
					Name:        "alerts",
					Description: "Get weather alerts for a location",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
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

	if req.Jsonrpc == "" {
		req.Jsonrpc = "2.0"
	}

	if req.ID == nil {
		req.ID = uuid.New().String()
	}

	switch req.Method {
	case "current":
		handleCurrent(c, req)
	case "forecast":
		handleForecast(c, req)
	case "conditions":
		handleConditions(c, req)
	case "alerts":
		handleAlerts(c, req)
	default:
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleCurrent(c *gin.Context, req a2a.JSONRPCRequest) {
	location, ok := req.Params["location"].(string)
	if !ok {
		sendError(c, req.ID, -32602, "parameter 'location' is required")
		return
	}

	weather := generateWeatherData(location)

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"weather": weather,
			"agent":   "weather-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleForecast(c *gin.Context, req a2a.JSONRPCRequest) {
	location, ok := req.Params["location"].(string)
	if !ok {
		sendError(c, req.ID, -32602, "parameter 'location' is required")
		return
	}

	days := 5
	if d, ok := req.Params["days"]; ok {
		if daysFloat, ok := d.(float64); ok {
			days = int(daysFloat)
		}
		if days > 7 {
			days = 7
		}
		if days < 1 {
			days = 1
		}
	}

	forecast := generateForecast(location, days)

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"location": location,
			"forecast": forecast,
			"days":     days,
			"agent":    "weather-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleConditions(c *gin.Context, req a2a.JSONRPCRequest) {
	location, ok := req.Params["location"].(string)
	if !ok {
		sendError(c, req.ID, -32602, "parameter 'location' is required")
		return
	}

	conditions := generateConditions(location)

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"location":   location,
			"conditions": conditions,
			"agent":      "weather-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleAlerts(c *gin.Context, req a2a.JSONRPCRequest) {
	location, ok := req.Params["location"].(string)
	if !ok {
		sendError(c, req.ID, -32602, "parameter 'location' is required")
		return
	}

	alerts := generateAlerts(location)

	response := a2a.JSONRPCSuccessResponse{
		ID: req.ID,
		Result: map[string]interface{}{
			"location": location,
			"alerts":   alerts,
			"agent":    "weather-agent",
		},
	}

	c.JSON(http.StatusOK, response)
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

func generateConditions(location string) map[string]interface{} {
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

	return map[string]interface{}{
		"air_quality":   airQuality,
		"uv_index":      uvIndex,
		"visibility_km": float64(int(visibility*10)) / 10,
		"sunrise":       "06:30",
		"sunset":        "18:45",
		"moon_phase":    getMoonPhase(),
		"feels_like":    generateFeelsLike(),
		"dew_point":     float64(rand.Intn(21)), // 0-20°C
	}
}

func generateAlerts(location string) []map[string]interface{} {
	if rand.Float64() < 0.3 { // 30% chance of having alerts
		return []map[string]interface{}{}
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

	alert := map[string]interface{}{
		"type":        alertType,
		"severity":    severity,
		"description": fmt.Sprintf("%s in effect for %s area", alertType, location),
		"start_time":  time.Now().Format("2006-01-02T15:04:05Z"),
		"end_time":    time.Now().Add(6 * time.Hour).Format("2006-01-02T15:04:05Z"),
	}

	return []map[string]interface{}{alert}
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
		Jsonrpc: "2.0",
		Error: a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, response)
}
