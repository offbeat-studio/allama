package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/offbeat-studio/allama/internal/models"
)

// OllamaProvider handles interactions with the Ollama API
type OllamaProvider struct {
	Endpoint string
	client   *http.Client
}

// NewOllamaProvider creates a new instance of OllamaProvider
func NewOllamaProvider(endpoint string) *OllamaProvider {
	return &OllamaProvider{
		Endpoint: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetModels retrieves the list of available models from Ollama
func (p *OllamaProvider) GetModels() ([]models.Model, error) {
	url := fmt.Sprintf("%s/api/tags", p.Endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var modelsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	var modelList []models.Model
	for _, m := range modelsResp.Models {
		modelList = append(modelList, models.Model{
			Name:     m.Name,
			ModelID:  m.Name,
			IsActive: true,
		})
	}

	return modelList, nil
}

// Chat sends a chat request to Ollama and returns the response
func (p *OllamaProvider) Chat(modelID string, messages []map[string]string) (string, error) {
	url := fmt.Sprintf("%s/api/chat", p.Endpoint)
	payload := map[string]interface{}{
		"model":    modelID,
		"messages": messages,
		"stream":   false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var chatResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	return chatResp.Message.Content, nil
}
