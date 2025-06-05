package router

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/offbeat-studio/allama/internal/config"
	"github.com/offbeat-studio/allama/internal/models"
	"github.com/offbeat-studio/allama/internal/provider"
)

// StorageInterface defines the interface that storage must implement
type StorageInterface interface {
	GetActiveProviders() ([]*models.Provider, error)
	GetProviderByName(name string) (*models.Provider, error)
	GetModelsByProviderID(providerID int) ([]models.Model, error)
	AddProvider(provider *models.Provider) error
	AddModel(model *models.Model) error
	GetActiveModels() ([]models.Model, error)
	Close() error
	ResetDatabase(databasePath string) error
}

// Router handles API routing and provider redirection logic
type Router struct {
	cfg    *config.Config
	store  StorageInterface
	router *gin.Engine
}

// NewRouter creates a new instance of Router with provider configurations
func NewRouter(cfg *config.Config, store StorageInterface, engine *gin.Engine) *Router {
	return &Router{
		cfg:    cfg,
		store:  store,
		router: engine,
	}
}

// SetupRoutes defines the API endpoints and routing logic
func (r *Router) SetupRoutes() {
	// ollama API
	// Tags endpoint to list available model tags from all providers as if from Ollama, directly under /api
	r.router.GET("/api/tags", r.listTags)

	// Show endpoint to get detailed information about a specific model, directly under /api
	r.router.POST("/api/show", r.showModel)

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
		providerImpl := provider.CreateProvider(prov)
		if providerImpl == nil {
			continue
		}

		var models []interface{}
		// Try fetching models from provider API
		m, err := providerImpl.GetModels()
		if err == nil {
			for _, model := range m {
				models = append(models, gin.H{
					"id":       model.ModelID,
					"object":   "model",
					"created":  0,
					"owned_by": prov.Name,
				})
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
	// Determine provider based on model ID using database lookup
	var requestBody struct {
		Model    string              `json:"model"`
		Messages []map[string]string `json:"messages"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

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

	// If provider is Ollama, forward the request directly
	if providerName == "ollama" {
		r.forwardOllamaRequest(c, prov, "/api/chat")
		return
	}

	// For other providers, use the existing logic
	providerImpl := provider.CreateProvider(prov)
	if providerImpl == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider"})
		return
	}

	responseContent, err := providerImpl.Chat(requestBody.Model, requestBody.Messages)

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

// forwardOllamaRequest forwards a request directly to Ollama
func (r *Router) forwardOllamaRequest(c *gin.Context, prov *models.Provider, path string) {
	// Read the request body (only if it exists)
	var body []byte
	var err error
	if c.Request.Body != nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
	}

	// Create Ollama provider instance
	ollamaProvider := provider.NewOllamaProvider(prov.Host)

	// Prepare headers
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Forward the request
	responseBody, statusCode, err := ollamaProvider.ForwardRequest(c.Request.Method, path, body, headers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set the response content type to JSON
	c.Header("Content-Type", "application/json")
	c.Data(statusCode, "application/json", responseBody)
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

// listTags retrieves and aggregates model tags from all active providers, presenting them as Ollama models
func (r *Router) listTags(c *gin.Context) {
	providers, err := r.store.GetActiveProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve providers"})
		return
	}

	// Check if there's an active Ollama provider and forward the request directly
	for _, prov := range providers {
		if prov.Name == "ollama" {
			r.forwardOllamaRequest(c, prov, "/api/tags")
			return
		}
	}

	// If no Ollama provider found, use the existing aggregation logic
	var allModels []interface{}
	for _, prov := range providers {
		providerImpl := provider.CreateProvider(prov)
		if providerImpl == nil {
			continue
		}

		var models []interface{}
		// Try fetching models from provider API
		m, err := providerImpl.GetModels()
		if err == nil {
			for _, model := range m {
				models = append(models, gin.H{
					"name":        model.ModelID,
					"modified_at": "1970-01-01T00:00:00.000Z",
					"size":        0,
					"digest":      "",
				})
			}
		}

		// If no models fetched from API or error occurred, fall back to local database models
		if len(models) == 0 {
			localModels, err := r.store.GetModelsByProviderID(prov.ID)
			if err == nil {
				for _, model := range localModels {
					if model.IsActive {
						models = append(models, gin.H{
							"name":        model.ModelID,
							"modified_at": "1970-01-01T00:00:00.000Z",
							"size":        0,
							"digest":      "",
						})
					}
				}
			}
		}
		allModels = append(allModels, models...)
	}

	c.JSON(http.StatusOK, gin.H{
		"models": allModels,
	})
}

// showModel retrieves detailed information about a specific model, presenting it as an Ollama model
func (r *Router) showModel(c *gin.Context) {
	var requestBody struct {
		Name string `json:"model"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if requestBody.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model name is required"})
		return
	}

	// Determine provider based on model ID using database lookup
	providerName := r.determineProviderFromModel(requestBody.Name)
	if providerName == "ollama" {
		prov, err := r.store.GetProviderByName(providerName)
		if err == nil && prov != nil {
			r.forwardOllamaRequest(c, prov, "/api/show")
			return
		}
	}

	// If not Ollama or Ollama provider not found, use the existing aggregation logic
	providers, err := r.store.GetActiveProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve providers"})
		return
	}

	var modelDetails interface{}
	found := false
	for _, prov := range providers {
		providerImpl := provider.CreateProvider(prov)
		if providerImpl == nil {
			continue
		}

		// Try fetching models from provider API
		m, err := providerImpl.GetModels()
		if err == nil {
			for _, model := range m {
				if model.ModelID == requestBody.Name {
					modelDetails = gin.H{
						"license":    "Unknown",
						"modelfile":  fmt.Sprintf("# Model information for %s model", prov.Name),
						"parameters": "N/A",
						"template":   "{{ .Prompt }}",
						"system":     "You are a helpful AI assistant.",
						"details": gin.H{
							"parent_model":       "",
							"format":             "gguf",
							"family":             prov.Name,
							"families":           []string{prov.Name},
							"parameter_size":     "unknown",
							"quantization_level": "N/A",
						},
					}
					found = true
					break
				}
			}
		}

		if !found {
			localModels, err := r.store.GetModelsByProviderID(prov.ID)
			if err == nil {
				for _, model := range localModels {
					if model.IsActive && model.ModelID == requestBody.Name {
						modelDetails = gin.H{
							"license":    "Unknown",
							"modelfile":  "# Model information from local database",
							"parameters": "N/A",
							"template":   "{{ .Prompt }}",
							"system":     "You are a helpful AI assistant.",
							"details": gin.H{
								"parent_model":       "",
								"format":             "gguf",
								"family":             prov.Name,
								"families":           []string{prov.Name},
								"parameter_size":     "unknown",
								"quantization_level": "N/A",
							},
						}
						found = true
						break
					}
				}
			}
		}
		if found {
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	c.JSON(http.StatusOK, modelDetails)
}
