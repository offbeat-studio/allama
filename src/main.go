package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nickhuang/allama/internal/config"
	"github.com/nickhuang/allama/internal/models"
	"github.com/nickhuang/allama/internal/router"
	"github.com/nickhuang/allama/internal/storage"
)

func main() {
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
	initializeDefaultData(store)

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

// initializeDefaultData checks for and inserts default data into the database if necessary.
func initializeDefaultData(store *storage.Storage) {
	// Check if there are any providers in the database
	providers, err := store.GetActiveProviders()
	if err != nil {
		log.Printf("Failed to check for existing providers: %v", err)
		return
	}

	// If there are no providers, add default ones
	if len(providers) == 0 {
		log.Println("No providers found, initializing default data...")

		// Add a default OpenAI provider
		openAIProvider := &models.Provider{
			Name:     "openai",
			APIKey:   "", // API key should be set via environment variables or configuration
			Endpoint: "https://api.openai.com",
			IsActive: true,
		}
		err = store.AddProvider(openAIProvider)
		if err != nil {
			log.Printf("Failed to add OpenAI provider: %v", err)
		} else {
			log.Printf("Added OpenAI provider with ID: %d", openAIProvider.ID)

			// Add a default model for the provider
			gptModel := &models.Model{
				ProviderID: openAIProvider.ID,
				Name:       "GPT-4",
				ModelID:    "gpt-4",
				IsActive:   true,
			}
			err = store.AddModel(gptModel)
			if err != nil {
				log.Printf("Failed to add GPT-4 model: %v", err)
			} else {
				log.Printf("Added GPT-4 model with ID: %d", gptModel.ID)
			}
		}

		// You can add more default providers and models here as needed
	} else {
		log.Println("Providers already exist, skipping default data initialization.")
	}
}
