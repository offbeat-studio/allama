// Package models contains the data structures and types used throughout the Allama application.
// This includes request and response payloads, database models, and other important entities.
package models

// Provider represents a configured AI provider.
// It holds the necessary information to connect to and authenticate with an AI service.
type Provider struct {
	ID       int    `json:"id"`        // ID is the unique identifier for the provider in the database.
	Name     string `json:"name"`      // Name is the common name of the provider (e.g., "openai", "anthropic", "ollama").
	APIKey   string `json:"api_key"`   // APIKey is the API key used for authenticating with the provider's service.
	Host     string `json:"host"`      // Host is the base URL for the provider's API.
	IsActive bool   `json:"is_active"` // IsActive indicates whether this provider configuration is currently enabled.
}

// Model represents a specific AI model available through a provider.
// It stores details about the model, including its association with a provider.
type Model struct {
	ID         int    `json:"id"`          // ID is the unique identifier for the model in the database.
	ProviderID int    `json:"provider_id"` // ProviderID is the foreign key linking this model to its provider.
	Name       string `json:"name"`        // Name is a user-friendly name for the model (e.g., "GPT-4 Turbo").
	ModelID    string `json:"model_id"`    // ModelID is the specific identifier used by the provider for this model (e.g., "gpt-4-1106-preview").
	IsActive   bool   `json:"is_active"`   // IsActive indicates whether this model is currently available for use.
}
