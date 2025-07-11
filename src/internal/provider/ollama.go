package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/offbeat-studio/allama/internal/models"
)

// OllamaProvider handles interactions with the Ollama API
type OllamaProvider struct {
	Host   string
	client *http.Client
}

// NewOllamaProvider creates a new instance of OllamaProvider
func NewOllamaProvider(host string) *OllamaProvider {
	return &OllamaProvider{
		Host: host,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetModels retrieves the list of available models from Ollama
func (p *OllamaProvider) GetModels() ([]models.Model, error) {
	url := fmt.Sprintf("%s/api/tags", p.Host)
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
	url := fmt.Sprintf("%s/api/chat", p.Host)
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

// ForwardRequest forwards a raw request to Ollama and returns the raw response
func (p *OllamaProvider) ForwardRequest(method, path string, body []byte, headers map[string]string) ([]byte, int, error) {
	url := fmt.Sprintf("%s%s", p.Host, path)

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, 0, err
	}

	// Copy headers from the original request
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return responseBody, resp.StatusCode, nil
}
