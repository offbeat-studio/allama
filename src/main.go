package main

import (
	"log"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/offbeat-studio/allama/internal/config"
	"github.com/offbeat-studio/allama/internal/models"
	"github.com/offbeat-studio/allama/internal/provider"
	"github.com/offbeat-studio/allama/internal/router"
	"github.com/offbeat-studio/allama/internal/storage"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Overload()
	if err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database storage
	store, err := storage.NewStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize default data
	initializeDefaultData(store, cfg)

	// Initialize Gin router
	ginRouter := gin.Default()

	// Define a simple health check endpoint
	ginRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Setup API routes
	apiRouter := router.NewRouter(cfg, store, ginRouter)
	apiRouter.SetupRoutes()

	// Start the server
	serverAddr := ":" + cfg.Port
	if err := ginRouter.Run(serverAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initializeDefaultData deletes the existing database and inserts default data into the database.
func initializeDefaultData(store *storage.Storage, cfg *config.Config) {
	log.Println("Initializing default data...")

	// Reset the database to ensure a clean state on each run
	if err := store.ResetDatabase(cfg.DatabasePath); err != nil {
		log.Printf("Failed to reset database: %v", err)
	} else {
		log.Println("Database reset successful")
	}

	// Add a default OpenAI provider
	// API key is loaded from .env file using godotenv.Load()
	openAIProvider := &models.Provider{
		Name:     "openai",
		APIKey:   os.Getenv("OPENAI_API_KEY"),
		Endpoint: "https://api.openai.com",
		IsActive: true,
	}
	err := store.AddProvider(openAIProvider)
	if err != nil {
		log.Printf("Failed to add OpenAI provider: %v", err)
	} else {
		log.Printf("Added OpenAI provider with ID: %d", openAIProvider.ID)
		// Fetch available models from OpenAI API
		fetchModelsForProvider(store, openAIProvider)
	}

	// Add a default Anthropic provider
	// API key is loaded from .env file using godotenv.Load()
	anthropicProvider := &models.Provider{
		Name:     "anthropic",
		APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		Endpoint: "https://api.anthropic.com",
		IsActive: true,
	}
	err = store.AddProvider(anthropicProvider)
	if err != nil {
		log.Printf("Failed to add Anthropic provider: %v", err)
	} else {
		log.Printf("Added Anthropic provider with ID: %d", anthropicProvider.ID)
		// Fetch available models from Anthropic API
		fetchModelsForProvider(store, anthropicProvider)
	}

	// Add a default Ollama provider
	// API key is loaded from .env file using godotenv.Load(), though not typically required for Ollama
	ollamaProvider := &models.Provider{
		Name:     "ollama",
		APIKey:   os.Getenv("OLLAMA_API_KEY"),
		Endpoint: "http://localhost:11434", // Default local Ollama endpoint
		IsActive: true,
	}
	err = store.AddProvider(ollamaProvider)
	if err != nil {
		log.Printf("Failed to add Ollama provider: %v", err)
	} else {
		log.Printf("Added Ollama provider with ID: %d", ollamaProvider.ID)
		// Fetch available models from Ollama API
		fetchModelsForProvider(store, ollamaProvider)
	}
}

// fetchModelsForProvider fetches available models from the provider's API and adds them to the database.
func fetchModelsForProvider(store *storage.Storage, prov *models.Provider) {
	log.Printf("Fetching models for provider: %s", prov.Name)

	var modelsToAdd []models.Model
	var err error

	// Use provider-specific logic to fetch models
	switch prov.Name {
	case "openai":
		openAIProvider := provider.NewOpenAIProvider(prov.APIKey)
		modelsToAdd, err = openAIProvider.GetModels()
		if err != nil {
			log.Printf("Failed to fetch models for OpenAI: %v", err)
			return
		}
	case "anthropic":
		anthropicProvider := provider.NewAnthropicProvider(prov.APIKey)
		modelsToAdd, err = anthropicProvider.GetModels()
		if err != nil {
			log.Printf("Failed to fetch models for Anthropic: %v", err)
			return
		}
	case "ollama":
		ollamaProvider := provider.NewOllamaProvider(prov.Endpoint)
		modelsToAdd, err = ollamaProvider.GetModels()
		if err != nil {
			log.Printf("Failed to fetch models for Ollama: %v", err)
			return
		}
	default:
		log.Printf("Unknown provider: %s, cannot fetch models", prov.Name)
		return
	}

	// Add fetched models to the database
	for _, model := range modelsToAdd {
		model.ProviderID = prov.ID
		err = store.AddModel(&model)
		if err != nil {
			log.Printf("Failed to add model %s for provider %s: %v", model.Name, prov.Name, err)
		} else {
			log.Printf("Added model %s with ID: %d for provider %s", model.Name, model.ID, prov.Name)
		}
	}
}
