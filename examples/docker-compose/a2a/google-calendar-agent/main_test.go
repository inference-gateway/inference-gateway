package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"google-calendar-agent/a2a"
)

func init() {
	gin.SetMode(gin.TestMode)
	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		panic("failed to initialize test logger: " + err.Error())
	}
}

func TestHandleA2ARequest(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name: "valid message/send request",
			requestBody: a2a.JSONRPCRequest{
				Jsonrpc: "2.0",
				Method:  "message/send",
				Params: map[string]interface{}{
					"message": map[string]interface{}{
						"parts": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "show me my events today",
							},
						},
					},
				},
				ID: "test-1",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "unknown method",
			requestBody: a2a.JSONRPCRequest{
				Jsonrpc: "2.0",
				Method:  "unknown/method",
				Params:  map[string]interface{}{},
				ID:      "test-2",
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock calendar service
			calendarService = &mockCalendarService{}

			router := gin.New()
			router.POST("/a2a", handleA2ARequest)

			requestBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/a2a", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}
		})
	}
}

func TestProcessCalendarRequest(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		contains    []string
	}{
		{
			name:        "list events today",
			input:       "show me my events today",
			expectError: false,
			contains:    []string{"events for today"},
		},
		{
			name:        "create meeting",
			input:       "schedule a meeting with John at 2pm tomorrow",
			expectError: false,
			contains:    []string{"Event created successfully"},
		},
		{
			name:        "help request",
			input:       "what can you do?",
			expectError: false,
			contains:    []string{"calendar management"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock calendar service
			calendarService = &mockCalendarService{}

			response, err := processCalendarRequest(tt.input)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if response != nil {
				for _, contains := range tt.contains {
					if !containsIgnoreCase(response.Text, contains) {
						t.Errorf("response text should contain '%s', got: %s", contains, response.Text)
					}
				}
			}
		})
	}
}

// Helper function for case-insensitive string contains check
func containsIgnoreCase(text, substr string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substr))
}
