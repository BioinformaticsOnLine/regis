package api

import (
	"fmt"
	"log"
	"os"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/BioinformaticsOnLine/regis/api/handlers"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Server represents the API server
type Server struct {
	App    *fiber.App
	Config *config.Config
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "REGIS Pipeline API",
		DisableStartupMessage: false,
	})

	// Middleware
	app.Use(logger.New())  // Request logging
	app.Use(recover.New()) // Recover from panics
	app.Use(cors.New())    // Enable CORS for frontend

	server := &Server{
		App:    app,
		Config: cfg,
	}

	// Setup routes
	server.SetupRoutes()

	return server
}

// Listen starts the server
func (s *Server) Listen(addr string) error {
	return s.App.Listen(addr)
}

// StartServer initializes and starts the API server
func StartServer(port, jobDir string) {
	// Initialize Database
	db.Init("regis.db")

	// Auto Migrate
	if err := db.GetDB().AutoMigrate(&handlers.Job{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Configure Base Job Directory
	handlers.BaseJobDir = jobDir

	// Initial Configuration
	cfg := config.NewConfig()

	// Create Server
	server := NewServer(cfg)

	// Start Job Worker
	handlers.InitWorker()

	// Start Server
	fmt.Printf("Starting REGIS API Server on port %s...\n", port)
	fmt.Printf("Jobs will be stored in: %s\n", jobDir)
	if err := server.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
		os.Exit(1)
	}
}
