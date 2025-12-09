package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Config holds all pipeline configuration
type Config struct {
	// Run identification
	RunID string `json:"run_id"` // Unique identifier for this pipeline run (UUID)
	Email string `json:"email"`  // User email (required for tracking/notification)

	// Required parameters
	DataType  string `json:"data_type"` // "single" or "paired"
	Method    string `json:"method"`    // "denovo" or "reference"
	OutputDir string `json:"output_dir"`
	File1     string `json:"file1"` // First FASTQ file
	File2     string `json:"file2"` // Second FASTQ file (for paired-end)

	// Reference mode parameters
	Reference string `json:"reference"` // Reference genome FASTA
	GTF       string `json:"gtf"`       // Reference annotation GTF

	// Optional parameters
	Threads int    `json:"threads"` // Number of threads (default: all available)
	Species string `json:"species"` // Species name for CPAT

	// Validation options
	SkipCPAT  bool   `json:"skip_cpat"`  // Skip CPAT validation (CPC2-only mode)
	CPATHex   string `json:"cpat_hex"`   // Custom CPAT hexamer file
	CPATLogit string `json:"cpat_logit"` // Custom CPAT logit model

	// Optional: Advanced Features
	EnableLncTar         bool `json:"enable_lnctar" yaml:"enable_lnctar"`
	LncTarBestOnly       bool `json:"lnctar_best_only" yaml:"lnctar_best_only"`
	LncTarComprehensive  bool `json:"lnctar_comprehensive" yaml:"lnctar_comprehensive"`
	EnableIntaRNA        bool `json:"enable_intarna" yaml:"enable_intarna"`
	IntaRNABestOnly      bool `json:"intarna_best_only" yaml:"intarna_best_only"`
	IntaRNAComprehensive bool `json:"intarna_comprehensive" yaml:"intarna_comprehensive"`

	// rRNA Filtering
	EnableSortMeRNA bool `json:"enable_sortmerna" yaml:"enable_sortmerna"` // Flag: --sortmerna

	// Asset paths (bundled models and tools)
	AssetsDir     string `json:"assets_dir"`      // Default: ./assets
	CPATModelsDir string `json:"cpat_models_dir"` // Default: ./assets/models
	LncTarDir     string `json:"lnctar_dir"`      // Default: ./assets/lnctar
	LncTarScript  string `json:"lnctar_script"`   // Default: ./assets/lnctar/LncTar.pl

	// Runtime state (set by validation)
	CPATSpecies     string `json:"-"` // Normalized species name for CPAT
	ValidationMode  string `json:"-"` // "consensus" or "cpc2-only"
	CPATHexamerFile string `json:"-"` // Path to CPAT hexamer model
	CPATLogitFile   string `json:"-"` // Path to CPAT logit model

	// Execution Configuration
	ExecutionMode string       `json:"execution_mode"` // "local" (default) or "slurm"
	Slurm         *SlurmConfig `json:"slurm,omitempty"`

	LogFile string `json:"-"` // Path to log file
}

// SlurmConfig holds Slurm-specific parameters
type SlurmConfig struct {
	Partition   string   `json:"partition"`    // e.g., "compute"
	JobName     string   `json:"job_name"`     // Default: regis_job
	Time        string   `json:"time"`         // e.g., "24:00:00"
	Memory      string   `json:"memory"`       // e.g., "8G"
	CPUs        int      `json:"cpus"`         // e.g., 8
	Nodes       int      `json:"nodes"`        // Default: 1
	Email       string   `json:"email"`        // For notifications
	ExtraArgs   string   `json:"extra_args"`   // To add custom flags
	ExtraScript []string `json:"extra_script"` // Custom bash commands (preamble)
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		RunID:   uuid.New().String(), // Generate unique run ID
		Threads: 0,                   // 0 means use all available
	}
}

// SetOutputDir sets the output directory, auto-generating one if not specified
func (c *Config) SetOutputDir(outputDir string) {
	if outputDir == "" {
		// Auto-generate output directory with timestamp and UUID
		timestamp := time.Now().Format("20060102_150405")
		shortID := c.RunID[:8] // Use first 8 chars of UUID
		c.OutputDir = filepath.Join(".", fmt.Sprintf("regis_out_%s_%s", timestamp, shortID))
	} else {
		c.OutputDir = outputDir
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validation logic will be implemented in utils/validation.go
	return nil
}
