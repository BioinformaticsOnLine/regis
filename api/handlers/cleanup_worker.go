package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// StartCleanupWorker starts a background routine to purge old job files and uploads
func StartCleanupWorker(cfg *config.Config) {
	if cfg.RetentionDays < 0 {
		utils.Logger.Info("Job cleanup disabled (RetentionDays < 0)")
		return
	}

	utils.Logger.Info("Starting Job Cleanup Worker", zap.Int("retention_days", cfg.RetentionDays))

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupOldJobs(cfg.RetentionDays)
				cleanupOldUploads(BaseJobDir, cfg.RetentionDays)
			}
		}
	}()
}

func cleanupOldJobs(retentionDays int) {
	var jobs []Job
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// Find Completed or Failed jobs older than cutoff
	// We check for status NOT 'purged' already
	result := db.GetDB().Where("end_time < ? AND (status = ? OR status = ?)", cutoffTime, "completed", "failed").Find(&jobs)
	if result.Error != nil {
		utils.Logger.Error("Cleanup worker failed to query DB", zap.Error(result.Error))
		return
	}

	if len(jobs) == 0 {
		return // Nothing to clean
	}

	for _, job := range jobs {
		// Verify OutputDir exists before trying to delete
		if job.Config.OutputDir == "" {
			continue
		}

		if _, err := os.Stat(job.Config.OutputDir); os.IsNotExist(err) {
			// Already gone, just update status
			updateJobStatusMerged(&job)
			continue
		}

		// Delete Directory
		utils.Logger.Info("Purging old job output", zap.String("job_id", job.ID), zap.String("dir", job.Config.OutputDir))
		if err := os.RemoveAll(job.Config.OutputDir); err != nil {
			utils.Logger.Error("Failed to delete job dir", zap.String("job_id", job.ID), zap.Error(err))
			continue
		}

		updateJobStatusMerged(&job)
	}
}

func updateJobStatusMerged(job *Job) {
	job.Status = "purged"
	job.Error = "Output files purged due to retention policy"
	if err := db.GetDB().Save(job).Error; err != nil {
		fmt.Printf("Error updating job status: %v\n", err)
	}
}

// cleanupOldUploads deletes upload directories that are no longer referenced by
// any active job (queued/running/submitted) and are older than retentionDays.
func cleanupOldUploads(baseJobDir string, retentionDays int) {
	uploadsDir := filepath.Join(baseJobDir, "uploads")
	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.Logger.Error("Failed to read uploads dir", zap.Error(err))
		}
		return
	}

	if len(entries) == 0 {
		return
	}

	// Collect file paths referenced by jobs that are still active.
	// We never delete uploads while a job is queued, running, or submitted.
	var activeJobs []Job
	db.GetDB().Where("status IN ?", []string{"queued", "running", "submitted"}).Find(&activeJobs)

	activeUploadDirs := make(map[string]struct{})
	for _, job := range activeJobs {
		if job.Config == nil {
			continue
		}
		for _, path := range []string{job.Config.File1, job.Config.File2, job.Config.Reference, job.Config.GTF} {
			if path == "" {
				continue
			}
			// Mark the upload UUID dir that contains this file as active.
			// Upload paths look like: {baseJobDir}/uploads/{uuid}/{filename}
			rel, err := filepath.Rel(uploadsDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			if len(parts) > 0 && parts[0] != "" {
				activeUploadDirs[parts[0]] = struct{}{}
			}
		}
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip if this upload dir is still needed by an active job.
		if _, isActive := activeUploadDirs[entry.Name()]; isActive {
			continue
		}

		uploadPath := filepath.Join(uploadsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(cutoff) {
			continue // Not old enough yet
		}

		utils.Logger.Info("Purging old upload directory",
			zap.String("upload_id", entry.Name()),
			zap.String("path", uploadPath),
			zap.Time("modified", info.ModTime()),
		)

		if err := os.RemoveAll(uploadPath); err != nil {
			utils.Logger.Error("Failed to delete upload dir",
				zap.String("path", uploadPath),
				zap.Error(err),
			)
		}
	}
}
