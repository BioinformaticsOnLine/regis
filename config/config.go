package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/confmap" // Added for manual flag mapping
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// Global koanf instance
var k = koanf.New(".")

// Config holds all pipeline configuration
type Config struct {
	// Run identification
	RunID string `json:"run_id" mapstructure:"run_id"` // Unique identifier (UUID)
	Email string `json:"email" mapstructure:"email" validate:"required,email"`   // User email

	// Required parameters
	DataType  string `json:"data_type" mapstructure:"data_type" validate:"required,oneof=single paired"`   // "single" or "paired"
	Method    string `json:"method" mapstructure:"method" validate:"required,oneof=denovo reference"`         // "denovo" or "reference"
	OutputDir string `json:"output_dir" mapstructure:"output_dir"` // Output directory
	File1     string `json:"file1" mapstructure:"file1" validate:"required"`           // First FASTQ file
	File2     string `json:"file2" mapstructure:"file2"`           // Second FASTQ file

	// Reference mode parameters
	Reference string `json:"reference" mapstructure:"reference"` // Reference genome FASTA
	GTF       string `json:"gtf" mapstructure:"gtf"`             // Reference annotation GTF

	// Optional parameters
	Threads int    `json:"threads" mapstructure:"threads"` // Number of threads
	Species string `json:"species" mapstructure:"species"` // Species name for CPAT

	// Validation options
	SkipCPAT  bool   `json:"skip_cpat" mapstructure:"skip_cpat"`   // CPC2-only mode
	CPATHex   string `json:"cpat_hex" mapstructure:"cpat_hex"`     // Custom CPAT hexamer model
	CPATLogit string `json:"cpat_logit" mapstructure:"cpat_logit"` // Custom CPAT logit model

	// LncTar settings
	EnableLncTar        bool `json:"enable_lnctar" mapstructure:"enable_lnctar"`
	LncTarBestOnly      bool `json:"lnctar_best_only" mapstructure:"lnctar_best_only"`
	LncTarComprehensive bool `json:"lnctar_comprehensive" mapstructure:"lnctar_comprehensive"`
	LncTarHighly        bool `json:"lnctar_highly" mapstructure:"lnctar_highly"`

	// IntaRNA settings
	EnableIntaRNA        bool `json:"enable_intarna" mapstructure:"enable_intarna"`
	IntaRNABestOnly      bool `json:"intarna_best_only" mapstructure:"intarna_best_only"`
	IntaRNAComprehensive bool `json:"intarna_comprehensive" mapstructure:"intarna_comprehensive"`
	IntaRNAHighly        bool `json:"intarna_highly" mapstructure:"intarna_highly"`

	// Security
	APIKey        string `json:"api_key" mapstructure:"api_key"`               // Static API Key for authentication
	RetentionDays int    `json:"retention_days" mapstructure:"retention_days"` // Days to keep job outputs (0 = forever)

	// rRNA Filtering
	EnableSortMeRNA bool `json:"enable_sortmerna" mapstructure:"enable_sortmerna"`

	// Asset paths
	AssetsDir     string `json:"assets_dir" mapstructure:"assets_dir"`
	CPATModelsDir string `json:"cpat_models_dir" mapstructure:"cpat_models_dir"`
	LncTarDir     string `json:"lnctar_dir" mapstructure:"lnctar_dir"`
	LncTarScript  string `json:"lnctar_script" mapstructure:"lnctar_script"`

	// Runtime state (ignored by config loader)
	CPATSpecies     string `json:"cpat_species" mapstructure:"-"`
	ValidationMode  string `json:"validation_mode" mapstructure:"-"`
	CPATHexamerFile string `json:"cpat_hexamer_file" mapstructure:"-"`
	CPATLogitFile   string `json:"cpat_logit_file" mapstructure:"-"`
	LogFile         string `json:"log_file" mapstructure:"-"`

	// Execution Configuration
	ExecutionMode string       `json:"execution_mode" mapstructure:"execution_mode"`
	Slurm         *SlurmConfig `json:"slurm,omitempty" mapstructure:"slurm"`
}

// SlurmConfig holds Slurm-specific parameters
type SlurmConfig struct {
	Partition   string   `json:"partition" mapstructure:"partition"`
	JobName     string   `json:"job_name" mapstructure:"job_name"`
	Time        string   `json:"time" mapstructure:"time"`
	Memory      string   `json:"memory" mapstructure:"memory"`
	CPUs        int      `json:"cpus" mapstructure:"cpus"`
	Nodes       int      `json:"nodes" mapstructure:"nodes"`
	Email       string   `json:"email" mapstructure:"email"`
	ExtraArgs   string   `json:"extra_args" mapstructure:"extra_args"`
	ExtraScript []string `json:"extra_script" mapstructure:"extra_script"`
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		RunID:         uuid.New().String(),
		Threads:       0,
		ExecutionMode: "local",
		AssetsDir:     "./assets",
		Slurm: &SlurmConfig{
			Partition: "compute",
			Nodes:     1,
			CPUs:      8,
			Memory:    "16G",
			Time:      "24:00:00",
		},
	}
}

// Load populates the configuration from Defaults -> Config File -> Env -> Flags
func Load(f *pflag.FlagSet, configFile string) (*Config, error) {
	// 1. Load Defaults
	if err := k.Load(structs.Provider(NewConfig(), "mapstructure"), nil); err != nil {
		return nil, fmt.Errorf("error loading defaults: %v", err)
	}

	// 2. Load Config File (if specified)
	if configFile != "" {
		if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("error loading config file: %v", err)
		}
	}

	// 3. Load Environment Variables (e.g. REGIS_DATA_TYPE)
	if err := k.Load(env.Provider("REGIS_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "REGIS_")), "_", ".", -1)
	}), nil); err != nil {
		return nil, fmt.Errorf("error loading env vars: %v", err)
	}

	// 4. Load Flags MANUALLY to ensure mapping works correctly
	// The native posflag provider in this version has limitation with key mapping
	if f != nil {
		flagMap := make(map[string]interface{})
		
		// Map flag names to Struct Field Names (PascalCase) to avoid issue with underscore/tags
		mapping := map[string]string{
			"f1":                   "File1",
			"f2":                   "File2",
			"data_type":            "DataType",
			"output_dir":           "OutputDir",
			"method":               "Method",
			"reference":            "Reference",
			"gtf":                  "GTF",
			"lnctar_best":          "LncTarBestOnly",
			"lnctar_highly":        "LncTarHighly",
			"lnctar_comprehensive": "LncTarComprehensive",
			"lnctar_all":           "LncTarComprehensive", // alias
			"intarna_best":         "IntaRNABestOnly",
			"intarna_highly":       "IntaRNAHighly",
			"intarna_comprehensive": "IntaRNAComprehensive",
			"intarna_all":           "IntaRNAComprehensive", // alias
			"species":              "Species",
			"skip_cpat":            "SkipCPAT",
			"sortmerna":            "EnableSortMeRNA",
			"threads":              "Threads",
			"email":                "Email",
		}

		// Only load flags that were explicitly set by the user
		f.Visit(func(flag *pflag.Flag) {
			key := flag.Name
			if mapped, ok := mapping[key]; ok {
				key = mapped
			}
			// DEBUG: Check what's being loaded
			// fmt.Printf("DEBUG FLAG VISIT: Name='%s' MappedKey='%s' Value='%s'\n", flag.Name, key, flag.Value.String())

			// Use the string value; koanf will handle type conversion if schema matches
			flagMap[key] = flag.Value.String()
		})

		if err := k.Load(confmap.Provider(flagMap, "."), nil); err != nil {
			return nil, fmt.Errorf("error loading flags: %v", err)
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %v", err)
	}

	// Post-process: ensure RunID is set (if overwritten by empty)
	if cfg.RunID == "" {
		cfg.RunID = uuid.New().String()
	}

	// Handle derived paths if necessary (e.g. set output dir default if empty)
	if cfg.OutputDir == "" {
		timestamp := time.Now().Format("20060102_150405")
		shortID := cfg.RunID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		cfg.OutputDir = filepath.Join(".", fmt.Sprintf("regis_out_%s_%s", timestamp, shortID))
	}

	// Apply Server/Security Defaults
	cfg.EnsureDefaults()

	return &cfg, nil
}

// EnsureDefaults sets critical default values and security settings
func (c *Config) EnsureDefaults() {
	// Security: Ensure API Key exists
	if c.APIKey == "" {
		// Generate a simple random key
		c.APIKey = uuid.New().String()
		// We print this to stdout so the user sees it when starting the server
		// fmt.Printf("\n🔒 No REGIS_API_KEY found. Generated temporary key: \033[32m%s\033[0m\n", c.APIKey)
		// fmt.Printf("   Use header \033[1mX-API-Key: %s\033[0m for requests.\n\n", c.APIKey)
	}

	// Default Retention Policy: 7 Days
	if c.RetentionDays == 0 {
		c.RetentionDays = 7
	}

	// Default Validation Mode: consensus
	if c.ValidationMode == "" {
		c.ValidationMode = "consensus"
	}
}


// SetOutputDir sets the output directory explicitly
func (c *Config) SetOutputDir(outputDir string) {
	c.OutputDir = outputDir
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// TODO: Add validator package logic here
	if c.DataType != "" && c.DataType != "single" && c.DataType != "paired" {
		return fmt.Errorf("invalid data type: %s", c.DataType)
	}
	if c.Method != "" && c.Method != "denovo" && c.Method != "reference" {
		return fmt.Errorf("invalid analysis method: %s", c.Method)
	}
	return nil
}
