// Package provider defines interfaces and implementations for various AI model providers.
// It abstracts the communication with different AI services, allowing the application
// to interact with them in a standardized way.
package provider

import (
	"log"

	"github.com/offbeat-studio/allama/internal/models"
	"github.com/offbeat-studio/allama/internal/storage"
)

// ProviderInterface defines the common methods that all AI provider implementations must satisfy.
// This interface allows the application to interact with different AI services in a uniform way.
type ProviderInterface interface {
	// GetModels retrieves a list of available models from the provider.
	// It returns a slice of models.Model and an error if any occurred.
	GetModels() ([]models.Model, error)
	// Chat sends a chat request to the provider with a given model and message history.
	// modelID specifies the model to use for the chat.
	// messages is a slice of maps, where each map represents a message with "role" and "content" keys.
	// It returns the assistant's response content as a string and an error if any occurred.
	Chat(modelID string, messages []map[string]string) (string, error)
}

// CreateProvider instantiates a concrete provider implementation based on the provider's configuration.
// prov contains the configuration details for the provider, such as its name, API key, and host.
// It returns a ProviderInterface, which is the concrete implementation for the specified provider name (e.g., OpenAI, Anthropic).
// If the provider name is unknown or not supported, it returns nil.
func CreateProvider(prov *models.Provider) ProviderInterface {
	switch prov.Name {
	case "openai":
		return NewOpenAIProvider(prov.APIKey, prov.Host)
	case "anthropic":
		return NewAnthropicProvider(prov.APIKey, prov.Host)
	case "ollama":
		return NewOllamaProvider(prov.Host)
	default:
		log.Printf("Unknown provider: %s, cannot create instance", prov.Name)
		return nil
	}
}

// FetchModelsForProvider retrieves models from a given provider's API and stores them in the application's database.
// store is the application's storage instance used to interact with the database.
// prov is the provider configuration for which to fetch and store models.
// This function logs the progress and any errors encountered during the fetching or storing process.
func FetchModelsForProvider(store *storage.Storage, prov *models.Provider) {
	log.Printf("Fetching models for provider: %s", prov.Name)

	providerImpl := CreateProvider(prov)
	if providerImpl == nil {
		log.Printf("Failed to create provider instance for: %s", prov.Name)
		return
	}

	modelsToAdd, err := providerImpl.GetModels()
	if err != nil {
		log.Printf("Failed to fetch models for %s: %v", prov.Name, err)
		return
	}

	// Add fetched models to the database
	for _, model := range modelsToAdd {
		model.ProviderID = prov.ID
		err = store.AddModel(&model)
		if err != nil {
			log.Printf("Failed to add model %s for provider %s: %v", model.Name, prov.Name, err)
		} else {
			log.Printf("Added model %s with ID: %d for provider %s", model.Name, model.ID, prov.Name)
		}
	}
}
