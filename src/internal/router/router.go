package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nickhuang/allama/internal/config"
	"github.com/nickhuang/allama/internal/provider"
	"github.com/nickhuang/allama/internal/storage"
)

// Router handles API routing and provider redirection logic
type Router struct {
	cfg    *config.Config
	store  *storage.Storage
	router *gin.Engine
}

// NewRouter creates a new instance of Router with provider configurations
func NewRouter(cfg *config.Config, store *storage.Storage, engine *gin.Engine) *Router {
	return &Router{
		cfg:    cfg,
		store:  store,
		router: engine,
	}
}

// SetupRoutes defines the API endpoints and routing logic
func (r *Router) SetupRoutes() {
	// API version 1 group
	v1 := r.router.Group("/api/v1")

	// Models endpoint to list available models from all providers
	v1.GET("/models", r.listModels)

	// Chat endpoint to handle chat requests and redirect to appropriate provider
	v1.POST("/chat/completions", r.handleChat)
}

// listModels retrieves and aggregates models from all active providers
func (r *Router) listModels(c *gin.Context) {
	providers, err := r.store.GetActiveProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve providers"})
		return
	}

	var allModels []interface{}
	for _, prov := range providers {
		var providerImpl interface{}
		switch prov.Name {
		case "openai":
			providerImpl = provider.NewOpenAIProvider(prov.APIKey)
		case "anthropic":
			providerImpl = provider.NewAnthropicProvider(prov.APIKey)
		case "ollama":
			providerImpl = provider.NewOllamaProvider(prov.Endpoint)
		default:
			continue
		}

		var models []interface{}
		switch p := providerImpl.(type) {
		case *provider.OpenAIProvider:
			m, err := p.GetModels()
			if err == nil {
				for _, model := range m {
					models = append(models, gin.H{
						"id":       model.ModelID,
						"object":   "model",
						"created":  0,
						"owned_by": "openai",
					})
				}
			}
		case *provider.AnthropicProvider:
			m, err := p.GetModels()
			if err == nil {
				for _, model := range m {
					models = append(models, gin.H{
						"id":       model.ModelID,
						"object":   "model",
						"created":  0,
						"owned_by": "anthropic",
					})
				}
			}
		case *provider.OllamaProvider:
			m, err := p.GetModels()
			if err == nil {
				for _, model := range m {
					models = append(models, gin.H{
						"id":       model.ModelID,
						"object":   "model",
						"created":  0,
						"owned_by": "ollama",
					})
				}
			}
		}
		allModels = append(allModels, models...)
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   allModels,
	})
}

// handleChat processes chat requests and redirects to the appropriate provider
func (r *Router) handleChat(c *gin.Context) {
	var requestBody struct {
		Model    string              `json:"model"`
		Messages []map[string]string `json:"messages"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Determine provider based on model prefix or configuration
	providerName := determineProviderFromModel(requestBody.Model)
	if providerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported model"})
		return
	}

	prov, err := r.store.GetProviderByName(providerName)
	if err != nil || prov == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not found"})
		return
	}

	var providerImpl interface{}
	switch providerName {
	case "openai":
		providerImpl = provider.NewOpenAIProvider(prov.APIKey)
	case "anthropic":
		providerImpl = provider.NewAnthropicProvider(prov.APIKey)
	case "ollama":
		providerImpl = provider.NewOllamaProvider(prov.Endpoint)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider"})
		return
	}

	var responseContent string
	switch p := providerImpl.(type) {
	case *provider.OpenAIProvider:
		responseContent, err = p.Chat(requestBody.Model, requestBody.Messages)
	case *provider.AnthropicProvider:
		responseContent, err = p.Chat(requestBody.Model, requestBody.Messages)
	case *provider.OllamaProvider:
		responseContent, err = p.Chat(requestBody.Model, requestBody.Messages)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      "chatcmpl-" + generateID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   requestBody.Model,
		"choices": []gin.H{
			{
				"message": gin.H{
					"role":    "assistant",
					"content": responseContent,
				},
				"finish_reason": "stop",
				"index":         0,
			},
		},
	})
}

// determineProviderFromModel extracts provider name from model ID
func determineProviderFromModel(modelID string) string {
	if modelID == "" {
		return ""
	}

	if modelID[0:6] == "claude" {
		return "anthropic"
	} else if modelID[0:3] == "gpt" || modelID[0:5] == "o1-mini" || modelID[0:8] == "o1-preview" {
		return "openai"
	} else {
		// Check if it's an Ollama model (could have various prefixes)
		// For simplicity, assume anything not matching above is Ollama
		return "ollama"
	}
}

// generateID creates a simple unique ID for responses
func generateID() string {
	return fmt.Sprintf("%d", time.Now().Nanosecond())
}
