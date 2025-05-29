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

// listModels retrieves and aggregates models from all active providers and local database
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
		// Try fetching models from provider API
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

		// If no models fetched from API or error occurred, fall back to local database models
		if len(models) == 0 {
			localModels, err := r.store.GetModelsByProviderID(prov.ID)
			if err == nil {
				for _, model := range localModels {
					if model.IsActive {
						models = append(models, gin.H{
							"id":       model.ModelID,
							"object":   "model",
							"created":  0,
							"owned_by": prov.Name,
						})
					}
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

	// Determine provider based on model ID using database lookup
	providerName := r.determineProviderFromModel(requestBody.Model)
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

// determineProviderFromModel retrieves the provider name associated with a model ID from the database
func (r *Router) determineProviderFromModel(modelID string) string {
	if modelID == "" {
		return ""
	}

	// Use the store instance from the Router struct to query the database
	providers, err := r.store.GetActiveProviders()
	if err != nil {
		return ""
	}

	// Iterate through providers to find a matching model
	for _, prov := range providers {
		models, err := r.store.GetModelsByProviderID(prov.ID)
		if err != nil {
			continue
		}
		for _, model := range models {
			if model.ModelID == modelID {
				return prov.Name
			}
		}
	}

	// If no match found, return empty string
	return ""
}

// generateID creates a simple unique ID for responses
func generateID() string {
	return fmt.Sprintf("%d", time.Now().Nanosecond())
}
