// Package provider provides configurations for different AI providers.
package provider

import "os"

// ProviderConfig defines the configuration for a provider.
type ProviderConfig struct {
	Name         string
	Host         string
	EnableEnvVar string
	ApiKeyEnvVar string
}

// GetProviderConfigs returns a list of provider configurations.
func GetProviderConfigs() []ProviderConfig {
	return []ProviderConfig{
		{Name: "openai", Host: os.Getenv("OPENAI_HOST"), EnableEnvVar: "IS_OPENAI_ACTIVE", ApiKeyEnvVar: "OPENAI_API_KEY"},
		{Name: "anthropic", Host: os.Getenv("ANTHROPIC_HOST"), EnableEnvVar: "IS_ANTHROPIC_ACTIVE", ApiKeyEnvVar: "ANTHROPIC_API_KEY"},
		{Name: "ollama", Host: os.Getenv("OLLAMA_HOST"), EnableEnvVar: "IS_OLLAMA_ACTIVE", ApiKeyEnvVar: "OLLAMA_API_KEY"},
	}
}
