package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BioinformaticsOnLine/regis/config"
)

// ValidateConfig validates the pipeline configuration
func ValidateConfig(cfg *config.Config) error {
	// Validate data type
	if cfg.DataType != "single" && cfg.DataType != "paired" {
		return fmt.Errorf("invalid data-type: %s (must be 'single' or 'paired')", cfg.DataType)
	}

	// Validate Email (Optional for local CLI, enforced by API handlers if needed)
	// if cfg.Email == "" { ... }

	// Validate method
	if cfg.Method != "denovo" && cfg.Method != "reference" {
		return fmt.Errorf("invalid method: %s (must be 'denovo' or 'reference')", cfg.Method)
	}

	// Validate and normalize assembler (only relevant for denovo)
	if cfg.Assembler == "" {
		cfg.Assembler = "trinity"
	}
	cfg.Assembler = strings.ToLower(cfg.Assembler)
	if cfg.Assembler != "trinity" && cfg.Assembler != "rnabloom" {
		return fmt.Errorf("invalid assembler: %s (must be 'trinity' or 'rnabloom')", cfg.Assembler)
	}

	// Validate and normalize strandedness
	if cfg.Stranded == "" {
		cfg.Stranded = "unstranded"
	}
	cfg.Stranded = strings.ToLower(cfg.Stranded)
	
	validStrand := false
	if cfg.Stranded == "unstranded" {
		validStrand = true
	} else if cfg.DataType == "paired" && (cfg.Stranded == "rf" || cfg.Stranded == "fr") {
		validStrand = true
	} else if cfg.DataType == "single" && (cfg.Stranded == "f" || cfg.Stranded == "r") {
		validStrand = true
	}

	if !validStrand {
		return fmt.Errorf("invalid strandedness '%s' for data_type '%s'. Valid options: unstranded, rf/fr (paired), f/r (single)", cfg.Stranded, cfg.DataType)
	}

	// Validate required files
	if cfg.File1 == "" {
		return fmt.Errorf("file1 is required")
	}

	if !FileExists(cfg.File1) {
		return fmt.Errorf("file1 does not exist: %s", cfg.File1)
	}

	// For paired-end, validate file2
	if cfg.DataType == "paired" {
		if cfg.File2 == "" {
			return fmt.Errorf("file2 is required for paired-end data")
		}
		if !FileExists(cfg.File2) {
			return fmt.Errorf("file2 does not exist: %s", cfg.File2)
		}
	}

	// For reference mode, validate reference and GTF
	if cfg.Method == "reference" {
		if cfg.Reference == "" {
			return fmt.Errorf("reference genome is required for reference mode")
		}
		if !FileExists(cfg.Reference) {
			return fmt.Errorf("reference genome does not exist: %s", cfg.Reference)
		}

		if cfg.GTF == "" {
			return fmt.Errorf("GTF annotation is required for reference mode")
		}
		if !FileExists(cfg.GTF) {
			return fmt.Errorf("GTF annotation does not exist: %s", cfg.GTF)
		}
	}

	// Validate output directory
	if cfg.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}

	// Validate and resolve threads
	// cfg.Threads == 0 means "use all available CPUs" (default / not set by user)
	availableCPUs := runtime.NumCPU()
	if cfg.Threads <= 0 {
		cfg.Threads = availableCPUs
	} else if cfg.Threads > availableCPUs {
		// Requested more threads than available — clamp to avoid fastp / HISAT2 errors
		cfg.Threads = availableCPUs
	}

	// Set asset paths if not provided
	// Support both development mode and conda installation
	if cfg.AssetsDir == "" {
		cfg.AssetsDir = FindAssetsDir()
	}
	// Convert to absolute path to avoid issues when running commands from different directories
	absAssetsDir, err := filepath.Abs(cfg.AssetsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for assets directory: %w", err)
	}
	cfg.AssetsDir = absAssetsDir

	if cfg.CPATModelsDir == "" {
		cfg.CPATModelsDir = filepath.Join(cfg.AssetsDir, "models")
	}
	if cfg.LncTarDir == "" {
		cfg.LncTarDir = filepath.Join(cfg.AssetsDir, "lnctar")
	}
	if cfg.LncTarScript == "" {
		cfg.LncTarScript = filepath.Join(cfg.LncTarDir, "LncTar.pl")
	}

	// Validate species (if provided)
	if cfg.Species != "" {
		normalizedSpecies, err := NormalizeSpecies(cfg.Species)
		if err != nil {
			return err
		}
		cfg.CPATSpecies = normalizedSpecies
	}

	// Determine validation mode and set CPAT model paths
	if cfg.SkipCPAT || cfg.Species == "" {
		cfg.ValidationMode = "cpc2-only"
	} else {
		// Check if species is supported by CPAT
		if IsCPATSupported(cfg.CPATSpecies) {
			cfg.ValidationMode = "consensus"

			// Set CPAT model file paths from bundled assets
			if cfg.CPATHex == "" && cfg.CPATLogit == "" {
				// Use bundled models
				cfg.CPATHexamerFile = filepath.Join(cfg.CPATModelsDir, cfg.CPATSpecies+"_Hexamer.tsv")
				cfg.CPATLogitFile = filepath.Join(cfg.CPATModelsDir, cfg.CPATSpecies+"_logitModel.RData")

				// Verify bundled models exist
				if !FileExists(cfg.CPATHexamerFile) || !FileExists(cfg.CPATLogitFile) {
					return fmt.Errorf("CPAT models not found for %s in %s", cfg.CPATSpecies, cfg.CPATModelsDir)
				}
			} else if cfg.CPATHex != "" && cfg.CPATLogit != "" {
				// Use custom models
				cfg.CPATHexamerFile = cfg.CPATHex
				cfg.CPATLogitFile = cfg.CPATLogit

				if !FileExists(cfg.CPATHexamerFile) || !FileExists(cfg.CPATLogitFile) {
					return fmt.Errorf("custom CPAT models not found")
				}
			} else {
				return fmt.Errorf("both CPAT hexamer and logit files must be provided if using custom models")
			}
		} else {
			cfg.ValidationMode = "cpc2-only"
		}
	}

	return nil
}

// NormalizeSpecies normalizes species names for CPAT
func NormalizeSpecies(species string) (string, error) {
	species = strings.ToLower(strings.TrimSpace(species))

	switch species {
	case "human", "homo sapiens", "h.sapiens", "hsapiens":
		return "Human", nil
	case "mouse", "mus musculus", "m.musculus", "mmusculus":
		return "Mouse", nil
	case "fly", "drosophila", "drosophila melanogaster", "d.melanogaster":
		return "Fly", nil
	case "zebrafish", "danio rerio", "d.rerio":
		return "Zebrafish", nil
	default:
		// Non-model organism, will use CPC2-only mode
		return species, nil
	}
}

// IsCPATSupported checks if a species is supported by CPAT
func IsCPATSupported(species string) bool {
	supported := map[string]bool{
		"Human":     true,
		"Mouse":     true,
		"Fly":       true,
		"Zebrafish": true,
	}
	return supported[species]
}

// ValidateInputFiles checks if multiple files exist
func ValidateInputFiles(files ...string) error {
	for _, file := range files {
		if !FileExists(file) {
			return fmt.Errorf("file does not exist: %s", file)
		}
	}
	return nil
}

// FindAssetsDir locates the assets directory
// Checks multiple locations to support both development and conda installation:
// 1. ./assets (development mode)
// 2. $CONDA_PREFIX/share/regis/assets (conda installation)
// 3. executable_dir/../share/regis/assets (relative to binary)
func FindAssetsDir() string {
	// 1. Check relative to executable (for relocated binary)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		relativeAssets := filepath.Join(exeDir, "..", "share", "regis", "assets")
		if DirExists(filepath.Join(relativeAssets, "models")) {
			return relativeAssets
		}
	}

	// 2. Check CONDA_PREFIX (conda installation)
	if condaPrefix := os.Getenv("CONDA_PREFIX"); condaPrefix != "" {
		condaAssets := filepath.Join(condaPrefix, "share", "regis", "assets")
		if DirExists(filepath.Join(condaAssets, "models")) {
			return condaAssets
		}
	}

	// 3. Check current directory (fallback/development mode)
	if DirExists("./assets/models") {
		return "./assets"
	}

	// Fallback to ./assets (will fail validation if not found)
	return "./assets"
}
