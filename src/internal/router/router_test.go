// src/internal/router/router_test.go
package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/offbeat-studio/allama/internal/config"
	"github.com/offbeat-studio/allama/internal/models"
	"github.com/offbeat-studio/allama/internal/provider" // Added for mocking provider.CreateProvider
	"github.com/offbeat-studio/allama/internal/storage"  // For storage.Store interface
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTime is used to override time.Now in tests
var MockTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// --- Mocks ---

// MockStorage is a mock implementation of the storage.Storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) GetProviderByName(name string) (*models.Provider, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Provider), args.Error(1)
}

func (m *MockStorage) GetModelsByProviderID(providerID int) ([]models.Model, error) {
	args := m.Called(providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Model), args.Error(1)
}

func (m *MockStorage) GetActiveProviders() ([]models.Provider, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Provider), args.Error(1)
}

// Add dummy implementations for other storage.Store methods if needed by NewRouter or SetupRoutes
func (m *MockStorage) InitDB(configpath string) error {
	args := m.Called(configpath)
	return args.Error(0)
}
func (m *MockStorage) AddProvider(p *models.Provider) error {
	args := m.Called(p)
	return args.Error(0)
}
func (m *MockStorage) AddModel(mdl *models.Model) error {
	args := m.Called(mdl)
	return args.Error(0)
}
func (m *MockStorage) GetModelByID(id int) (*models.Model, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}
func (m *MockStorage) UpdateProvider(p *models.Provider) error {
	args := m.Called(p)
	return args.Error(0)
}
func (m *MockStorage) UpdateModel(mdl *models.Model) error {
	args := m.Called(mdl)
	return args.Error(0)
}
func (m *MockStorage) DeleteProvider(id int) error {
	args := m.Called(id)
	return args.Error(0)
}
func (m *MockStorage) DeleteModel(id int) error {
	args := m.Called(id)
	return args.Error(0)
}
func (m *MockStorage) GetAllModels() ([]models.Model, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Model), args.Error(1)
}


// MockProvider is a mock implementation of the provider.ProviderInterface
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) GetModels() ([]models.Model, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Model), args.Error(1)
}

func (m *MockProvider) Chat(modelID string, messages []map[string]string) (string, error) {
	args := m.Called(modelID, messages)
	return args.String(0), args.Error(1)
}

// --- Test Functions ---

func TestGenerateFakeOllamaResponse(t *testing.T) {
	originalTimeNow := timeNow
	timeNow = func() time.Time { return MockTime }
	defer func() { timeNow = originalTimeNow }()

	modelID := "test-model"
	responseContent := "This is a test response."
	expectedID := fmt.Sprintf("chatcmpl-%d", MockTime.UnixNano())

	response := generateFakeOllamaResponse(modelID, responseContent)

	assert.Equal(t, expectedID, response["id"])
	assert.Equal(t, "chat.completion", response["object"])
	assert.Equal(t, MockTime.Unix(), response["created"])
	assert.Equal(t, modelID, response["model"])

	choices, ok := response["choices"].([]gin.H)
	assert.True(t, ok)
	assert.Len(t, choices, 1)
	assert.Equal(t, 0, choices[0]["index"])
	assert.Equal(t, "stop", choices[0]["finish_reason"])

	message, ok := choices[0]["message"].(gin.H)
	assert.True(t, ok)
	assert.Equal(t, "assistant", message["role"])
	assert.Equal(t, responseContent, message["content"])

	usage, ok := response["usage"].(gin.H)
	assert.True(t, ok)
	assert.Equal(t, 0, usage["prompt_tokens"])
	assert.Equal(t, 0, usage["completion_tokens"])
	assert.Equal(t, 0, usage["total_tokens"])
}

func setupTestRouter(t *testing.T, store storage.Store, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// Ensure store can be cast to *MockStorage or whatever concrete type NewRouter expects if it's not the interface.
	// For this test setup, NewRouter is expected to take the storage.Store interface.
	// However, our NewRouter function takes *storage.Storage.
	// So we pass the MockStorage directly.
	routerInstance := NewRouter(cfg, store.(*MockStorage), engine)
	routerInstance.SetupRoutes()
	return engine
}


func TestHandleChat_OllamaProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		var receivedBody map[string]interface{}
		err = json.Unmarshal(bodyBytes, &receivedBody)
		assert.NoError(t, err)
		assert.Equal(t, "ollama-model/llama2", receivedBody["model"])

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Ollama-Header", "ollama-value")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"response": "proxied ollama response"}`)
	}))
	defer ollamaServer.Close()

	mockStorage := new(MockStorage)
	ollamaProvider := &models.Provider{ID: 1, Name: "ollama", Host: ollamaServer.URL, IsActive: true}
	mockStorage.On("GetProviderByName", "ollama").Return(ollamaProvider, nil)

	cfg := &config.Config{}
	engine := setupTestRouter(t, mockStorage, cfg)


	requestBody := gin.H{
		"model":    "ollama-model/llama2",
		"messages": []map[string]string{{"role": "user", "content": "Hello"}},
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "my-value")

	rr := httptest.NewRecorder()
	engine.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Equal(t, "ollama-value", rr.Header().Get("X-Custom-Ollama-Header"))
	assert.JSONEq(t, `{"response": "proxied ollama response"}`, rr.Body.String())

	mockStorage.AssertExpectations(t)
}


func TestHandleChat_NonOllamaFakeResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalTimeNow := timeNow
	timeNow = func() time.Time { return MockTime }
	defer func() { timeNow = originalTimeNow }()

	mockStorage := new(MockStorage)
	mockNonOllamaChatProvider := new(MockProvider)


	nonOllamaProv := &models.Provider{ID: 2, Name: "test-provider", APIKey: "test-key", Host: "http://testhost", IsActive: true}
	nonOllamaModel := models.Model{ID: 1, ProviderID: 2, Name: "test-model-id", ModelID: "test-model-id", IsActive: true}

	// Mocking determineProviderFromModel logic
	mockStorage.On("GetActiveProviders").Return([]models.Provider{*nonOllamaProv}, nil)
	mockStorage.On("GetModelsByProviderID", nonOllamaProv.ID).Return([]models.Model{nonOllamaModel}, nil)
	// Mocking GetProviderByName for the direct call after determineProviderFromModel
	mockStorage.On("GetProviderByName", "test-provider").Return(nonOllamaProv, nil)

	originalCreateProviderFunc := provider.CreateProvider
	provider.CreateProvider = func(p *models.Provider) provider.ProviderInterface {
		if p.Name == "test-provider" {
			return mockNonOllamaChatProvider
		}
		return originalCreateProviderFunc(p)
	}
	defer func() { provider.CreateProvider = originalCreateProviderFunc }()

	expectedContent := "Response from test-provider"
	mockNonOllamaChatProvider.On("Chat", "test-model-id", mock.AnythingOfType("[]map[string]string")).Return(expectedContent, nil)


	cfg := &config.Config{}
	engine := setupTestRouter(t, mockStorage, cfg)

	requestBody := gin.H{
		"model":    "test-model-id",
		"messages": []map[string]string{{"role": "user", "content": "Hello non-ollama"}},
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	engine.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &responseBody)
	assert.NoError(t, err)

	assert.Equal(t, "chatcmpl-"+fmt.Sprintf("%d", MockTime.UnixNano()), responseBody["id"])
	assert.Equal(t, "chat.completion", responseBody["object"])
	assert.Equal(t, float64(MockTime.Unix()), responseBody["created"])
	assert.Equal(t, "test-model-id", responseBody["model"])

	choices, ok := responseBody["choices"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, choices, 1)
	choice1 := choices[0].(map[string]interface{})
	assert.Equal(t, float64(0), choice1["index"]) // JSON numbers are float64
	assert.Equal(t, "stop", choice1["finish_reason"])

	message := choice1["message"].(map[string]interface{})
	assert.Equal(t, "assistant", message["role"])
	assert.Equal(t, expectedContent, message["content"])

	usage := responseBody["usage"].(map[string]interface{})
	assert.Equal(t, float64(0), usage["prompt_tokens"])
	assert.Equal(t, float64(0), usage["completion_tokens"])
	assert.Equal(t, float64(0), usage["total_tokens"])

	mockStorage.AssertExpectations(t)
	mockNonOllamaChatProvider.AssertExpectations(t)
}


func TestDetermineProviderFromModel(t *testing.T) {
	mockStorage := new(MockStorage)

	activeProviders := []models.Provider{{ID: 1, Name: "provider1", IsActive: true}}
	modelsForProvider1 := []models.Model{{ID: 10, ProviderID: 1, ModelID: "model-abc", IsActive: true}}

	mockStorage.On("GetActiveProviders").Return(activeProviders, nil).Once()
	mockStorage.On("GetModelsByProviderID", 1).Return(modelsForProvider1, nil).Once()

	cfg := &config.Config{}
	// Pass mockStorage as storage.Store, which is fine since *MockStorage implements storage.Store
	router := NewRouter(cfg, mockStorage, gin.New())

	providerName := router.determineProviderFromModel("model-abc")
	assert.Equal(t, "provider1", providerName)

	mockStorage.On("GetActiveProviders").Return(activeProviders, nil).Once()
	mockStorage.On("GetModelsByProviderID", 1).Return(modelsForProvider1, nil).Once()

	providerName = router.determineProviderFromModel("unknown-model")
	assert.Equal(t, "", providerName)

	providerName = router.determineProviderFromModel("")
	assert.Equal(t, "", providerName)

	mockStorage.AssertExpectations(t)
}

func TestListAndShowRoutes(t *testing.T) {
	mockStorage := new(MockStorage)
	cfg := &config.Config{}
	engine := setupTestRouter(t, mockStorage, cfg)

	provider1 := models.Provider{ID: 1, Name: "ollama", Host: "http://ollama", IsActive: true}
	provider2 := models.Provider{ID: 2, Name: "another", Host: "http://another", IsActive: true}

	// --- Test /api/tags error path ---
	mockStorage.On("GetActiveProviders").Return(nil, fmt.Errorf("db error")).Once()
	reqTags, _ := http.NewRequest("GET", "/api/tags", nil)
	rrTags := httptest.NewRecorder()
	engine.ServeHTTP(rrTags, reqTags)
	assert.Equal(t, http.StatusInternalServerError, rrTags.Code)
	mockStorage.AssertExpectations(t) // Reset expectations for next test segment
	// Manually clear mock calls for the next independent test segment with the same mock
	mockStorage.ExpectedCalls = nil
	mockStorage.Calls = []mock.Call{}


	// --- Test /api/show success path (DB fallback) ---
	anotherModelsDB := []models.Model{{ModelID: "another/model1", Name: "another/model1", ProviderID: 2, IsActive: true}}
	mockStorage.On("GetActiveProviders").Return([]models.Provider{provider2}, nil).Once()

	originalCreateProviderFunc := provider.CreateProvider
	mockAnotherProvider := new(MockProvider) // Defined here for this specific test case
	provider.CreateProvider = func(p *models.Provider) provider.ProviderInterface {
		if p.Name == "another" {
			mockAnotherProvider.On("GetModels").Return(nil, fmt.Errorf("api error")).Once()
			return mockAnotherProvider
		}
		return originalCreateProviderFunc(p)
	}
	defer func() { provider.CreateProvider = originalCreateProviderFunc }()

	mockStorage.On("GetModelsByProviderID", provider2.ID).Return(anotherModelsDB, nil).Once()

	showBody := gin.H{"model": "another/model1"}
	jsonShowBody, _ := json.Marshal(showBody)
	reqShow, _ := http.NewRequest("POST", "/api/show", bytes.NewBuffer(jsonShowBody))
	reqShow.Header.Set("Content-Type", "application/json")
	rrShow := httptest.NewRecorder()
	engine.ServeHTTP(rrShow, reqShow)

	assert.Equal(t, http.StatusOK, rrShow.Code)
	var showResp gin.H
	json.Unmarshal(rrShow.Body.Bytes(), &showResp)
	assert.Equal(t, "# Model information from local database", showResp["modelfile"])
	mockStorage.AssertExpectations(t)
	mockAnotherProvider.AssertExpectations(t)
}


func TestHandleChat_OllamaProviderNotConfigured(t *testing.T) {
    gin.SetMode(gin.TestMode)
    mockStorage := new(MockStorage)
    // Simulate error or nil provider when fetching "ollama"
    mockStorage.On("GetProviderByName", "ollama").Return(nil, fmt.Errorf("ollama not found in db")).Once()

    cfg := &config.Config{}
    engine := setupTestRouter(t, mockStorage, cfg)

    requestBody := gin.H{"model": "ollama-model", "messages": []map[string]string{{"role": "user", "content": "Hello"}}}
    jsonBody, _ := json.Marshal(requestBody)
    req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    engine.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusInternalServerError, rr.Code)
    var respBody gin.H
    json.Unmarshal(rr.Body.Bytes(), &respBody)
    assert.Equal(t, "Ollama provider not configured or host not found", respBody["error"])
    mockStorage.AssertExpectations(t)

	// Test case where provider is found but host is empty
	mockStorage.On("GetProviderByName", "ollama").Return(&models.Provider{Name: "ollama", Host: ""}, nil).Once()
	rr = httptest.NewRecorder()
    engine.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
    json.Unmarshal(rr.Body.Bytes(), &respBody)
    assert.Equal(t, "Ollama provider not configured or host not found", respBody["error"])
    mockStorage.AssertExpectations(t)
}


func TestHandleChat_OllamaProxyError(t *testing.T) {
    gin.SetMode(gin.TestMode)

    mockStorage := new(MockStorage)
    // Use a non-resolvable or invalid host to cause a connection error
    ollamaProvider := &models.Provider{ID: 1, Name: "ollama", Host: "http://invalid-ollama-host-that-will-fail-connect", IsActive: true}
    mockStorage.On("GetProviderByName", "ollama").Return(ollamaProvider, nil)

    cfg := &config.Config{}
    engine := setupTestRouter(t, mockStorage, cfg)

    requestBody := gin.H{
        "model":    "ollama-model/llama2",
        "messages": []map[string]string{{"role": "user", "content": "Hello"}},
    }
    jsonBody, _ := json.Marshal(requestBody)

    req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    engine.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusBadGateway, rr.Code)
    var errorResp gin.H
    json.Unmarshal(rr.Body.Bytes(), &errorResp)
    assert.True(t, strings.HasPrefix(errorResp["error"].(string), "Failed to proxy request to Ollama:"), "Error message mismatch")


    mockStorage.AssertExpectations(t)
}

func TestHandleChat_UnsupportedModel(t *testing.T) {
    gin.SetMode(gin.TestMode)
    mockStorage := new(MockStorage)
    cfg := &config.Config{}

    // Setup determineProviderFromModel to return ""
    mockStorage.On("GetActiveProviders").Return(nil, nil) // No active providers, or no models match

    engine := setupTestRouter(t, mockStorage, cfg)

    requestBody := gin.H{
        "model":    "unknown-model",
        "messages": []map[string]string{{"role": "user", "content": "Hello"}},
    }
    jsonBody, _ := json.Marshal(requestBody)

    req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    engine.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusBadRequest, rr.Code)
    var errorResp gin.H
    json.Unmarshal(rr.Body.Bytes(), &errorResp)
    assert.Equal(t, "Unsupported model", errorResp["error"])

    mockStorage.AssertExpectations(t)
}

func TestHandleChat_ProviderChatError(t *testing.T) {
    gin.SetMode(gin.TestMode)
	originalTimeNow := timeNow
	timeNow = func() time.Time { return MockTime } // Not strictly needed here but good for consistency
	defer func() { timeNow = originalTimeNow }()

    mockStorage := new(MockStorage)
    mockChatProvider := new(MockProvider)

    prov := &models.Provider{ID: 2, Name: "error-provider", IsActive: true}
    model := models.Model{ID: 1, ProviderID: 2, ModelID: "error-model", IsActive: true}

    mockStorage.On("GetActiveProviders").Return([]models.Provider{*prov}, nil)
    mockStorage.On("GetModelsByProviderID", prov.ID).Return([]models.Model{model}, nil)
    mockStorage.On("GetProviderByName", "error-provider").Return(prov, nil)

	originalCreateProviderFunc := provider.CreateProvider
	provider.CreateProvider = func(p *models.Provider) provider.ProviderInterface {
		if p.Name == "error-provider" {
			return mockChatProvider
		}
		return originalCreateProviderFunc(p)
	}
	defer func() { provider.CreateProvider = originalCreateProviderFunc }()

    mockChatProvider.On("Chat", "error-model", mock.AnythingOfType("[]map[string]string")).Return("", fmt.Errorf("provider chat failed"))

    cfg := &config.Config{}
    engine := setupTestRouter(t, mockStorage, cfg)


    requestBody := gin.H{
        "model":    "error-model",
        "messages": []map[string]string{{"role": "user", "content": "Trigger error"}},
    }
    jsonBody, _ := json.Marshal(requestBody)
    req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    engine.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusInternalServerError, rr.Code)
    var errorResp gin.H
    json.Unmarshal(rr.Body.Bytes(), &errorResp)
    assert.Equal(t, "Chat completion error: provider chat failed", errorResp["error"])

    mockStorage.AssertExpectations(t)
    mockChatProvider.AssertExpectations(t)
}

func TestHandleChat_InvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStorage := new(MockStorage) // Will not be used but NewRouter needs it
	cfg := &config.Config{}
	engine := setupTestRouter(t, mockStorage, cfg)

	req, _ := http.NewRequest("POST", "/api/v1/chat/completions", bytes.NewBufferString("this is not json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	engine.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var respBody gin.H
	json.Unmarshal(rr.Body.Bytes(), &respBody)
	assert.Contains(t, respBody["error"], "Invalid request body")
}

func TestHandleChat_BodyReadError(t *testing.T) {
    gin.SetMode(gin.TestMode)
    mockStorage := new(MockStorage)
    cfg := &config.Config{}
    engine := setupTestRouter(t, mockStorage, cfg)

    // Create a reader that will always return an error
    errorReader := &ErrorReader{}

    req, _ := http.NewRequest("POST", "/api/v1/chat/completions", errorReader)
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    engine.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusInternalServerError, rr.Code)
    var respBody gin.H
    json.Unmarshal(rr.Body.Bytes(), &respBody)
    assert.Equal(t, "Failed to read request body", respBody["error"])
}

// ErrorReader is a helper struct that implements io.Reader and always returns an error.
type ErrorReader struct{}

func (er *ErrorReader) Read(p []byte) (n int, err error) {
    return 0, fmt.Errorf("simulated read error")
}
func (er *ErrorReader) Close() error { return nil }
