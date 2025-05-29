package dbutils

import (
	"fmt"
	"log"

	"github.com/nickhuang/allama/internal/config"
	"github.com/nickhuang/allama/internal/models"
	"github.com/nickhuang/allama/internal/storage"
)

func RunDBUtils() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database storage
	store, err := storage.NewStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Example: Adding a provider
	openAIProvider := &models.Provider{
		Name:     "openai",
		APIKey:   "your-openai-api-key-here",
		Endpoint: "https://api.openai.com",
		IsActive: true,
	}
	err = store.AddProvider(openAIProvider)
	if err != nil {
		log.Printf("Failed to add OpenAI provider: %v", err)
	} else {
		fmt.Printf("Added OpenAI provider with ID: %d\n", openAIProvider.ID)
	}

	// Example: Adding a model for the provider
	gptModel := &models.Model{
		ProviderID: openAIProvider.ID,
		Name:       "GPT-4",
		ModelID:    "gpt-4",
		IsActive:   true,
	}
	err = store.AddModel(gptModel)
	if err != nil {
		log.Printf("Failed to add GPT-4 model: %v", err)
	} else {
		fmt.Printf("Added GPT-4 model with ID: %d\n", gptModel.ID)
	}

	// Example: Fetching a provider by name
	fetchedProvider, err := store.GetProviderByName("openai")
	if err != nil {
		log.Printf("Failed to fetch OpenAI provider: %v", err)
	} else if fetchedProvider != nil {
		fmt.Printf("Fetched OpenAI provider: ID=%d, Name=%s, IsActive=%v\n", fetchedProvider.ID, fetchedProvider.Name, fetchedProvider.IsActive)
	} else {
		fmt.Println("OpenAI provider not found")
	}

	// Example: Fetching models by provider ID
	models, err := store.GetModelsByProviderID(openAIProvider.ID)
	if err != nil {
		log.Printf("Failed to fetch models for OpenAI: %v", err)
	} else {
		fmt.Println("Models for OpenAI provider:")
		for _, model := range models {
			fmt.Printf("- ID=%d, Name=%s, ModelID=%s, IsActive=%v\n", model.ID, model.Name, model.ModelID, model.IsActive)
		}
	}

	// Example: Fetching all active providers
	activeProviders, err := store.GetActiveProviders()
	if err != nil {
		log.Printf("Failed to fetch active providers: %v", err)
	} else {
		fmt.Println("Active providers:")
		for _, provider := range activeProviders {
			fmt.Printf("- ID=%d, Name=%s, IsActive=%v\n", provider.ID, provider.Name, provider.IsActive)
		}
	}

	// Example: Fetching all active models
	activeModels, err := store.GetActiveModels()
	if err != nil {
		log.Printf("Failed to fetch active models: %v", err)
	} else {
		fmt.Println("Active models:")
		for _, model := range activeModels {
			fmt.Printf("- ID=%d, ProviderID=%d, Name=%s, ModelID=%s, IsActive=%v\n", model.ID, model.ProviderID, model.Name, model.ModelID, model.IsActive)
		}
	}
}
