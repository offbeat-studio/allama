package provider

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOllamaResponseTransformer_TransformChatResponse(t *testing.T) {
	transformer := NewOllamaResponseTransformer()
	content := "Hello, how can I help you today?"
	modelID := "gpt-3.5-turbo"

	responseBytes, err := transformer.TransformChatResponse(content, modelID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check required fields
	if response["model"] != modelID {
		t.Errorf("Expected model %s, got %v", modelID, response["model"])
	}

	if response["done"] != true {
		t.Errorf("Expected done to be true, got %v", response["done"])
	}

	// Check message structure
	message, ok := response["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected message to be a map, got %T", response["message"])
	}

	if message["role"] != "assistant" {
		t.Errorf("Expected role to be 'assistant', got %v", message["role"])
	}

	if message["content"] != content {
		t.Errorf("Expected content %s, got %v", content, message["content"])
	}

	// Check created_at is a valid timestamp
	createdAt, ok := response["created_at"].(string)
	if !ok {
		t.Errorf("Expected created_at to be a string, got %T", response["created_at"])
	}

	_, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		t.Errorf("Expected created_at to be a valid RFC3339 timestamp, got %s", createdAt)
	}
}

func TestOllamaResponseTransformer_TransformGenerateResponse(t *testing.T) {
	transformer := NewOllamaResponseTransformer()
	content := "This is a generated response."
	modelID := "claude-3-sonnet"

	responseBytes, err := transformer.TransformGenerateResponse(content, modelID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check required fields
	if response["model"] != modelID {
		t.Errorf("Expected model %s, got %v", modelID, response["model"])
	}

	if response["done"] != true {
		t.Errorf("Expected done to be true, got %v", response["done"])
	}

	if response["response"] != content {
		t.Errorf("Expected response %s, got %v", content, response["response"])
	}

	// Check created_at is a valid timestamp
	createdAt, ok := response["created_at"].(string)
	if !ok {
		t.Errorf("Expected created_at to be a string, got %T", response["created_at"])
	}

	_, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		t.Errorf("Expected created_at to be a valid RFC3339 timestamp, got %s", createdAt)
	}
}
