package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nickhuang/allama/internal/config"
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
