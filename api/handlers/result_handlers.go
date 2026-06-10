package handlers

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/gofiber/fiber/v2"
)

// FileEntry describes a single file or directory in the browser response.
type FileEntry struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`     // "file" or "dir"
	Size     int64     `json:"size"`     // bytes; 0 for directories
	Modified time.Time `json:"modified"` // last modification time
	Path     string    `json:"path"`     // path relative to the job output dir
}

// safeRelPath resolves a user-supplied relative path inside baseDir and
// returns the cleaned absolute path. Returns ("", false) if the resolved
// path escapes baseDir (path-traversal guard).
func safeRelPath(baseDir, userPath string) (string, bool) {
	// Normalise: strip leading slashes / dots so the user can pass "." or ""
	// for the root of the output directory.
	if userPath == "" || userPath == "." {
		return baseDir, true
	}
	joined := filepath.Join(baseDir, filepath.Clean("/"+userPath))
	rel, err := filepath.Rel(baseDir, joined)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return joined, true
}

// GetJobMetrics serves the pipeline_summary.json file
// @Summary Get job metrics
// @Description Get computational metrics (runtime, resources) for a completed job
// @Tags jobs
// @Produce json
// @Param uuid path string true "Job UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /jobs/{uuid}/results/metrics [get]
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
// @Summary Download job results
// @Description Download the full output directory as a ZIP file
// @Tags jobs
// @Produce application/zip
// @Param uuid path string true "Job UUID"
// @Success 200 {file} file
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /jobs/{uuid}/results/download [get]
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

// BrowseJobFiles lists the contents of a directory inside a job's output folder.
// @Summary Browse job output files
// @Description List files and directories at a given path inside the job output directory.
// @Tags jobs
// @Produce json
// @Param uuid path string true "Job UUID"
// @Param path query string false "Relative sub-path to browse (default: root of output dir)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /jobs/{uuid}/results/files [get]
func BrowseJobFiles(c *fiber.Ctx) error {
	jobID := c.Params("uuid")

	var job Job
	if result := db.GetDB().First(&job, "id = ?", jobID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Job not found"})
	}

	outputDir := job.Config.OutputDir
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Output directory not found"})
	}

	subPath := c.Query("path", "")
	targetDir, ok := safeRelPath(outputDir, subPath)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid path"})
	}

	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Path not found or is not a directory"})
	}

	dirEntries, err := os.ReadDir(targetDir)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read directory"})
	}

	// Directories first, then files — both groups sorted alphabetically.
	sort.Slice(dirEntries, func(i, j int) bool {
		iDir := dirEntries[i].IsDir()
		jDir := dirEntries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		fi, err := de.Info()
		if err != nil {
			continue
		}

		entryType := "file"
		if de.IsDir() {
			entryType = "dir"
		}

		// Build path relative to the output root so the frontend always works
		// from a stable base regardless of the current browsing depth.
		absEntry := filepath.Join(targetDir, de.Name())
		rel, err := filepath.Rel(outputDir, absEntry)
		if err != nil {
			continue
		}

		entries = append(entries, FileEntry{
			Name:     de.Name(),
			Type:     entryType,
			Size:     fi.Size(),
			Modified: fi.ModTime().UTC(),
			Path:     rel,
		})
	}

	// Breadcrumb: compute the current path relative to the output root.
	currentRel, _ := filepath.Rel(outputDir, targetDir)

	return c.JSON(fiber.Map{
		"job_id":  jobID,
		"path":    currentRel,
		"entries": entries,
	})
}

// DownloadJobSelection streams a ZIP of the caller-selected files and folders.
// @Summary Download selected files/folders
// @Description Stream a ZIP archive containing only the specified paths (files or directories) from the job output.
// @Tags jobs
// @Produce application/zip
// @Param uuid path string true "Job UUID"
// @Param paths query []string true "Relative paths to include (repeat the param for multiple)"
// @Success 200 {file} file
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /jobs/{uuid}/results/files/download [get]
func DownloadJobSelection(c *fiber.Ctx) error {
	jobID := c.Params("uuid")

	var job Job
	if result := db.GetDB().First(&job, "id = ?", jobID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Job not found"})
	}

	outputDir := job.Config.OutputDir
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Output directory not found"})
	}

	selectedPaths := c.Query("paths")
	rawPaths := c.Context().QueryArgs().PeekMulti("paths")
	if len(rawPaths) == 0 && selectedPaths == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No paths specified. Use ?paths=rel/path (repeat for multiple)"})
	}

	// Deduplicate and validate all requested paths up front.
	seen := make(map[string]struct{})
	var validPaths []string
	for _, raw := range rawPaths {
		p := string(raw)
		if _, already := seen[p]; already {
			continue
		}
		seen[p] = struct{}{}

		abs, ok := safeRelPath(outputDir, p)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid path: %s", p)})
		}
		if _, err := os.Stat(abs); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": fmt.Sprintf("Path not found: %s", p)})
		}
		validPaths = append(validPaths, abs)
	}

	zipName := fmt.Sprintf("regis_%s_selection.zip", jobID[:8])
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipName))
	c.Set("Content-Type", "application/zip")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		addToZip := func(absPath string) error {
			return filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, err := filepath.Rel(outputDir, path)
				if err != nil {
					return err
				}
				zf, err := zipWriter.Create(rel)
				if err != nil {
					return err
				}
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(zf, f)
				return err
			})
		}

		for _, abs := range validPaths {
			if err := addToZip(abs); err != nil {
				fmt.Printf("Error zipping %s: %v\n", abs, err)
			}
		}
	})

	return nil
}
