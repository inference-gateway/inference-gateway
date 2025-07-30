package tools

import (
	"context"
	"fmt"

	"github.com/inference-gateway/adk/server"
)

// NewGetWeatherTool creates a new weather tool that gets current weather information for a location
func NewGetWeatherTool() server.Tool {
	return server.NewBasicTool(
		"get_weather",
		"Get current weather information for a location",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"units": map[string]interface{}{
					"type":        "string",
					"description": "Temperature units: celsius, fahrenheit, or kelvin",
					"enum":        []string{"celsius", "fahrenheit", "kelvin"},
				},
			},
			"required": []string{"location"},
		},
		getWeatherHandler,
	)
}

// getWeatherHandler handles the weather information retrieval
func getWeatherHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	location := args["location"].(string)
	units := "celsius"
	if u, ok := args["units"].(string); ok {
		units = u
	}

	// Mock weather data based on location
	var temp string
	var description string
	switch location {
	case "San Francisco, CA":
		switch units {
		case "fahrenheit":
			temp = "65°F"
		case "kelvin":
			temp = "291K"
		default:
			temp = "18°C"
		}
		description = "Partly cloudy with light fog"
	case "New York, NY":
		switch units {
		case "fahrenheit":
			temp = "72°F"
		case "kelvin":
			temp = "295K"
		default:
			temp = "22°C"
		}
		description = "Sunny with scattered clouds"
	default:
		switch units {
		case "fahrenheit":
			temp = "70°F"
		case "kelvin":
			temp = "294K"
		default:
			temp = "21°C"
		}
		description = "Moderate weather conditions"
	}

	result := fmt.Sprintf(`{
		"location": "%s",
		"temperature": "%s",
		"description": "%s",
		"units": "%s",
		"humidity": "60%%",
		"wind_speed": "15 km/h"
	}`, location, temp, description, units)

	return result, nil
}
