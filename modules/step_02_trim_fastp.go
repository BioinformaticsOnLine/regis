package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step02TrimFastp runs fastp quality trimming and adapter removal
func Step02TrimFastp(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(2, "Quality Trimming with fastp")

	// Create output directory
	trimDir := filepath.Join(cfg.OutputDir, "02_trimming")
	if err := utils.CreateDirs(trimDir); err != nil {
		return fmt.Errorf("failed to create trimming directory: %w", err)
	}

	// fastp report outputs
	jsonReport := filepath.Join(trimDir, "fastp_report.json")
	htmlReport := filepath.Join(trimDir, "fastp_report.html")

	if cfg.DataType == "paired" {
		// Paired-end mode
		utils.ShowProgress(fmt.Sprintf("Trimming paired-end reads with fastp (using %d threads)", cfg.Threads))

		paired1 := filepath.Join(trimDir, "paired_1.fastq")
		paired2 := filepath.Join(trimDir, "paired_2.fastq")
		unpaired1 := filepath.Join(trimDir, "unpaired_1.fastq")
		unpaired2 := filepath.Join(trimDir, "unpaired_2.fastq")

		args := []string{
			"-i", cfg.File1,
			"-I", cfg.File2,
			"-o", paired1,
			"-O", paired2,
			"--unpaired1", unpaired1,
			"--unpaired2", unpaired2,
			"-w", strconv.Itoa(cfg.Threads),
			"-l", "36", // Minimum read length (same as previous MINLEN:36)
			"-j", jsonReport,
			"-h", htmlReport,
		}

		if err := utils.RunCommand(ctx, "fastp", args...); err != nil {
			return fmt.Errorf("fastp PE failed: %w", err)
		}

		// Update config to use trimmed files
		cfg.File1 = paired1
		cfg.File2 = paired2

	} else {
		// Single-end mode
		utils.ShowProgress(fmt.Sprintf("Trimming single-end reads with fastp (using %d threads)", cfg.Threads))

		trimmed := filepath.Join(trimDir, "trimmed.fastq")

		args := []string{
			"-i", cfg.File1,
			"-o", trimmed,
			"-w", strconv.Itoa(cfg.Threads),
			"-l", "36", // Minimum read length
			"-j", jsonReport,
			"-h", htmlReport,
		}

		if err := utils.RunCommand(ctx, "fastp", args...); err != nil {
			return fmt.Errorf("fastp SE failed: %w", err)
		}

		// Update config to use trimmed file
		cfg.File1 = trimmed
	}

	utils.StepComplete(2, "Quality Trimming with fastp", stepStart)
	utils.Info("Trimmed reads saved to", zap.String("dir", trimDir))

	return nil
}
