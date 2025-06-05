package provider

import (
	"encoding/json"
	"log"
	"time"

	"github.com/offbeat-studio/allama/internal/models"
	"github.com/offbeat-studio/allama/internal/storage"
)

// ProviderInterface defines the common interface for all provider implementations.
type ProviderInterface interface {
	GetModels() ([]models.Model, error)
	Chat(modelID string, messages []map[string]string) (string, error)
}

// ResponseTransformer defines the interface for transforming provider responses to Ollama format
type ResponseTransformer interface {
	TransformChatResponse(content string, modelID string) ([]byte, error)
	TransformGenerateResponse(content string, modelID string) ([]byte, error)
}

// OllamaResponseTransformer transforms responses to match Ollama's response formats
type OllamaResponseTransformer struct{}

// NewOllamaResponseTransformer creates a new instance of OllamaResponseTransformer
func NewOllamaResponseTransformer() *OllamaResponseTransformer {
	return &OllamaResponseTransformer{}
}

// TransformChatResponse transforms a simple string response to Ollama's chat response format
func (t *OllamaResponseTransformer) TransformChatResponse(content string, modelID string) ([]byte, error) {
	response := map[string]interface{}{
		"model":      modelID,
		"created_at": time.Now().Format(time.RFC3339),
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": content,
		},
		"done": true,
	}

	return json.Marshal(response)
}

// TransformGenerateResponse transforms a simple string response to Ollama's generate response format
func (t *OllamaResponseTransformer) TransformGenerateResponse(content string, modelID string) ([]byte, error) {
	response := map[string]interface{}{
		"model":      modelID,
		"created_at": time.Now().Format(time.RFC3339),
		"response":   content,
		"done":       true,
	}

	return json.Marshal(response)
}

// CreateProvider creates an instance of the appropriate provider based on the provider name.
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

// FetchModelsForProvider fetches available models from the provider's API and adds them to the database.
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
