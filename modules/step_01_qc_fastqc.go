package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step01QCFastQC runs FastQC quality control on input FASTQ files
func Step01QCFastQC(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(1, "Quality Control with FastQC")

	// Create output directory
	fastqcDir := filepath.Join(cfg.OutputDir, "01_fastqc")
	if err := utils.CreateDirs(fastqcDir); err != nil {
		return fmt.Errorf("failed to create FastQC directory: %w", err)
	}

	// Run FastQC on file1
	utils.ShowProgress(fmt.Sprintf("Running FastQC on %s", filepath.Base(cfg.File1)))
	if err := utils.RunCommand(ctx, "fastqc", "-o", fastqcDir, cfg.File1); err != nil {
		return fmt.Errorf("FastQC failed on file1: %w", err)
	}

	// For paired-end data, run FastQC on file2
	if cfg.DataType == "paired" {
		utils.ShowProgress(fmt.Sprintf("Running FastQC on %s", filepath.Base(cfg.File2)))
		if err := utils.RunCommand(ctx, "fastqc", "-o", fastqcDir, cfg.File2); err != nil {
			return fmt.Errorf("FastQC failed on file2: %w", err)
		}
	}

	utils.StepComplete(1, "Quality Control with FastQC", stepStart)
	utils.Info("FastQC output saved to", zap.String("dir", fastqcDir))

	return nil
}
