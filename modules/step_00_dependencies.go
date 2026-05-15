package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
)

// Step00CheckDependencies verifies that all required external tools are available.
func Step00CheckDependencies(ctx context.Context, cfg *config.Config) error {
	utils.Info("Checking for required dependencies...")

	var missingTools []string

	// 1. Core Tools (always required)
	coreTools := []string{
		"fastqc",
		"fastp",

		"gffcompare",
		"RNAfold",
		"bedtools",
	}

	for _, tool := range coreTools {
		if _, err := exec.LookPath(tool); err != nil {
			missingTools = append(missingTools, tool)
		}
	}

	// 2. Conditional Tools based on configuration

	// SortMeRNA
	if cfg.EnableSortMeRNA {
		if _, err := exec.LookPath("sortmerna"); err != nil {
			missingTools = append(missingTools, "sortmerna")
		}
	}

	// Reference-based vs De Novo
	if cfg.Method == "reference" {
		refTools := []string{"hisat2", "samtools", "stringtie", "gffread"}
		for _, tool := range refTools {
			if _, err := exec.LookPath(tool); err != nil {
				missingTools = append(missingTools, tool)
			}
		}
	} else if cfg.Method == "denovo" {
		if cfg.Assembler == "rnabloom" {
			if _, err := exec.LookPath("rnabloom"); err != nil {
				missingTools = append(missingTools, "rnabloom")
			}
		} else {
			// Trinity sometimes installed as Trinity or trinity
			if _, err := exec.LookPath("Trinity"); err != nil {
				if _, err := exec.LookPath("trinity"); err != nil {
					missingTools = append(missingTools, "Trinity")
				}
			}
		}
	}

	// CPAT
	if !cfg.SkipCPAT {
		// Check binary
		cpatFound := false
		if _, err := exec.LookPath("cpat.py"); err == nil {
			cpatFound = true
		} else if _, err := exec.LookPath("cpat"); err == nil {
			cpatFound = true
		}

		if !cpatFound {
			missingTools = append(missingTools, "cpat.py")
		}

		// Check keys exist in config (already validated/set by FindAssetsDir in validation.go)
		if cfg.Species != "" {
			// Use the resolved paths from config
			if _, err := os.Stat(cfg.CPATHexamerFile); os.IsNotExist(err) {
				missingTools = append(missingTools, fmt.Sprintf("CPAT Hexamer Model (%s)", cfg.CPATHexamerFile))
			}
			if _, err := os.Stat(cfg.CPATLogitFile); os.IsNotExist(err) {
				missingTools = append(missingTools, fmt.Sprintf("CPAT Logit Model (%s)", cfg.CPATLogitFile))
			}
		}
	}

	// CPC2 (usually Python-based)
	// Some installs expose 'cpc2', others require 'python /path/to/cpc2.py'
	// We'll check for 'cpc2' in path for now, but fail gracefully if python is there but cpc2 isn't wrapped
	if _, err := exec.LookPath("cpc2"); err != nil {
		// Try checking generically if it might be a script, but standard practice involves a wrapper
		missingTools = append(missingTools, "cpc2")
	}

	// LncTar
	if cfg.EnableLncTar {
		// Requires perl
		if _, err := exec.LookPath("perl"); err != nil {
			missingTools = append(missingTools, "perl (for LncTar)")
		}

		// Check for bundled script using config path
		if _, err := os.Stat(cfg.LncTarScript); os.IsNotExist(err) {
			missingTools = append(missingTools, fmt.Sprintf("LncTar script (%s)", cfg.LncTarScript))
		}
	}

	// IntaRNA
	if cfg.EnableIntaRNA {
		if _, err := exec.LookPath("IntaRNA"); err != nil {
			missingTools = append(missingTools, "IntaRNA")
		}
	}

	// Report results
	if len(missingTools) > 0 {
		errMsg := fmt.Sprintf("Missing required tools/files:\n  - %s\n\nPlease install them or ensure they are in your PATH/assets folder.", strings.Join(missingTools, "\n  - "))
		utils.Error(errMsg)
		return fmt.Errorf("dependency check failed: missing %s", strings.Join(missingTools, ", "))
	}

	// Log versions for key tools
	logToolVersions()

	utils.Info("✓ All dependencies satisfied")
	return nil
}

func logToolVersions() {
	tools := []struct {
		name string
		arg  string
	}{
		{"fastqc", "--version"},
		{"fastp", "--version"},
		{"samtools", "--version"},
		{"stringtie", "--version"},
		{"gffread", "--version"},
		{"bedtools", "--version"},

		{"cpc2", "--version"},
		// User requested additions
		{"cpat.py", "--version"},
		{"multiqc", "--version"},
		{"Trinity", "--version"},
		{"rnabloom", "-v"},
		{"featureCounts", "-v"}, // Subread package
		{"IntaRNA", "--version"},
	}

	for _, t := range tools {
		if path, err := exec.LookPath(t.name); err == nil {
			// Get version
			cmd := exec.Command(path, t.arg)
			out, err := cmd.CombinedOutput()
			if err == nil {
				// Take just the first line/part of output to be concise
				ver := strings.TrimSpace(string(out))
				if idx := strings.Index(ver, "\n"); idx != -1 {
					ver = ver[:idx]
				}
				utils.Info(fmt.Sprintf("%s found: %s", t.name, ver))
			} else {
				utils.Info(fmt.Sprintf("%s found (version check failed)", t.name))
			}
		}
	}
}
