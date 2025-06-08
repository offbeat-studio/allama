package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/offbeat-studio/allama/internal/config"
	"github.com/offbeat-studio/allama/internal/middleware"
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
	r := &Router{
		cfg:    cfg,
		store:  store,
		router: engine,
	}

	logDir := "logs"
	loggingMiddleware := middleware.LoggingMiddleware(logDir)
	engine.Use(loggingMiddleware)

	return r
}

func (r *Router) SetupRoutes() {
	// ollama API
	r.router.GET("/api/tags", r.listTags)
	r.router.POST("/api/show", r.showModelWithRawBody)

	// API version 1 group
	v1 := r.router.Group("/api/v1")
	v1.GET("/models", r.listModels)
	v1.POST("/chat/completions", r.handleChat)

	// New endpoints
	r.router.POST("/api/generate", r.handleGenerate)
	r.router.POST("/api/chat", r.handleChat)
	r.router.GET("/api/version", r.handleVersion)
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

func (r *Router) handleChat(c *gin.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			errMsg := fmt.Sprintf("panic recovered in handleChat: %v", rec)
			fmt.Println(errMsg)
			c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		}
	}()

	// Read raw body first
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("handleChat: failed to read request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	// Reset body for further reading
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	// Determine provider from model in raw body
	var temp struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &temp); err != nil {
		fmt.Printf("handleChat: invalid request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	providerName := r.determineProviderFromModel(temp.Model)
	if providerName == "" {
		fmt.Println("handleChat: unsupported model")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported model"})
		return
	}

	prov, err := r.store.GetProviderByName(providerName)
	if err != nil || prov == nil {
		fmt.Printf("handleChat: provider not found: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not found"})
		return
	}

	if providerName == "ollama" {
		// Forward raw body directly to Ollama
		r.forwardOllamaRequestWithBody(c, prov, "/api/chat", body)
		return
	}

	// For other providers, unmarshal into struct
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var requestBody struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	if err := json.Unmarshal(body, &requestBody); err != nil {
		fmt.Printf("handleChat: invalid request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	providerImpl := provider.CreateProvider(prov)
	if providerImpl == nil {
		fmt.Println("handleChat: unsupported provider")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider"})
		return
	}

	// Convert []Message to []map[string]string for providerImpl.Chat
	messages := make([]map[string]string, len(requestBody.Messages))
	for i, msg := range requestBody.Messages {
		messages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	responseContent, err := providerImpl.Chat(requestBody.Model, messages)

	if err != nil {
		fmt.Printf("handleChat: provider chat error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Transform response to Ollama format for non-Ollama providers
	transformer := provider.NewOllamaResponseTransformer()
	transformedResponse, err := transformer.TransformChatResponse(responseContent, requestBody.Model)
	if err != nil {
		fmt.Printf("handleChat: response transformation error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transform response"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", transformedResponse)
}

// handleGenerate processes generate requests and redirects to the appropriate provider
func (r *Router) handleGenerate(c *gin.Context) {
	var requestBody struct {
		Model  string                 `json:"model"`
		Prompt string                 `json:"prompt"`
		Params map[string]interface{} `json:"parameters"`
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

	if providerName == "ollama" {
		r.forwardOllamaRequest(c, prov, "/api/generate")
		return
	}

	providerImpl := provider.CreateProvider(prov)
	if providerImpl == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider"})
		return
	}

	// Since providerImpl does not have Generate method, use Chat with prompt wrapped as message
	responseContent, err := providerImpl.Chat(requestBody.Model, []map[string]string{
		{
			"role":    "user",
			"content": requestBody.Prompt,
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Transform response to Ollama generate format for non-Ollama providers
	transformer := provider.NewOllamaResponseTransformer()
	transformedResponse, err := transformer.TransformGenerateResponse(responseContent, requestBody.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transform response"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", transformedResponse)
}

// forwardOllamaRequest forwards a request directly to Ollama
func (r *Router) forwardOllamaRequest(c *gin.Context, prov *models.Provider, path string) {
	var body []byte
	var err error
	if c.Request.Body != nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		// Log the request body for debugging
		fmt.Printf("forwardOllamaRequest: forwarding body: %s\n", string(body))
		// Log headers for debugging
		for key, values := range c.Request.Header {
			fmt.Printf("forwardOllamaRequest: header %s: %v\n", key, values)
		}
		// Reset the request body so it can be read again if needed
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	ollamaProvider := provider.NewOllamaProvider(prov.Host)

	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	responseBody, statusCode, err := ollamaProvider.ForwardRequest(c.Request.Method, path, body, headers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(statusCode, "application/json", responseBody)
}

// forwardOllamaRequestWithBody forwards a request with a specific body to Ollama
func (r *Router) forwardOllamaRequestWithBody(c *gin.Context, prov *models.Provider, path string, body []byte) {
	ollamaProvider := provider.NewOllamaProvider(prov.Host)

	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	responseBody, statusCode, err := ollamaProvider.ForwardRequest(c.Request.Method, path, body, headers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(statusCode, "application/json", responseBody)
}

// determineProviderFromModel retrieves the provider name associated with a model ID from the database
func (r *Router) determineProviderFromModel(modelID string) string {
	if modelID == "" {
		return ""
	}

	providers, err := r.store.GetActiveProviders()
	if err != nil {
		return ""
	}

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

	return ""
}

// listTags retrieves and aggregates model tags from all active providers, presenting them as Ollama models
func (r *Router) listTags(c *gin.Context) {
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

// showModelWithRawBody handles the /api/show endpoint by forwarding to Ollama
func (r *Router) showModelWithRawBody(c *gin.Context) {
	// Read raw body first
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("showModelWithRawBody: failed to read request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Determine provider from model in raw body
	var temp struct {
		Name string `json:"model"`
	}
	if err := json.Unmarshal(body, &temp); err != nil {
		fmt.Printf("showModelWithRawBody: invalid request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	providerName := r.determineProviderFromModel(temp.Name)
	if providerName == "" {
		fmt.Println("showModelWithRawBody: unsupported model")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported model"})
		return
	}

	prov, err := r.store.GetProviderByName(providerName)
	if err != nil || prov == nil {
		fmt.Printf("showModelWithRawBody: provider not found: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not found"})
		return
	}

	if providerName == "ollama" {
		// Forward raw body directly to Ollama
		r.forwardOllamaRequestWithBody(c, prov, "/api/show", body)
		return
	}

	// For non-Ollama providers, return a response matching Ollama API format
	c.JSON(http.StatusOK, gin.H{
		"license":    "",
		"modelfile":  fmt.Sprintf("# Model: %s\n# Provider: %s", temp.Name, providerName),
		"parameters": "",
		"template":   "",
		"details": gin.H{
			"parent_model":       "",
			"format":             "gguf",
			"family":             "llama",
			"families":           []string{"llama"},
			"parameter_size":     "7B",
			"quantization_level": "Q4_0",
		},
		"model_info": gin.H{
			"general.architecture":       "llama",
			"general.file_type":          2,
			"general.parameter_count":    7000000000,
			"llama.context_length":       128000,
			"llama.embedding_length":     128000,
			"llama.block_count":          32,
			"llama.attention.head_count": 32,
		},
		"capabilities": []string{"completion", "tools"},
	})
}

// handleVersion handles the /api/version endpoint
func (r *Router) handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "0.1.0",
	})
}
