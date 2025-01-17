package main

import (
	"fmt"
	"log"

	sdk "github.com/edenreich/inference-gateway/sdk"
)

func main() {
	// Create a new client
	apiClient := sdk.NewClient("http://localhost:8080")

	// List all models
	models, err := apiClient.ListModels()
	if err != nil {
		log.Fatalf("Error listing models: %v", err)
	}

	fmt.Println("Available models:")
	for _, model := range models {
		fmt.Printf("Provider: %s, Models: %v\n", model.Provider, model.Models)
	}

	// Generate content using a specific provider's model
	provider := "openai"
	model := "gpt-4o-mini"
	prompt := "Explain the importance of fast language models. Keep it short and concise."

	response, err := apiClient.GenerateContent(provider, model, prompt)
	if err != nil {
		log.Fatalf("Error generating content: %v", err)
	}

	fmt.Printf("Generated content: %s\n", response.Content)
}
