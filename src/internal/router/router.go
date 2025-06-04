// src/internal/router/router.go modification for testable time
// Package router implements the HTTP routing for the Allama API.
// It defines API endpoints and maps them to handler functions
// that interact with providers and the storage layer.
package router

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time" // Keep this standard import

	"github.com/gin-gonic/gin"
	"github.com/offbeat-studio/allama/internal/config"
	"github.com/offbeat-studio/allama/internal/provider"
	"github.com/offbeat-studio/allama/internal/storage"
)

// timeNow is a variable that can be replaced by a mock in tests.
var timeNow = time.Now


// Router handles API routing and provider redirection logic.
// It manages the interaction between incoming HTTP requests and the appropriate handlers,
// as well as coordinating with storage and provider services.
type Router struct {
	cfg    *config.Config   // cfg is the application configuration.
	store  *storage.Storage // store provides access to the application's data storage.
	router *gin.Engine      // router is the Gin HTTP router engine.
}

// NewRouter creates a new instance of Router.
// It initializes the router with application configuration, storage access, and the Gin engine.
// cfg is the application configuration.
// store is the storage instance.
// engine is the Gin HTTP router engine.
// It returns a pointer to the initialized Router.
func NewRouter(cfg *config.Config, store *storage.Storage, engine *gin.Engine) *Router {
	return &Router{
		cfg:    cfg,
		store:  store,
		router: engine,
	}
}

// SetupRoutes defines the API endpoints and maps them to their respective handler functions.
// This method configures the HTTP routes for the application.
func (r *Router) SetupRoutes() {
	r.router.GET("/api/tags", r.listTags)
	r.router.POST("/api/show", r.showModel)
	v1 := r.router.Group("/api/v1")
	v1.GET("/models", r.listModels)
	v1.POST("/chat/completions", r.handleChat)
}

// listModels handles requests to list available AI models.
// It fetches active models from all configured providers and returns them in a structured format.
// The response includes a list of models with their ID, object type, creation timestamp (dummy), and owner.
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
			localModels, errDb := r.store.GetModelsByProviderID(prov.ID)
			if errDb == nil {
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

// handleChat processes chat completion requests.
// It determines the appropriate provider based on the requested model,
// proxies the request to Ollama if the model is an Ollama model,
// or forwards the request to the configured provider for other models.
// The request body is expected to contain:
//   - "model": string, the ID of the model to use.
//   - "messages": []map[string]string, a list of messages in the chat history.
//   - "stream": *bool (optional), whether to stream the response.
// It returns a JSON response with the chat completion or an error.
func (r *Router) handleChat(c *gin.Context) {
	var requestBody struct {
		Model    string              `json:"model"`
		Messages []map[string]string `json:"messages"`
		Stream   *bool               `json:"stream"`
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, c.Request.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}
	firstPassReader := bytes.NewReader(buf.Bytes())
	secondPassReader := bytes.NewReader(buf.Bytes())

	c.Request.Body = io.NopCloser(firstPassReader)

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	c.Request.Body = io.NopCloser(secondPassReader)

	if strings.Contains(strings.ToLower(requestBody.Model), "ollama") {
		ollamaProv, err := r.store.GetProviderByName("ollama")
		if err != nil || ollamaProv == nil || ollamaProv.Host == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ollama provider not configured or host not found"})
			return
		}

		targetURL := ollamaProv.Host + "/api/chat"

		proxyReq, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy request: " + err.Error()})
			return
		}

		proxyReq.Header = make(http.Header)
		for h, val := range c.Request.Header {
			proxyReq.Header[h] = val
		}
		if proxyReq.Header.Get("Content-Type") == "" {
			proxyReq.Header.Set("Content-Type", "application/json")
		}

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to proxy request to Ollama: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			if key == "Content-Encoding" && strings.Contains(strings.Join(values, ","), "gzip") {
				continue
			}
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		c.Writer.WriteHeader(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
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

	providerImpl := provider.CreateProvider(prov)
	if providerImpl == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider for model"})
		return
	}

	responseContent, err := providerImpl.Chat(requestBody.Model, requestBody.Messages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chat completion error: " + err.Error()})
		return
	}

	fakeResponse := generateFakeOllamaResponse(requestBody.Model, responseContent)
	c.JSON(http.StatusOK, fakeResponse)
}

// generateFakeOllamaResponse creates a standardized Ollama-like JSON response for chat completions.
// modelID is the ID of the model used.
// responseContent is the content of the assistant's message.
// It returns a gin.H map representing the JSON response.
func generateFakeOllamaResponse(modelID string, responseContent string) gin.H {
	return gin.H{
		"id":      "chatcmpl-" + generateID(),
		"object":  "chat.completion",
		"created": timeNow().Unix(), // Use timeNow for testability
		"model":   modelID,
		"choices": []gin.H{
			{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": responseContent,
				},
				"finish_reason": "stop",
			},
		},
		"usage": gin.H{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
	}
}

func (r *Router) determineProviderFromModel(modelID string) string {
	if modelID == "" {
		return ""
	}

	providers, err := r.store.GetActiveProviders()
	if err != nil {
		return ""
	}

	for _, prov := range providers {
		models, errDb := r.store.GetModelsByProviderID(prov.ID)
		if errDb != nil {
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

// generateID creates a simple unique ID for responses
// It now uses timeNow() for testability.
func generateID() string {
	return fmt.Sprintf("%d", timeNow().UnixNano())
}

// listTags handles requests to list available model tags, similar to Ollama's /api/tags endpoint.
// It retrieves models from active providers and formats them in a way that mimics Ollama's tag listing.
// Each tag includes the model name, a fixed modification timestamp, size (dummy), and digest (dummy).
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
			localModels, errDb := r.store.GetModelsByProviderID(prov.ID)
			if errDb == nil {
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

// showModel handles requests to show details for a specific model, similar to Ollama's /api/show endpoint.
// It expects a JSON request body with a "model" field specifying the model name.
// It fetches details for the requested model from active providers and returns them in a structured format.
// The details include license, modelfile content, parameters, template, and other model-specific information.
// If the model is not found, it returns a 404 error.
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
			localModels, errDb := r.store.GetModelsByProviderID(prov.ID)
			if errDb == nil {
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
