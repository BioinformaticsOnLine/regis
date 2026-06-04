package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// UploadFile handles file uploads strictly to a staging directory
// @Summary Upload a file
// @Description Upload a file for use in the pipeline (e.g., FASTQ, FASTA, GTF). Returns the absolute path to be used in job submission.
// @Tags upload
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /upload [post]
func UploadFile(c *fiber.Ctx) error {
	// 1. Get the file from form data
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing or invalid 'file' field in form data",
		})
	}

	// 2. Generate Staging Directory
	// BaseJobDir/uploads/UUID/
	uploadID := uuid.New().String()
	uploadDir := filepath.Join(BaseJobDir, "uploads", uploadID)
	
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create upload directory: %v", err),
		})
	}

	// 3. Define target absolute path
	// Clean the filename strictly
	safeFilename := filepath.Base(file.Filename)
	targetPath := filepath.Join(uploadDir, safeFilename)

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		// Fallback to relative if Abs fails for some strange reason
		absPath = targetPath 
	}

	// 4. Save the file to disk
	if err := c.SaveFile(file, absPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to save file: %v", err),
		})
	}

	// 5. Respond with the absolute path
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":       "File uploaded successfully",
		"file_path":     absPath,
		"original_name": safeFilename,
		"upload_id":     uploadID,
		"size_bytes":    file.Size,
	})
}
