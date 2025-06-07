package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// CreateGoogleCredentialsFile creates a Google credentials JSON file from environment variable content
func CreateGoogleCredentialsFile(l *zap.Logger) error {
	// Get the JSON content from environment variable
	jsonContent := os.Getenv("GOOGLE_CALENDAR_SA_JSON")
	if jsonContent == "" {
		l.Debug("google_calendar_sa_json environment variable not set, skipping credentials file creation")
		return nil
	}

	// Validate that the content is valid JSON
	var temp interface{}
	if err := json.Unmarshal([]byte(jsonContent), &temp); err != nil {
		return fmt.Errorf("invalid json content in google_calendar_sa_json: %w", err)
	}

	// Define the target file path
	credentialsPath := "/app/secrets/google-credentials.json"

	// Create the directory if it doesn't exist
	dir := filepath.Dir(credentialsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the JSON content to the file
	if err := os.WriteFile(credentialsPath, []byte(jsonContent), 0600); err != nil {
		return fmt.Errorf("failed to write google credentials file %s: %w", credentialsPath, err)
	}

	return nil
}
