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
	Name       string `json:"name"` // User-friendly name
	ModelID    string `json:"model_id"` // Actual ID used by the provider
	IsActive   bool   `json:"is_active"`
}

// Message represents a single message in a chat conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the request body for the /chat/completions endpoint
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// GenerateRequest represents the request body for the /api/generate endpoint
type GenerateRequest struct {
	Model  string                 `json:"model"`
	Prompt string                 `json:"prompt"`
	Params map[string]interface{} `json:"parameters,omitempty"` // omitempty if you want to allow no params
}

// ShowModelRequest represents the request body for the /api/show endpoint
// Note: The task description mentioned 'temp.Name', but Ollama uses 'model' for /api/show
// For consistency with Ollama /api/show, using 'model' field.
// If 'name' is strictly required by an external client for this specific endpoint,
// this needs to be clarified. Assuming 'model' for now.
type ShowModelRequest struct {
	Model string `json:"model"`
}

// ModelEntry represents a single model in the list returned by /api/v1/models
type ModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"` // Typically "model"
	Created int64  `json:"created"` // Timestamp, using int64 for flexibility
	OwnedBy string `json:"owned_by"`
}

// ListModelsResponse represents the response structure for /api/v1/models
type ListModelsResponse struct {
	Object string       `json:"object"` // Typically "list"
	Data   []ModelEntry `json:"data"`
}

// TagEntry represents a single tag (model) in the list returned by /api/tags
type TagEntry struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
}

// ListTagsResponse represents the response structure for /api/tags
type ListTagsResponse struct {
	Models []TagEntry `json:"models"`
}

// VersionResponse represents the response for the /api/version endpoint
type VersionResponse struct {
	Version string `json:"version"`
}

// ErrorResponse represents a generic error message structure
type ErrorResponse struct {
	Error string `json:"error"`
}

// OllamaChatCompletionMessage represents a message part of an Ollama chat completion response.
type OllamaChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatCompletionChoice represents a choice in an Ollama chat completion response.
type OllamaChatCompletionChoice struct {
	Index        int                         `json:"index"`
	Message      OllamaChatCompletionMessage `json:"message"`
	FinishReason string                      `json:"finish_reason"`
}

// OllamaChatCompletionResponse represents the structure of a chat completion response from Ollama.
// This is used when transforming responses from other providers to match Ollama's format.
type OllamaChatCompletionResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"` // e.g., "chat.completion"
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []OllamaChatCompletionChoice `json:"choices"`
	// Usage can be added here if needed
}

// OllamaGenerateResponse represents the structure of a generate response from Ollama.
// This is used when transforming responses from other providers to match Ollama's format.
// Note: Ollama's generate API streams responses by default. This struct represents a single (potentially final) message.
// For a streaming response, the structure might be different or repeated.
type OllamaGenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"` // Timestamp string
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	// Context, TotalDuration, etc. can be added if needed for full compatibility
}

// ShowModelDetail represents the 'details' part of the /api/show response.
type ShowModelDetail struct {
	ParentModel      string   `json:"parent_model"`
	Format           string   `json:"format"`
	Family           string   `json:"family"`
	Families         []string `json:"families"`
	ParameterSize    string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ShowModelInfo represents the 'model_info' part of the /api/show response.
type ShowModelInfo struct {
	GeneralArchitecture    string `json:"general.architecture"`
	GeneralFileType        int    `json:"general.file_type"`
	GeneralParameterCount  uint64 `json:"general.parameter_count"` // Using uint64 for large numbers
	LlamaContextLength     uint32 `json:"llama.context_length"`
	LlamaEmbeddingLength   uint32 `json:"llama.embedding_length"`
	LlamaBlockCount        uint32 `json:"llama.block_count"`
	LlamaAttentionHeadCount uint32 `json:"llama.attention.head_count"`
}

// ShowModelResponse represents the response for the /api/show endpoint (for non-Ollama providers).
type ShowModelResponse struct {
	License      string          `json:"license"`
	Modelfile    string          `json:"modelfile"`
	Parameters   string          `json:"parameters"`
	Template     string          `json:"template"`
	Details      ShowModelDetail `json:"details"`
	ModelInfo    ShowModelInfo   `json:"model_info"`
	Capabilities []string        `json:"capabilities,omitempty"`
}
