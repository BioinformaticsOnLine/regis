package api

import (
	"github.com/BioinformaticsOnLine/regis/api/handlers"
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes registers all API routes
func (s *Server) SetupRoutes() {
	// API Group
	api := s.App.Group("/api")
	v1 := api.Group("/v1")

	// Health Check
	v1.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Regis API is running",
		})
	})

	// Statistics (Public - No Auth)
	v1.Get("/stats", handlers.GetStats)

	// File Upload (Protected)
	v1.Post("/upload", s.APIKeyMiddleware, handlers.UploadFile)

	jobs := v1.Group("/jobs", s.APIKeyMiddleware)
	jobs.Post("/submit", handlers.SubmitJob)
	jobs.Get("/", handlers.ListJobs) // Listing jobs
	jobs.Get("/:uuid/status", handlers.GetJobStatus)
	jobs.Get("/:uuid/results", handlers.GetJobResults)
	jobs.Get("/:uuid/results/metrics", handlers.GetJobMetrics)
	jobs.Get("/:uuid/results/download", handlers.DownloadJobResults)
	jobs.Get("/:uuid/results/files", handlers.BrowseJobFiles)
	jobs.Get("/:uuid/results/files/download", handlers.DownloadJobSelection)

	// Fallback for unknown routes
	s.App.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Endpoint not found",
		})
	})
}
