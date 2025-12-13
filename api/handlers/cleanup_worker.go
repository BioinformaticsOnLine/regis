package handlers

import (
	"fmt"
	"os"
	"time"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// StartCleanupWorker starts a background routine to purge old job files
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
