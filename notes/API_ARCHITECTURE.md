# REGIS API Architecture Guide (v1.0.5)

This document provides an in-depth explanation of the REST API server architecture, database design, authentication, job queue system, and Slurm integration.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Server Startup Flow](#server-startup-flow)
3. [Database Layer](#database-layer)
4. [Authentication & API Key](#authentication--api-key)
5. [Job Queue System](#job-queue-system)
6. [Job Lifecycle](#job-lifecycle)
7. [Route Definitions](#route-definitions)
8. [Slurm Integration](#slurm-integration)
9. [Configuration System](#configuration-system)
10. [Cleanup Worker](#cleanup-worker)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        REGIS API Server                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐          │
│   │   Fiber     │────▶│  Handlers   │────▶│  Database   │          │
│   │  (HTTP)     │     │             │     │  (SQLite)   │          │
│   └─────────────┘     └─────────────┘     └─────────────┘          │
│          │                   │                   │                  │
│          │                   ▼                   │                  │
│          │            ┌─────────────┐            │                  │
│          │            │   goqite    │◀───────────┘                  │
│          │            │   (Queue)   │                               │
│          │            └─────────────┘                               │
│          │                   │                                      │
│          ▼                   ▼                                      │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐          │
│   │  Swagger    │     │   Worker    │────▶│  Pipeline   │          │
│   │   Docs      │     │  (Background)│     │  Runner     │          │
│   └─────────────┘     └─────────────┘     └─────────────┘          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | File | Description |
|-----------|------|-------------|
| HTTP Server | `api/server.go` | Fiber-based HTTP server |
| Routes | `api/routes.go` | API endpoint definitions |
| Job Handlers | `api/handlers/job_handlers.go` | Job CRUD operations |
| Result Handlers | `api/handlers/result_handlers.go` | Results & downloads |
| Queue | `api/handlers/queue.go` | Persistent job queue |
| Database | `api/db/client.go` | GORM SQLite connection |
| Cleanup | `api/handlers/cleanup_worker.go` | Job retention cleanup |

---

## 2. Server Startup Flow

### Entry Point: `main.go`

```go
// When user runs: regis serve --port 3000 --job-dir ./jobs
case "serve":
    serveCmd := pflag.NewFlagSet("serve", pflag.ExitOnError)
    port := serveCmd.StringP("port", "p", "3000", "Port to run the server on")
    jobDir := serveCmd.String("job-dir", "./jobs", "Directory to store job outputs")
    serveCmd.Parse(os.Args[2:])
    api.StartServer(*port, *jobDir)
```

### Server Initialization: `api/server.go`

```go
func StartServer(port, jobDir string) {
    // 1. Initialize Database
    db.Init("regis.db")
    
    // 2. Auto Migrate Job Table
    db.GetDB().AutoMigrate(&handlers.Job{})
    
    // 3. Configure Base Job Directory
    handlers.BaseJobDir = jobDir
    
    // 4. Initialize Logger
    utils.InitLogger(filepath.Join(jobDir, "server.log"))
    
    // 5. Load/Generate Configuration (API Key, etc.)
    cfg := config.NewConfig()
    cfg.EnsureDefaults()  // Generates API Key if missing
    
    // 6. Create Fiber Server
    server := NewServer(cfg)
    
    // 7. Initialize Persistent Queue
    sqlDB, _ := db.GetSqlDB()
    handlers.InitQueue(sqlDB)
    
    // 8. Start Background Worker
    handlers.InitWorker()
    
    // 9. Start Cleanup Worker
    handlers.StartCleanupWorker(cfg)
    
    // 10. Print Startup Banner
    fmt.Printf("REGIS Pipeline API Server\n")
    fmt.Printf("Port: %s\n", port)
    fmt.Printf("API Key: %s\n", cfg.APIKey)
    
    // 11. Start Listening
    server.Listen(":" + port)
}
```

---

## 3. Database Layer

### File: `api/db/client.go`

### Database: SQLite

REGIS uses **SQLite** for simplicity and zero-configuration. The database file is created automatically at startup.

```go
var DB *gorm.DB

func Init(dbName string) {
    // Create directory if needed
    dir := filepath.Dir(dbName)
    os.MkdirAll(dir, 0755)
    
    // Open GORM connection
    DB, err = gorm.Open(sqlite.Open(dbName), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Warn),
    })
}

func GetDB() *gorm.DB {
    return DB
}

func GetSqlDB() (*sql.DB, error) {
    return DB.DB()
}
```

### Database File Location

```
regis.db           # Created in current working directory
```

### Job Table Schema

```sql
CREATE TABLE jobs (
    id          TEXT PRIMARY KEY,       -- UUID
    status      TEXT,                   -- queued|running|completed|failed|submitted
    config      TEXT,                   -- JSON serialized Config
    start_time  DATETIME,
    end_time    DATETIME,
    external_id TEXT,                   -- Slurm Job ID (if applicable)
    error       TEXT                    -- Error message (if failed)
);
```

### Job Model: `api/handlers/job_handlers.go`

```go
type Job struct {
    ID         string         `json:"id" gorm:"primaryKey"`
    Status     string         `json:"status"`                        
    Config     *config.Config `json:"config" gorm:"serializer:json"` 
    StartTime  time.Time      `json:"start_time"`
    EndTime    time.Time      `json:"end_time"`
    ExternalID string         `json:"external_id,omitempty"` 
    Error      string         `json:"error,omitempty"`
}
```

### GORM Serializer

The `Config` field uses `gorm:"serializer:json"` to automatically serialize/deserialize the entire configuration object to/from a JSON string in the database.

---

## 4. Authentication & API Key

### API Key Generation

If no API key is provided, one is **auto-generated** on server startup:

```go
// config/config.go
func (c *Config) EnsureDefaults() {
    if c.APIKey == "" {
        c.APIKey = uuid.New().String()
    }
}
```

### API Key Display

The API key is printed to the console on startup:

```
REGIS Pipeline API Server
Version: 1.0.5
──────────────────────────────────────────────────
 • Port             : 3000
 • Job Directory    : ./jobs
 • API Key          : 550e8400-e29b-41d4-a716-446655440000  ← Here
 • Execution Mode   : local
 • Retention Policy : 7 days
 • Documentation    : http://localhost:3000/swagger/index.html
──────────────────────────────────────────────────
```

### API Key Middleware: `api/server.go`

```go
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
```

### Protected Routes

The middleware is applied to all `/api/v1/jobs/*` routes:

```go
// api/routes.go
jobs := v1.Group("/jobs", s.APIKeyMiddleware)  // ← Middleware applied here
jobs.Post("/submit", handlers.SubmitJob)
jobs.Get("/:uuid/status", handlers.GetJobStatus)
jobs.Get("/:uuid/results", handlers.GetJobResults)
jobs.Get("/:uuid/results/metrics", handlers.GetJobMetrics)
jobs.Get("/:uuid/results/download", handlers.DownloadJobResults)
```

### Providing API Key (Frontend)

**Option 1: Header (Recommended)**
```http
GET /api/v1/jobs/123/status
X-API-Key: 550e8400-e29b-41d4-a716-446655440000
```

**Option 2: Query Parameter**
```http
GET /api/v1/jobs/123/status?api_key=550e8400-e29b-41d4-a716-446655440000
```

### Setting Custom API Key

You can set a custom API key via:

1. **Environment Variable:**
   ```bash
   export REGIS_API_KEY=my-secret-key
   regis serve --port 3000
   ```

2. **Config File (config.yaml):**
   ```yaml
   api_key: my-secret-key
   ```

---

## 5. Job Queue System

### File: `api/handlers/queue.go`

REGIS uses **goqite** - a persistent, SQLite-backed job queue.

### Queue Initialization

```go
var Queue *goqite.Queue

func InitQueue(db *sql.DB) {
    // 1. Setup Table (creates if not exists)
    goqite.Setup(context.Background(), db)
    
    // 2. Initialize Queue with name
    Queue = goqite.New(goqite.NewOpts{
        DB:   db,
        Name: "regis_jobs",
    })
}
```

### Queue Table Schema (Auto-created)

```sql
CREATE TABLE goqite (
    id        TEXT PRIMARY KEY,
    queue     TEXT,
    body      BLOB,
    timeout   DATETIME,
    received  INTEGER,
    created   DATETIME
);
```

### Why goqite?

- **Persistent**: Jobs survive server restarts
- **SQLite-backed**: Uses same database as jobs
- **Simple**: No external dependencies (Redis, RabbitMQ, etc.)
- **Reliable**: At-least-once delivery

---

## 6. Job Lifecycle

### State Diagram

```
                    ┌─────────────┐
                    │   Submit    │
                    │   Request   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │   queued    │ ← Initial state
                    └──────┬──────┘
                           │ Worker picks up
                           ▼
              ┌────────────┴────────────┐
              │                         │
              ▼                         ▼
       ┌─────────────┐          ┌─────────────┐
       │   running   │          │  submitted  │ (Slurm only)
       └──────┬──────┘          └──────┬──────┘
              │                        │
              ▼                        │ (Slurm job finishes)
       ┌──────┴──────┐                 │
       │             │                 │
       ▼             ▼                 │
┌─────────────┐ ┌─────────────┐       │
│  completed  │ │   failed    │◀──────┘
└─────────────┘ └─────────────┘
```

### Job Submission Flow

```go
// api/handlers/job_handlers.go
func SubmitJob(c *fiber.Ctx) error {
    // 1. Parse JSON body
    cfg := new(config.Config)
    c.BodyParser(cfg)
    
    // 2. Validate with go-playground/validator
    validate := validator.New()
    validate.Struct(cfg)
    
    // 3. Generate UUID
    jobID := uuid.New().String()
    
    // 4. Set output directory (if empty)
    if cfg.OutputDir == "" {
        cfg.OutputDir = filepath.Join(BaseJobDir, jobID)
    }
    
    // 5. Apply defaults
    cfg.EnsureDefaults()
    
    // 6. Validate configuration logic
    utils.ValidateConfig(cfg)
    
    // 7. Create Job object
    job := &Job{
        ID:     jobID,
        Status: "queued",
        Config: cfg,
    }
    
    // 8. Save to database
    db.GetDB().Create(job)
    
    // 9. Send to queue
    Queue.Send(context.Background(), goqite.Message{
        Body: []byte(jobID),
    })
    
    // 10. Return response
    return c.Status(202).JSON(fiber.Map{
        "job_id":     jobID,
        "status":     "queued",
        "message":    "Job submitted successfully",
        "output_dir": cfg.OutputDir,
    })
}
```

### Background Worker

```go
// api/handlers/job_handlers.go
func InitWorker() {
    go func() {
        for {
            // 1. Poll queue for messages
            msg, err := Queue.Receive(ctx)
            if msg == nil {
                time.Sleep(1 * time.Second)
                continue
            }
            
            // 2. Parse job ID
            jobID := string(msg.Body)
            
            // 3. Fetch job from database
            var job Job
            db.GetDB().First(&job, "id = ?", jobID)
            
            // 4. Process job
            processJob(&job)
            
            // 5. Delete message from queue
            Queue.Delete(ctx, msg.ID)
        }
    }()
}
```

### Job Processing

```go
func processJob(job *Job) {
    // 1. Create output directory
    os.MkdirAll(job.Config.OutputDir, 0755)
    
    // 2. Update start time
    job.StartTime = time.Now()
    db.GetDB().Save(job)
    
    // 3. Check execution mode
    if job.Config.ExecutionMode == "slurm" {
        handleSlurmSubmission(job)
        return
    }
    
    // LOCAL EXECUTION
    // 4. Update status to running
    job.Status = "running"
    db.GetDB().Save(job)
    
    // 5. Initialize logger
    utils.InitLogger(filepath.Join(job.Config.OutputDir, "pipeline.log"))
    
    // 6. Run pipeline
    runner := modules.NewPipelineRunner(job.Config)
    err := runner.RunHeadless(ctx)
    
    // 7. Update final status
    job.EndTime = time.Now()
    if err != nil {
        job.Status = "failed"
        job.Error = err.Error()
    } else {
        job.Status = "completed"
    }
    db.GetDB().Save(job)
}
```

---

## 7. Route Definitions

### File: `api/routes.go`

```go
func (s *Server) SetupRoutes() {
    api := s.App.Group("/api")
    v1 := api.Group("/v1")
    
    // Health Check (No Auth)
    v1.Get("/health", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{
            "status":  "ok",
            "message": "Regis API is running",
        })
    })
    
    // Job Routes (Auth Required)
    jobs := v1.Group("/jobs", s.APIKeyMiddleware)
    jobs.Post("/submit", handlers.SubmitJob)
    jobs.Get("/:uuid/status", handlers.GetJobStatus)
    jobs.Get("/:uuid/results", handlers.GetJobResults)
    jobs.Get("/:uuid/results/metrics", handlers.GetJobMetrics)
    jobs.Get("/:uuid/results/download", handlers.DownloadJobResults)
    
    // 404 Handler
    s.App.Use(func(c *fiber.Ctx) error {
        return c.Status(404).JSON(fiber.Map{
            "error": "Endpoint not found",
        })
    })
}
```

### Swagger Documentation

Swagger UI is available at:
```
http://localhost:3000/swagger/index.html
```

Generated from annotations in handler functions:
```go
// @Summary Submit a new job
// @Description Submit a job with the provided configuration
// @Tags jobs
// @Accept json
// @Produce json
// @Param job body config.Config true "Job Configuration"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /jobs/submit [post]
func SubmitJob(c *fiber.Ctx) error { ... }
```

---

## 8. Slurm Integration

### File: `modules/slurm.go`

When `execution_mode: "slurm"`, REGIS generates a sbatch script and submits to the HPC scheduler.

### Slurm Submission Flow

```go
func handleSlurmSubmission(job *Job) {
    // 1. Save config to JSON file
    configPath := filepath.Join(job.Config.OutputDir, "job_config.json")
    utils.WriteConfigJSON(job.Config, configPath)
    
    // 2. Generate sbatch script
    scriptPath := modules.GenerateSbatchScript(job.Config)
    
    // 3. Submit to Slurm
    slurmID := modules.SubmitSbatch(scriptPath)
    
    // 4. Update job status
    job.Status = "submitted"
    job.ExternalID = slurmID
    db.GetDB().Save(job)
}
```

### Generated sbatch Script

```bash
#!/bin/bash
#SBATCH --job-name=regis_pipeline
#SBATCH --partition=compute
#SBATCH --nodes=1
#SBATCH --ntasks-per-node=40
#SBATCH --mem=120G
#SBATCH --time=24:00:00
#SBATCH --output=%x_%j.out
#SBATCH --error=%x_%j.err

# User-specified extra script lines
export PATH="/path/to/tools:$PATH"
eval "$(conda shell.bash hook)"
conda activate regis

# Run pipeline in headless mode
regis submit_internal --config /path/to/job_config.json
```

### Internal Submit Command

The pipeline runs via an internal subcommand:

```go
// main.go
case "submit_internal":
    submitCmd := pflag.NewFlagSet("submit_internal", pflag.ExitOnError)
    configPath := submitCmd.String("config", "", "Path to job configuration JSON")
    submitCmd.Parse(os.Args[2:])
    runInternalJob(*configPath)
```

### Slurm Configuration Options

```go
type SlurmConfig struct {
    Partition   string   `json:"partition"`     // compute, gpu, etc.
    JobName     string   `json:"job_name"`      // Job identifier
    Time        string   `json:"time"`          // 24:00:00
    Memory      string   `json:"memory"`        // 120G
    CPUs        int      `json:"cpus"`          // 40
    Nodes       int      `json:"nodes"`         // 1
    Email       string   `json:"email"`         // For notifications
    ExtraArgs   string   `json:"extra_args"`    // --exclusive, etc.
    ExtraScript []string `json:"extra_script"`  // Bash lines to prepend
}
```

---

## 9. Configuration System

### File: `config/config.go`

REGIS uses **koanf** for layered configuration:

```
Priority (highest to lowest):
1. CLI Flags
2. Environment Variables (REGIS_*)
3. Config File (YAML/JSON)
4. Defaults
```

### Configuration Loading

```go
func Load(f *pflag.FlagSet, configFile string) (*Config, error) {
    // 1. Load Defaults
    k.Load(structs.Provider(NewConfig(), "mapstructure"), nil)
    
    // 2. Load Config File
    if configFile != "" {
        k.Load(file.Provider(configFile), yaml.Parser())
    }
    
    // 3. Load Environment Variables
    k.Load(env.Provider("REGIS_", ".", func(s string) string {
        return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "REGIS_")), "_", ".", -1)
    }), nil)
    
    // 4. Load CLI Flags
    flagMap := make(map[string]interface{})
    f.Visit(func(flag *pflag.Flag) {
        key := mapping[flag.Name]
        flagMap[key] = flag.Value.String()
    })
    k.Load(confmap.Provider(flagMap, "."), nil)
    
    // 5. Unmarshal
    var cfg Config
    k.Unmarshal("", &cfg)
    
    return &cfg, nil
}
```

### Environment Variable Examples

```bash
export REGIS_API_KEY=my-secret-key
export REGIS_DATA_TYPE=paired
export REGIS_METHOD=reference
export REGIS_THREADS=16
```

---

## 10. Cleanup Worker

### File: `api/handlers/cleanup_worker.go`

Automatically deletes old job outputs based on retention policy.

### Default Retention: 7 Days

```go
// config/config.go
func (c *Config) EnsureDefaults() {
    if c.RetentionDays == 0 {
        c.RetentionDays = 7
    }
}
```

### Cleanup Logic

```go
func StartCleanupWorker(cfg *config.Config) {
    go func() {
        for {
            // Run once per day
            time.Sleep(24 * time.Hour)
            
            // Find jobs older than retention period
            cutoff := time.Now().AddDate(0, 0, -cfg.RetentionDays)
            
            var oldJobs []Job
            db.GetDB().Where("end_time < ? AND status = ?", cutoff, "completed").Find(&oldJobs)
            
            for _, job := range oldJobs {
                // Delete output directory
                os.RemoveAll(job.Config.OutputDir)
                
                // Delete from database
                db.GetDB().Delete(&job)
            }
        }
    }()
}
```

### Disabling Cleanup

Set `retention_days: 0` to keep jobs forever:

```yaml
# config.yaml
retention_days: 0
```

---

## Summary

| Component | Technology | Location |
|-----------|-----------|----------|
| HTTP Server | Fiber | `api/server.go` |
| Database | GORM + SQLite | `api/db/client.go` |
| Queue | goqite | `api/handlers/queue.go` |
| Auth | API Key Middleware | `api/server.go` |
| Validation | go-playground/validator | `api/handlers/job_handlers.go` |
| Docs | Swagger | `docs/docs.go` |
| Slurm | sbatch generation | `modules/slurm.go` |

---

*Last Updated: December 2025 | REGIS v1.0.5*
