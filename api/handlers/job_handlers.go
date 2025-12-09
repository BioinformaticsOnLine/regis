package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/modules"
	"github.com/BioinformaticsOnLine/regis/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// BaseJobDir is the directory where jobs will be stored if no output dir is provided
var BaseJobDir = "./jobs"

// Job represents a pipeline job
type Job struct {
	ID         string         `json:"id" gorm:"primaryKey"`
	Status     string         `json:"status"`                        // queued, running, completed, failed
	Config     *config.Config `json:"config" gorm:"serializer:json"` // Store as JSON in DB
	StartTime  time.Time      `json:"start_time"`
	EndTime    time.Time      `json:"end_time"`
	ExternalID string         `json:"external_id,omitempty"` // Slurm Job ID
	Error      string         `json:"error,omitempty"`
}

var (
	// JobQueue is still used for in-memory worker processing of new submissions
	// In a more robust system, we might poll the DB for "queued" jobs on startup
	JobQueue = make(chan *Job, 100)
)

// InitWorker starts the background worker that processes jobs
func InitWorker() {
	// Ensure base job dir exists
	if err := os.MkdirAll(BaseJobDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create base job dir %s: %v\n", BaseJobDir, err)
	}

	go func() {
		for job := range JobQueue {
			processJob(job)
		}
	}()
}

func processJob(job *Job) {
	// Setup output directory
	// Note: utils.Logger is global, so only one job can run at a time safely (in local mode)
	if err := os.MkdirAll(job.Config.OutputDir, 0755); err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("Failed to create output dir: %v", err)
		db.GetDB().Save(job)
		return
	}

	// Update DB (Start processing)
	job.StartTime = time.Now()
	db.GetDB().Save(job)

	// Check Execution Mode
	if job.Config.ExecutionMode == "slurm" {
		handleSlurmSubmission(job)
		return
	}

	// LOCAL EXECUTION
	job.Status = "running"
	db.GetDB().Save(job)

	// Initialize logger for this job
	logFile := filepath.Join(job.Config.OutputDir, "pipeline.log")
	if err := utils.InitLogger(logFile); err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("Failed to init logger: %v", err)
		db.GetDB().Save(job)
		return
	}
	defer utils.Sync()

	// Run Pipeline
	runner := modules.NewPipelineRunner(job.Config)
	ctx := context.Background() // Could be cancellable

	err := runner.RunHeadless(ctx)

	job.EndTime = time.Now()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
	} else {
		job.Status = "completed"
	}

	// Final DB Update
	db.GetDB().Save(job)
}

func handleSlurmSubmission(job *Job) {
	// 1. Serialize Config to JSON file (so the job can read it)
	configPath := filepath.Join(job.Config.OutputDir, "job_config.json")
	if err := utils.WriteConfigJSON(job.Config, configPath); err != nil {
		failJob(job, fmt.Sprintf("Failed to write config: %v", err))
		return
	}

	// 2. Generate Sbatch Script
	scriptPath, err := modules.GenerateSbatchScript(job.Config)
	if err != nil {
		failJob(job, fmt.Sprintf("Failed to generate sbatch: %v", err))
		return
	}

	// 3. Submit to Slurm
	slurmID, err := modules.SubmitSbatch(scriptPath)
	if err != nil {
		failJob(job, fmt.Sprintf("Failed to submit to Slurm: %v", err))
		return
	}

	// 4. Update Job Status
	job.Status = "submitted"
	job.ExternalID = slurmID
	db.GetDB().Save(job)

	fmt.Printf("Job %s submitted to Slurm as %s\n", job.ID, slurmID)
}

func failJob(job *Job, msg string) {
	job.Status = "failed"
	job.Error = msg
	job.EndTime = time.Now()
	db.GetDB().Save(job)
}

// SubmitJob handles new job submissions
func SubmitJob(c *fiber.Ctx) error {
	// Parse request body into Config
	cfg := new(config.Config)
	if err := c.BodyParser(cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON body",
		})
	}

	jobID := uuid.New().String()

	// Handle Output Directory
	// If OutputDir is empty, create a new folder in BaseJobDir using the UUID
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(BaseJobDir, jobID)
		// Ensure absolute path for safety?
		// For now simple relative path is fine, but absolute is safer for tools
		if absPath, err := filepath.Abs(cfg.OutputDir); err == nil {
			cfg.OutputDir = absPath
		}
	}

	// Enforce Email for API submissions (Security Policy)
	if cfg.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email is required for API job submission",
		})
	}

	// Validate configuration
	if err := utils.ValidateConfig(cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid configuration: %v", err),
		})
	}

	job := &Job{
		ID:     jobID,
		Status: "queued",
		Config: cfg,
	}

	// Save initial state to DB
	if result := db.GetDB().Create(job); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to save job to database: %v", result.Error),
		})
	}

	// Send to worker
	JobQueue <- job

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":     jobID,
		"status":     "queued",
		"message":    "Job submitted successfully",
		"output_dir": cfg.OutputDir,
	})
}

// GetJobStatus returns the status of a specific job
func GetJobStatus(c *fiber.Ctx) error {
	jobID := c.Params("uuid")

	var job Job
	if result := db.GetDB().First(&job, "id = ?", jobID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	return c.JSON(job)
}

// GetJobResults returns the results summary for a job
func GetJobResults(c *fiber.Ctx) error {
	jobID := c.Params("uuid")

	var job Job
	if result := db.GetDB().First(&job, "id = ?", jobID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	if job.Status != "completed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "Job not completed yet",
			"status": job.Status,
		})
	}

	return c.JSON(fiber.Map{
		"job_id":     jobID,
		"output_dir": job.Config.OutputDir,
		"status":     "completed",
	})
}
