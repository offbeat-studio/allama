package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nickhuang/allama/internal/models"
)

// AnthropicProvider handles interactions with the Anthropic API
type AnthropicProvider struct {
	APIKey string
	client *http.Client
}

// NewAnthropicProvider creates a new instance of AnthropicProvider
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		APIKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetModels retrieves the list of available models from Anthropic
func (p *AnthropicProvider) GetModels() ([]models.Model, error) {
	url := "https://api.anthropic.com/v1/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	var modelList []models.Model
	for _, m := range modelsResp.Data {
		modelList = append(modelList, models.Model{
			Name:     m.Name,
			ModelID:  m.ID,
			IsActive: true,
		})
	}

	return modelList, nil
}

// Chat sends a chat request to Anthropic and returns the response
func (p *AnthropicProvider) Chat(modelID string, messages []map[string]string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	// Convert messages to Anthropic format
	var anthropicMessages []map[string]interface{}
	var systemMessage string
	for _, msg := range messages {
		role := msg["role"]
		content := msg["content"]
		if role == "system" {
			systemMessage = content
		} else {
			// Ensure role is compatible with Anthropic API (e.g., 'user' or 'assistant')
			anthropicRole := role
			if role == "user" || role == "assistant" {
				anthropicRole = role
			} else {
				// Default to 'user' for unknown roles to maintain compatibility
				anthropicRole = "user"
			}
			anthropicMessages = append(anthropicMessages, map[string]interface{}{
				"role":    anthropicRole,
				"content": content,
			})
		}
	}

	payload := map[string]interface{}{
		"model":      modelID,
		"max_tokens": 1024,
		"messages":   anthropicMessages,
		"system":     systemMessage,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var chatResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Content) > 0 {
		return chatResp.Content[0].Text, nil
	}
	return "", fmt.Errorf("no response content found")
}
