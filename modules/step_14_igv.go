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

// Step13IGV creates an interactive IGV report
func Step14IGV(ctx context.Context, cfg *config.Config) error {
	// Skip for de novo mode
	if cfg.Method != "reference" {
		utils.Info("IGV report requires reference mode, skipping")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(14, "Creating IGV Genome Browser Report")

	// Create directory
	// Bash: 13_igv_report
	igvDir := filepath.Join(cfg.OutputDir, "14_igv_report")
	if err := utils.CreateDirs(igvDir); err != nil {
		return fmt.Errorf("failed to create IGV report directory: %w", err)
	}

	gtfDir := filepath.Join(cfg.OutputDir, "08_lncrna_analysis")
	lncRNABed := filepath.Join(gtfDir, "filtered", "lncrna.bed")

	if !utils.FileExists(lncRNABed) {
		utils.Warn("lncRNA BED file not found, skipping IGV report")
		return nil
	}

	utils.ShowProgress("Generating interactive IGV report")

	// Prepare tracks list
	alignmentDir := filepath.Join(cfg.OutputDir, "04_alignment")
	bamFile := filepath.Join(alignmentDir, "aligned_sorted.bam")

	// Build tracks list
	tracks := []string{lncRNABed}
	if utils.FileExists(bamFile) {
		tracks = append(tracks, bamFile)
	}
	if utils.FileExists(cfg.GTF) {
		tracks = append(tracks, cfg.GTF)
	}

	// Create IGV report
	reportFile := filepath.Join(igvDir, "lncrna_igv_report.html")
	args := []string{
		lncRNABed,
		"--fasta", cfg.Reference,
		"--flanking", "2000",
		"--title", "lncRNA Genome Browser Report",
		"--output", reportFile,
	}

	// Add tracks
	args = append(args, "--tracks")
	args = append(args, tracks...)

	if err := utils.RunCommand(ctx, "create_report", args...); err != nil {
		utils.Warn("create_report failed", zap.Error(err))
		utils.Info("Install with: mamba install -c bioconda igv-reports")
		return nil
	}

	if utils.FileExists(reportFile) {
		utils.Info("IGV report created", zap.String("file", reportFile))
		utils.Info("Open this file in a web browser to interactively explore lncRNAs")
	}

	utils.StepComplete(13, "IGV Report", stepStart)
	utils.Info("IGV report complete", zap.String("output", igvDir))

	return nil
}
