package models

// Provider represents an AI service provider configuration
type Provider struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	APIKey   string `json:"api_key"`
	Host     string `json:"host"`
	IsActive bool   `json:"is_active"`
}

// Model represents a specific AI model offered by a provider
type Model struct {
	ID         int    `json:"id"`
	ProviderID int    `json:"provider_id"`
	Name       string `json:"name"`
	ModelID    string `json:"model_id"`
	IsActive   bool   `json:"is_active"`
}
