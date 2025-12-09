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

// Step14MultiQC creates a comprehensive QC dashboard
func Step15MultiQC(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(15, "Generating MultiQC Report")

	// Create directory
	// Bash: 14_multiqc
	multiqcDir := filepath.Join(cfg.OutputDir, "15_multiqc")
	if err := utils.CreateDirs(multiqcDir); err != nil {
		return fmt.Errorf("failed to create MultiQC directory: %w", err)
	}

	utils.ShowProgress("Creating comprehensive QC report")

	// Run MultiQC on entire output directory
	if err := utils.RunCommand(ctx, "multiqc",
		cfg.OutputDir,
		"-o", multiqcDir,
		"-n", "lncrna_pipeline_report.html",
		"--title", "Regis lncRNA Pipeline Report",
		"--comment", "Complete quality control and analysis summary",
		"--force",
		"--quiet",
	); err != nil {
		utils.Warn("MultiQC failed", zap.Error(err))
		utils.Info("Install with: mamba install -c bioconda multiqc")
		return nil
	}

	reportFile := filepath.Join(multiqcDir, "lncrna_pipeline_report.html")
	if utils.FileExists(reportFile) {
		utils.Info("MultiQC report created", zap.String("file", reportFile))
		utils.Info("Open this file in a web browser to view interactive QC dashboard")
	}

	utils.StepComplete(14, "MultiQC Dashboard", stepStart)
	utils.Info("MultiQC report generated", zap.String("output", multiqcDir))

	return nil
}
