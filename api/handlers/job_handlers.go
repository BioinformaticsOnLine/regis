package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BioinformaticsOnLine/regis/api/db"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/modules"
	"github.com/BioinformaticsOnLine/regis/utils"
	"github.com/BioinformaticsOnLine/regis/version"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"maragu.dev/goqite"
)

// BaseJobDir is the directory where jobs will be stored if no output dir is provided
var BaseJobDir = "./jobs"
var ServerStartTime time.Time

// Job represents a pipeline job
type Job struct {
	ID         string         `json:"id" gorm:"primaryKey"`
	JobName    string         `json:"job_name"`                      // User provided name
	UserEmail  string         `json:"user_email" gorm:"index"`       // Indexed email for filtering
	Status     string         `json:"status"`                        // queued, running, completed, failed
	Config     *config.Config `json:"config" gorm:"serializer:json"` // Store as JSON in DB
	StartTime  time.Time      `json:"start_time"`
	EndTime    time.Time      `json:"end_time"`
	ExternalID string         `json:"external_id,omitempty"` // Slurm Job ID
	Error      string         `json:"error,omitempty"`
}

// InitWorker starts the background worker that processes jobs from the persistent queue
func InitWorker() {
	// Ensure base job dir exists
	if err := os.MkdirAll(BaseJobDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create base job dir %s: %v\n", BaseJobDir, err)
	}

	go func() {
		ctx := context.Background()
		for {
			// Poll queue for new jobs
			msg, err := Queue.Receive(ctx)
			if err != nil {
				// Log error and wait a bit before retrying (backoff)
				fmt.Printf("Error receiving from queue: %v\n", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// If no message, wait and retry
			if msg == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			// Parse Job ID from message body
			jobID := string(msg.Body)

			// Fetch Job from DB
			var job Job
			if err := db.GetDB().First(&job, "id = ?", jobID).Error; err != nil {
				fmt.Printf("Error fetching job %s: %v. Deleting message.\n", jobID, err)
				// Delete 'poison' message so we don't loop forever
				_ = Queue.Delete(ctx, msg.ID)
				continue
			}

			// Process
			processJob(&job)

			// Delete message from queue after processing
			if err := Queue.Delete(ctx, msg.ID); err != nil {
				fmt.Printf("Failed to delete message %s: %v\n", msg.ID, err)
			}
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

// SubmitJob submits a new pipeline job
// @Summary Submit a new job
// @Description Submit a job with the provided configuration. Requires strictly validated JSON payload.
// @Tags jobs
// @Accept json
// @Produce json
// @Param job body config.Config true "Job Configuration"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /jobs/submit [post]
func SubmitJob(c *fiber.Ctx) error {
	// Parse request body into Config
	cfg := new(config.Config)
	if err := c.BodyParser(cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON body",
		})
	}

	// VALIDATE INPUT using go-playground/validator
	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		// Return friendly validation errors
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	jobID := uuid.New().String()

	// Handle Output Directory
	// If OutputDir is empty, create a new folder in BaseJobDir using the UUID
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(BaseJobDir, jobID)
		if absPath, err := filepath.Abs(cfg.OutputDir); err == nil {
			cfg.OutputDir = absPath
		}
	}

	// Apply Defaults (ValidationMode, etc.)
	cfg.EnsureDefaults()

	// Enforce Email (redundant check if validator works, but good for custom error)
	if cfg.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email is required for API job submission",
		})
	}

	// Validate configuration logic (legacy check)
	if err := utils.ValidateConfig(cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid configuration: %v", err),
		})
	}

	job := &Job{
		ID:        jobID,
		JobName:   cfg.JobName,
		UserEmail: cfg.Email,
		Status:    "queued",
		Config:    cfg,
	}

	// Save initial state to DB
	if result := db.GetDB().Create(job); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to save job to database: %v", result.Error),
		})
	}

	// Send to Persistent Queue
	err := Queue.Send(context.Background(), goqite.Message{
		Body: []byte(jobID),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to enqueue job: %v", err),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":     jobID,
		"status":     "queued",
		"message":    "Job submitted successfully",
		"output_dir": cfg.OutputDir,
	})
}

// ListJobs returns a list of jobs, optionally filtered by email
// @Summary List jobs
// @Description Get a list of jobs, optionally filtered by user email
// @Tags jobs
// @Produce json
// @Param email query string false "Filter by User Email"
// @Success 200 {array} Job
// @Router /jobs [get]
func ListJobs(c *fiber.Ctx) error {
	email := c.Query("email")

	var jobs []Job
	query := db.GetDB().Model(&Job{}).Order("start_time desc")

	if email != "" {
		query = query.Where("user_email = ?", email)
	}

	if result := query.Find(&jobs); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch jobs",
		})
	}

	return c.JSON(jobs)
}

// GetJobStatus returns the status of a specific job
// @Summary Get job status
// @Description Get the current status of a job by UUID
// @Tags jobs
// @Produce json
// @Param uuid path string true "Job UUID"
// @Success 200 {object} Job
// @Failure 404 {object} map[string]interface{}
// @Router /jobs/{uuid}/status [get]
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
// @Summary Get job results
// @Description Get the result summary/paths for a completed job
// @Tags jobs
// @Produce json
// @Param uuid path string true "Job UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /jobs/{uuid}/results [get]
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

// GetStats returns API statistics
// @Summary Get API statistics
// @Description Get system-wide statistics including total jobs, queued jobs, and last submission time
// @Tags stats
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /stats [get]
func GetStats(c *fiber.Ctx) error {
	var totalJobs int64
	var queuedJobs int64
	var runningJobs int64
	var completedJobs int64
	var failedJobs int64
	var lastJob Job

	// 1. Job Statistics
	db.GetDB().Model(&Job{}).Count(&totalJobs)
	db.GetDB().Model(&Job{}).Where("status = ?", "queued").Count(&queuedJobs)
	db.GetDB().Model(&Job{}).Where("status = ?", "running").Count(&runningJobs)
	db.GetDB().Model(&Job{}).Where("status = ?", "completed").Count(&completedJobs)
	db.GetDB().Model(&Job{}).Where("status = ?", "failed").Count(&failedJobs)

	// Latest job by start_time (empty table is normal — avoid First(), which logs ErrRecordNotFound)
	result := db.GetDB().Order("start_time desc").Limit(1).Find(&lastJob)

	// 2. System Statistics (Real Data)
	v, _ := mem.VirtualMemory()
	cStats, _ := cpu.Percent(0, false)
	cpuPercent := 0.0
	if len(cStats) > 0 {
		cpuPercent = cStats[0]
	}

	response := fiber.Map{
		"server": fiber.Map{
			"uptime_seconds": time.Since(ServerStartTime).Seconds(),
			"uptime_human":   time.Since(ServerStartTime).String(),
			"start_time":     ServerStartTime.Format(time.RFC3339),
			"version":        version.Version,
		},
		"jobs": fiber.Map{
			"total":     totalJobs,
			"queued":    queuedJobs,
			"running":   runningJobs,
			"completed": completedJobs,
			"failed":    failedJobs,
		},
		"system": fiber.Map{
			"cpus":            runtime.NumCPU(),
			"cpu_percent":     cpuPercent,
			"memory_used_mb":  v.Used / 1024 / 1024,
			"memory_total_mb": v.Total / 1024 / 1024,
			"memory_percent":  v.UsedPercent,
		},
		"last_job_submitted": nil,
	}

	if result.RowsAffected > 0 && !lastJob.StartTime.IsZero() {
		response["last_job_submitted"] = lastJob.StartTime.Format(time.RFC3339)
	}

	return c.JSON(response)
}
