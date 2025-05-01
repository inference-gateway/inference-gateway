package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inference-gateway/inference-gateway/api/middlewares"
	mockBase "github.com/inference-gateway/inference-gateway/tests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestMCPClientDiscoverCapabilities tests the DiscoverCapabilities method
func TestMCPClientDiscoverCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/capabilities", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "getWeather",
					"description": "Get the weather for a location",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
							"required": []interface{}{"location"},
						},
					},
				},
				map[string]interface{}{
					"name":        "searchWeb",
					"description": "Search the web for information",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type":        "string",
								"description": "The search query",
							},
							"required": []interface{}{"query"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := mockBase.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := middlewares.NewMCPClient([]string{server.URL}, "", false, mockLogger)

	capabilities, err := client.DiscoverCapabilities(context.Background())
	assert.NoError(t, err)

	assert.Len(t, capabilities, 1)

	assert.Equal(t, server.URL, capabilities[0]["_server_url"])

	tools := capabilities[0]["tools"].([]interface{})
	assert.Len(t, tools, 2)

	tool1 := tools[0].(map[string]interface{})
	assert.Equal(t, "getWeather", tool1["name"])
	assert.Equal(t, "Get the weather for a location", tool1["description"])

	tool2 := tools[1].(map[string]interface{})
	assert.Equal(t, "searchWeb", tool2["name"])
	assert.Equal(t, "Search the web for information", tool2["description"])
}

// TestMCPClientExecuteTool tests the ExecuteTool method
func TestMCPClientExecuteTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tools", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var requestBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&requestBody)

		assert.Equal(t, "getWeather", requestBody["name"])
		params := requestBody["params"].(map[string]interface{})
		assert.Equal(t, "San Francisco, CA", params["location"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"temperature": 72,
			"conditions":  "Sunny",
			"humidity":    45,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := mockBase.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := middlewares.NewMCPClient([]string{server.URL}, "", false, mockLogger)

	params := map[string]interface{}{"location": "San Francisco, CA"}
	serverURL := server.URL
	result, err := client.ExecuteTool(context.Background(), "getWeather", params, serverURL)

	assert.NoError(t, err)
	assert.Equal(t, float64(72), result["temperature"])
	assert.Equal(t, "Sunny", result["conditions"])
	assert.Equal(t, float64(45), result["humidity"])
}

// TestMCPClientExecuteToolError tests the error handling of ExecuteTool method
func TestMCPClientExecuteToolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tools", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := map[string]interface{}{
			"error": "Internal server error",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := mockBase.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	client := middlewares.NewMCPClient([]string{server.URL}, "", false, mockLogger)

	params := map[string]interface{}{"location": "San Francisco, CA"}
	serverURL := server.URL
	_, err := client.ExecuteTool(context.Background(), "getWeather", params, serverURL)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute tool, status code: 500")
}
