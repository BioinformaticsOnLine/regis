package handlers

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/gofiber/fiber/v2"
)

// GetJobMetrics serves the pipeline_summary.json file
func GetJobMetrics(c *fiber.Ctx) error {
	jobID := c.Params("uuid")

	// 1. Get Job from DB to find OutputDir
	var job Job
	if result := db.GetDB().First(&job, "id = ?", jobID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	// 2. Locate summary file
	// Path: OutputDir/16_pipeline_report/pipeline_summary.json
	summaryPath := filepath.Join(job.Config.OutputDir, "16_pipeline_report", "pipeline_summary.json")

	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Metrics summary not found (pipeline might still be running)",
		})
	}

	// 3. Read and verify it's valid JSON
	content, err := os.ReadFile(summaryPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read summary file",
		})
	}

	// 4. Return as JSON
	// We unmarshal and remarshal to ensure it's valid JSON, or just stream it
	// Streaming raw bytes is faster
	c.Set("Content-Type", "application/json")
	return c.Send(content)
}

// DownloadJobResults streams a ZIP archive of the results
func DownloadJobResults(c *fiber.Ctx) error {
	jobID := c.Params("uuid")

	// 1. Get Job
	var job Job
	if result := db.GetDB().First(&job, "id = ?", jobID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	outputDir := job.Config.OutputDir
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Output directory not found",
		})
	}

	// 2. Stream ZIP response
	isLight := c.Query("light") == "true"
	filename := fmt.Sprintf("regis_results_%s.zip", jobID)
	if isLight {
		filename = fmt.Sprintf("regis_results_%s_report.zip", jobID)
	}

	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Set("Content-Type", "application/zip")

	// Use write streaming
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		// Walk output directory
		err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Create relative path for ZIP
			relPath, err := filepath.Rel(outputDir, path)
			if err != nil {
				return err
			}

			// Filter logic for Lightweight Mode
			if isLight && info.IsDir() {
				// Skip Heavy Folders
				if strings.HasPrefix(relPath, "02_trimming") ||
					strings.HasPrefix(relPath, "04_alignment") ||
					strings.HasPrefix(relPath, "05_assembly") { // Assembly can be large too sometimes
					return filepath.SkipDir
				}
			}

			if info.IsDir() {
				return nil
			}

			// Create ZIP file entry
			zipFile, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			// Copy file content
			fsFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fsFile.Close()

			_, err = io.Copy(zipFile, fsFile)
			return err
		})

		if err != nil {
			fmt.Printf("Error creating zip: %v\n", err)
			// Removed 'return false' as the new signature doesn't return a boolean
		}
		// Removed 'return true' as the new signature doesn't return a boolean
	})

	return nil
}
