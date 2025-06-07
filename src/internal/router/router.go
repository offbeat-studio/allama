package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/offbeat-studio/allama/internal/config"
	"github.com/offbeat-studio/allama/internal/middleware"
	"github.com/offbeat-studio/allama/internal/models"
	"github.com/offbeat-studio/allama/internal/provider"
)

// StorageInterface defines the interface that storage must implement
// Not used by handlers directly, but good for context.
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
// @Summary List available models
// @Description Retrieves a list of all available models from active providers and the local database.
// @Description Models are presented in a format similar to OpenAI's /v1/models endpoint.
// @Tags Models
// @Accept json
// @Produce json
// @Success 200 {object} models.ListModelsResponse "A list of available models"
// @Failure 500 {object} models.ErrorResponse "Internal server error if providers cannot be retrieved"
// @Router /api/v1/models [get]
func (r *Router) listModels(c *gin.Context) {
	providers, err := r.store.GetActiveProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to retrieve providers"})
		return
	}

	var modelEntries []models.ModelEntry
	processedModels := make(map[string]bool) // To track processed model IDs and avoid duplicates

	for _, prov := range providers {
		providerImpl := provider.CreateProvider(prov)
		if providerImpl == nil {
			continue
		}

		// Attempt to fetch from provider API first
		fetchedModels, err := providerImpl.GetModels()
		if err == nil {
			for _, model := range fetchedModels {
				if _, exists := processedModels[model.ModelID]; !exists {
					modelEntries = append(modelEntries, models.ModelEntry{
						ID:      model.ModelID,
						Object:  "model",
						Created: time.Now().Unix(), // Using current time as created time
						OwnedBy: prov.Name,
					})
					processedModels[model.ModelID] = true
				}
			}
		}

		// Fallback or supplement with local DB models
		localModels, err := r.store.GetModelsByProviderID(prov.ID)
		if err == nil {
			for _, model := range localModels {
				if model.IsActive {
					if _, exists := processedModels[model.ModelID]; !exists {
						modelEntries = append(modelEntries, models.ModelEntry{
							ID:      model.ModelID,
							Object:  "model",
							Created: time.Now().Unix(), // Using current time as created time
							OwnedBy: prov.Name,
						})
						processedModels[model.ModelID] = true
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, models.ListModelsResponse{
		Object: "list",
		Data:   modelEntries,
	})
}

// handleChat handles chat completion requests.
// @Summary Generate chat completions
// @Description Processes a chat request. If the model is from Ollama, the request is forwarded directly.
// @Description For other providers, the request is processed, and the response is transformed into Ollama's chat completion format.
// @Description The provider is determined based on the 'model' field in the request body.
// @Tags Chat
// @Accept json
// @Produce json
// @Param chatRequest body models.ChatRequest true "Chat request payload. The 'model' field is used to determine the provider."
// @Success 200 {object} models.OllamaChatCompletionResponse "Chat completion response, formatted like Ollama's /api/chat response."
// @Failure 400 {object} models.ErrorResponse "Bad request if the request body is invalid, model is missing, or model/provider is unsupported."
// @Failure 500 {object} models.ErrorResponse "Internal server error if provider cannot be found or if there's an error during processing or response transformation."
// @Router /api/chat [post]
// @Router /api/v1/chat/completions [post]
func (r *Router) handleChat(c *gin.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			errMsg := fmt.Sprintf("panic recovered in handleChat: %v", rec)
			fmt.Println(errMsg)
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: errMsg})
		}
	}()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("handleChat: failed to read request body: %v\n", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body for further reading

	var chatReq models.ChatRequest
	// First, try to unmarshal into ChatRequest to get the model name
	// This is a bit redundant as we might unmarshal again later for non-Ollama
	// but necessary to determine the provider from the model.
	var tempReq struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &tempReq); err != nil {
		fmt.Printf("handleChat: invalid request body (for model detection): %v\n", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body: missing model field"})
		return
	}

	providerName := r.determineProviderFromModel(tempReq.Model)
	if providerName == "" {
		fmt.Println("handleChat: unsupported model")
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Unsupported model: " + tempReq.Model})
		return
	}

	prov, err := r.store.GetProviderByName(providerName)
	if err != nil || prov == nil {
		fmt.Printf("handleChat: provider not found for model %s: %v\n", tempReq.Model, err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Provider not found for model: " + tempReq.Model})
		return
	}

	if providerName == "ollama" {
		r.forwardOllamaRequestWithBody(c, prov, "/api/chat", body)
		return
	}

	// Now, unmarshal the full request for non-Ollama providers
	if err := json.Unmarshal(body, &chatReq); err != nil {
		fmt.Printf("handleChat: invalid request body (full unmarshal): %v\n", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	providerImpl := provider.CreateProvider(prov)
	if providerImpl == nil {
		fmt.Println("handleChat: unsupported provider implementation")
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Unsupported provider: " + providerName})
		return
	}

	messagesForProvider := make([]map[string]string, len(chatReq.Messages))
	for i, msg := range chatReq.Messages {
		messagesForProvider[i] = map[string]string{"role": msg.Role, "content": msg.Content}
	}

	responseContent, err := providerImpl.Chat(chatReq.Model, messagesForProvider)
	if err != nil {
		fmt.Printf("handleChat: provider chat error: %v\n", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	transformer := provider.NewOllamaResponseTransformer()
	transformedResponse, err := transformer.TransformChatResponse(responseContent, chatReq.Model)
	if err != nil {
		fmt.Printf("handleChat: response transformation error: %v\n", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to transform response"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", transformedResponse)
}

// handleGenerate processes generate requests and redirects to the appropriate provider
// @Summary Generate text based on a prompt
// @Description Processes a generate request. If the model is from Ollama, the request is forwarded directly.
// @Description For other providers, the request is adapted (using chat endpoint) and the response is transformed into Ollama's generate format.
// @Description The provider is determined based on the 'model' field in the request body.
// @Tags Generate
// @Accept json
// @Produce json
// @Param generateRequest body models.GenerateRequest true "Generate request payload. The 'model' field is used to determine the provider."
// @Success 200 {object} models.OllamaGenerateResponse "Generate response, formatted like Ollama's /api/generate response. Note: This might be a single (final) response object even if the underlying provider streams."
// @Failure 400 {object} models.ErrorResponse "Bad request if the request body is invalid or model/provider is unsupported."
// @Failure 500 {object} models.ErrorResponse "Internal server error if provider cannot be found or if there's an error during processing or response transformation."
// @Router /api/generate [post]
func (r *Router) handleGenerate(c *gin.Context) {
	var genReq models.GenerateRequest
	if err := c.ShouldBindJSON(&genReq); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body: " + err.Error()})
		return
	}

	providerName := r.determineProviderFromModel(genReq.Model)
	if providerName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Unsupported model: " + genReq.Model})
		return
	}

	prov, err := r.store.GetProviderByName(providerName)
	if err != nil || prov == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Provider not found for model: " + genReq.Model})
		return
	}

	if providerName == "ollama" {
		// For Ollama, we need to pass the original request body
		// ShouldBindJSON consumes the body, so we need to re-serialize genReq or pass original if available
		// For simplicity here, we re-serialize. A more robust way might involve reading body once and passing.
		originalBody, err := json.Marshal(genReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to re-serialize request for Ollama"})
			return
		}
		// Reset the request body with the original content if needed by forwardOllamaRequestWithBody
		// However, forwardOllamaRequest (not WithBody) is called below. Let's adjust.
		// If forwardOllamaRequest reads from c.Request.Body, it needs to be repopulated.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(originalBody))
		r.forwardOllamaRequest(c, prov, "/api/generate") // forwardOllamaRequest reads c.Request.Body
		return
	}

	providerImpl := provider.CreateProvider(prov)
	if providerImpl == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Unsupported provider: " + providerName})
		return
	}

	responseContent, err := providerImpl.Chat(genReq.Model, []map[string]string{
		{"role": "user", "content": genReq.Prompt},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	transformer := provider.NewOllamaResponseTransformer()
	transformedResponse, err := transformer.TransformGenerateResponse(responseContent, genReq.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to transform response"})
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
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Failed to read request body"})
			return
		}
		fmt.Printf("forwardOllamaRequest: forwarding body: %s\n", string(body))
		// Reset body for potential re-reads by other middlewares/handlers (though not typical here)
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
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.Header("Content-Type", "application/json") // Assuming Ollama always responds with JSON
	c.Data(statusCode, "application/json", responseBody)
}

// forwardOllamaRequestWithBody forwards a request with a specific body to Ollama
func (r *Router) forwardOllamaRequestWithBody(c *gin.Context, prov *models.Provider, path string, body []byte) {
	fmt.Printf("forwardOllamaRequestWithBody: forwarding body: %s\n", string(body))

	ollamaProvider := provider.NewOllamaProvider(prov.Host)
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		// Filter out problematic headers like "Content-Length" which will be set by the HTTP client
		if key != "Content-Length" {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	responseBody, statusCode, err := ollamaProvider.ForwardRequest(c.Request.Method, path, body, headers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.Header("Content-Type", "application/json") // Assuming Ollama always responds with JSON
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

// generateID creates a simple unique ID for responses
func generateID() string {
	return fmt.Sprintf("%d", time.Now().Nanosecond())
}

// listTags retrieves and aggregates model tags from all active providers, presenting them as Ollama models
// @Summary List available model tags (Ollama compatible)
// @Description Retrieves a list of all available model tags from active providers and the local database.
// @Description The response is formatted to be compatible with Ollama's /api/tags endpoint.
// @Tags Models
// @Accept json
// @Produce json
// @Success 200 {object} models.ListTagsResponse "A list of available model tags"
// @Failure 500 {object} models.ErrorResponse "Internal server error if providers cannot be retrieved"
// @Router /api/tags [get]
func (r *Router) listTags(c *gin.Context) {
	providers, err := r.store.GetActiveProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to retrieve providers"})
		return
	}

	var tagEntries []models.TagEntry
	processedModels := make(map[string]bool) // To track processed model IDs

	for _, prov := range providers {
		providerImpl := provider.CreateProvider(prov)
		if providerImpl == nil {
			continue
		}

		// Attempt to fetch from provider API first
		fetchedModels, err := providerImpl.GetModels()
		if err == nil {
			for _, model := range fetchedModels {
				if _, exists := processedModels[model.ModelID]; !exists {
					tagEntries = append(tagEntries, models.TagEntry{
						Name:       model.ModelID,
						ModifiedAt: time.Now().UTC().Format(time.RFC3339), // Using current time
						Size:       0,                                     // Placeholder, actual size often unknown
						Digest:     "",                                    // Placeholder
					})
					processedModels[model.ModelID] = true
				}
			}
		}

		// Fallback or supplement with local DB models
		localModels, err := r.store.GetModelsByProviderID(prov.ID)
		if err == nil {
			for _, model := range localModels {
				if model.IsActive {
					if _, exists := processedModels[model.ModelID]; !exists {
						tagEntries = append(tagEntries, models.TagEntry{
							Name:       model.ModelID,
							ModifiedAt: time.Now().UTC().Format(time.RFC3339),
							Size:       0,
							Digest:     "",
						})
						processedModels[model.ModelID] = true
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, models.ListTagsResponse{
		Models: tagEntries,
	})
}

// showModelWithRawBody handles the /api/show endpoint
// @Summary Show model information (Ollama compatible)
// @Description Retrieves information about a specific model. If the model is from Ollama, the request is forwarded directly.
// @Description For other providers, a mock response is generated that mimics Ollama's /api/show format.
// @Description The provider is determined based on the 'model' field (or 'name' if that was the intended field for ShowModelRequest) in the request body.
// @Tags Models
// @Accept json
// @Produce json
// @Param showRequest body models.ShowModelRequest true "Request payload containing the model name. The 'model' field is used to determine the provider."
// @Success 200 {object} models.ShowModelResponse "Detailed information about the model, formatted like Ollama's /api/show response."
// @Failure 400 {object} models.ErrorResponse "Bad request if the request body is invalid or model/provider is unsupported."
// @Failure 500 {object} models.ErrorResponse "Internal server error if provider cannot be found."
// @Router /api/show [post]
func (r *Router) showModelWithRawBody(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("showModelWithRawBody: failed to read request body: %v\n", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body

	var showReq models.ShowModelRequest
	if err := json.Unmarshal(body, &showReq); err != nil {
		fmt.Printf("showModelWithRawBody: invalid request body: %v\n", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body: " + err.Error()})
		return
	}

	providerName := r.determineProviderFromModel(showReq.Model)
	if providerName == "" {
		fmt.Println("showModelWithRawBody: unsupported model")
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Unsupported model: " + showReq.Model})
		return
	}

	prov, err := r.store.GetProviderByName(providerName)
	if err != nil || prov == nil {
		fmt.Printf("showModelWithRawBody: provider not found for model %s: %v\n", showReq.Model, err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Provider not found for model: " + showReq.Model})
		return
	}

	if providerName == "ollama" {
		r.forwardOllamaRequestWithBody(c, prov, "/api/show", body)
		return
	}

	// For non-Ollama providers, return a mock response matching Ollama's /api/show format
	// The details here are placeholders as per the original code's logic.
	resp := models.ShowModelResponse{
		License:    "",
		Modelfile:  fmt.Sprintf("# Model: %s\n# Provider: %s", showReq.Model, providerName),
		Parameters: "",
		Template:   "",
		Details: models.ShowModelDetail{
			ParentModel:       "",
			Format:            "gguf", // Placeholder
			Family:            "generic", // Placeholder
			Families:          []string{"generic"}, // Placeholder
			ParameterSize:     "N/A", // Placeholder
			QuantizationLevel: "N/A", // Placeholder
		},
		ModelInfo: models.ShowModelInfo{ // Placeholder values
			GeneralArchitecture:    "transformer",
			GeneralFileType:        1,
			GeneralParameterCount:  0,
			LlamaContextLength:     2048,
			LlamaEmbeddingLength:   2048,
			LlamaBlockCount:        24,
			LlamaAttentionHeadCount:16,
		},
		Capabilities: []string{"completion"}, // Assuming completion by default
	}
	c.JSON(http.StatusOK, resp)
}

// handleVersion handles the /api/version endpoint
// @Summary Get API version
// @Description Returns the current version of the API.
// @Tags Version
// @Accept json
// @Produce json
// @Success 200 {object} models.VersionResponse "API version information"
// @Router /api/version [get]
func (r *Router) handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, models.VersionResponse{
		Version: "0.1.0", // This should ideally come from a config or build variable
	})
}
