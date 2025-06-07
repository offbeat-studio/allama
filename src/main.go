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

	_ "myproject/docs" // Import the generated docs
)

// @title Allama API
// @version 1.0
// @description This is the Allama API server.
// @termsOfService http://yourapi.com/terms
// @contact.name API Support
// @contact.url http://yourapi.com/support
// @contact.email support@yourapi.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /api/v1
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

	// Get provider configurations
	providers := provider.GetProviderConfigs()

	// Iterate over provider configurations to initialize enabled providers
	for _, p := range providers {
		if enable := os.Getenv(p.EnableEnvVar); enable == "true" {
			prov := &models.Provider{
				Name:     p.Name,
				APIKey:   os.Getenv(p.ApiKeyEnvVar),
				Host:     p.Host,
				IsActive: true,
			}
			err := store.AddProvider(prov)
			if err != nil {
				log.Printf("Failed to add %s provider: %v", p.Name, err)
			} else {
				log.Printf("Added %s provider with ID: %d", p.Name, prov.ID)
				// Fetch available models from provider API
				provider.FetchModelsForProvider(store, prov)
			}
		} else {
			log.Printf("%s provider not enabled (%s is not set to 'true')", p.Name, p.EnableEnvVar)
		}
	}
}
