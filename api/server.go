package api

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/BioinformaticsOnLine/regis/api/handlers"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/docs"
	"github.com/BioinformaticsOnLine/regis/utils"
	"github.com/BioinformaticsOnLine/regis/version"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

// Set Swagger version dynamically from version.Version
func init() {
	docs.SwaggerInfo.Version = version.Version
}

// @title REGIS Pipeline API
// @version 1.1.4
// @description REST API for controlling the REGIS lncRNA identification pipeline.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@regis.bio

// @license.name GPL-3.0
// @license.url https://github.com/BioinformaticsOnLine/regis/blob/main/LICENSE

// @host localhost:3000
// @BasePath /api/v1

// Server represents the API server
type Server struct {
	App       *fiber.App
	Config    *config.Config
	Validator *validator.Validate
}

// Default upload limit for FASTQ/FASTA and similar (Fiber default is 4 MiB, which yields 413 + confusing CORS errors in browsers).
const defaultBodyLimitBytes = 5 * 1024 * 1024 * 1024 // 5 GiB (large FASTQ uploads)

// allowDevCORSOrigin permits browser calls from local dev and typical private LAN URLs (e.g. Vite on http://192.168.x.x:5173).
func allowDevCORSOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u := strings.ToLower(origin)
	switch {
	case strings.HasPrefix(u, "http://localhost"), strings.HasPrefix(u, "https://localhost"):
		return true
	case strings.HasPrefix(u, "http://127.0.0.1"), strings.HasPrefix(u, "https://127.0.0.1"):
		return true
	case strings.HasPrefix(u, "http://192.168."), strings.HasPrefix(u, "https://192.168."):
		return true
	case strings.HasPrefix(u, "http://10."), strings.HasPrefix(u, "https://10."):
		return true
	case strings.HasPrefix(u, "http://172.17."), strings.HasPrefix(u, "http://172.18."), strings.HasPrefix(u, "http://172.19."),
		strings.HasPrefix(u, "http://172.20."), strings.HasPrefix(u, "http://172.21."), strings.HasPrefix(u, "http://172.22."),
		strings.HasPrefix(u, "http://172.23."), strings.HasPrefix(u, "http://172.24."), strings.HasPrefix(u, "http://172.25."),
		strings.HasPrefix(u, "http://172.26."), strings.HasPrefix(u, "http://172.27."), strings.HasPrefix(u, "http://172.28."),
		strings.HasPrefix(u, "http://172.29."), strings.HasPrefix(u, "http://172.30."), strings.HasPrefix(u, "http://172.31."),
		strings.HasPrefix(u, "https://172.17."), strings.HasPrefix(u, "https://172.18."), strings.HasPrefix(u, "https://172.19."),
		strings.HasPrefix(u, "https://172.20."), strings.HasPrefix(u, "https://172.21."), strings.HasPrefix(u, "https://172.22."),
		strings.HasPrefix(u, "https://172.23."), strings.HasPrefix(u, "https://172.24."), strings.HasPrefix(u, "https://172.25."),
		strings.HasPrefix(u, "https://172.26."), strings.HasPrefix(u, "https://172.27."), strings.HasPrefix(u, "https://172.28."),
		strings.HasPrefix(u, "https://172.29."), strings.HasPrefix(u, "https://172.30."), strings.HasPrefix(u, "https://172.31."):
		return true
	default:
		return false
	}
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "REGIS Pipeline API",
		DisableStartupMessage: false,
		BodyLimit:             defaultBodyLimitBytes,
	})

	// Middleware
	app.Use(logger.New())  // Request logging
	app.Use(recover.New()) // Recover from panics
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: allowDevCORSOrigin,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-API-Key, X-Requested-With",
	}))

	server := &Server{
		App:       app,
		Config:    cfg,
		Validator: validator.New(),
	}

	// Register Swagger route
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	// Setup routes
	server.SetupRoutes()

	return server
}

// APIKeyMiddleware checks for valid API Key in headers or query params
func (s *Server) APIKeyMiddleware(c *fiber.Ctx) error {
	// 1. Check Header
	key := c.Get("X-API-Key")

	// 2. Check Query Param (fallback)
	if key == "" {
		key = c.Query("api_key")
	}

	// 3. Validate
	if key != s.Config.APIKey {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized: Invalid or missing API Key",
		})
	}

	return c.Next()
}

// Listen starts the server
func (s *Server) Listen(addr string) error {
	return s.App.Listen(addr)
}

// StartServer initializes and starts the API server
func StartServer(port, jobDir string) {
	// Initialize Database
	db.Init("regis.db")

	// Record Start Time
	handlers.ServerStartTime = time.Now()

	// Auto Migrate
	if err := db.GetDB().AutoMigrate(&handlers.Job{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Configure Base Job Directory
	handlers.BaseJobDir = jobDir

	// Initialize Logger
	logPath := filepath.Join(jobDir, "server.log")
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		log.Fatalf("Failed to create job dir: %v", err)
	}
	if err := utils.InitLogger(logPath); err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer utils.Sync()

	// Initial Configuration
	// TODO: Load this from a file or env instead of default if needed for the server itself
	cfg := config.NewConfig()
	cfg.EnsureDefaults()

	// Create Server
	server := NewServer(cfg)

	// Initialize Persistent Queue
	sqlDB, err := db.GetSqlDB()
	if err != nil {
		log.Fatalf("Failed to get SQL DB: %v", err)
	}
	handlers.InitQueue(sqlDB)

	// Start Job Worker
	handlers.InitWorker()

	// Start Cleanup Worker
	handlers.StartCleanupWorker(cfg)

	// Custom Startup Banner
	fmt.Print("\n\033[1;36mREGIS Pipeline API Server\033[0m\n")
	fmt.Printf("Version: %s\n", version.Version)
	fmt.Println("──────────────────────────────────────────────────")
	fmt.Printf(" • \033[1mPort\033[0m             : %s\n", port)
	fmt.Printf(" • \033[1mJob Directory\033[0m    : %s\n", jobDir)
	fmt.Printf(" • \033[1mAPI Key\033[0m          : %s\n", cfg.APIKey)
	fmt.Printf(" • \033[1mExecution Mode\033[0m   : %s\n", cfg.ExecutionMode)
	fmt.Printf(" • \033[1mRetention Policy\033[0m : %d days\n", cfg.RetentionDays)
	fmt.Printf(" • \033[1mDocumentation\033[0m    : http://localhost:%s/swagger/index.html\n", port)
	fmt.Println("──────────────────────────────────────────────────")
	fmt.Println()

	if err := server.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
		os.Exit(1)
	}
}
