package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step12RSeQC performs RNA-seq quality control analysis
func Step13RSeQC(ctx context.Context, cfg *config.Config) error {
	// Skip for de novo mode
	if cfg.Method != "reference" {
		utils.Info("RSeQC requires reference mode, skipping")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(13, "RNA-seq Quality Assessment with RSeQC")

	// Create directory
	// Bash: 12_rseqc
	rseqcDir := filepath.Join(cfg.OutputDir, "13_rseqc")
	if err := utils.CreateDirs(rseqcDir); err != nil {
		return fmt.Errorf("failed to create RSeQC directory: %w", err)
	}

	alignmentDir := filepath.Join(cfg.OutputDir, "04_alignment")
	bamFile := filepath.Join(alignmentDir, "aligned_sorted.bam")

	if !utils.FileExists(bamFile) {
		utils.Warn("BAM file not found, skipping RSeQC")
		return nil
	}

	utils.ShowProgress("Running RSeQC quality checks")

	// Convert GTF to BED12 for RSeQC
	bed12File := filepath.Join(rseqcDir, "reference.bed12")
	if err := convertGTFToBED12(ctx, cfg.GTF, rseqcDir, bed12File); err != nil {
		utils.Warn("Failed to convert GTF to BED12", zap.Error(err))
		return nil
	}

	if !utils.FileExists(bed12File) {
		utils.Warn("BED12 file not created, skipping RSeQC")
		return nil
	}

	// Gene body coverage
	utils.ShowProgress("Calculating gene body coverage")
	if err := utils.RunCommand(ctx, "geneBody_coverage.py",
		"-i", bamFile,
		"-r", bed12File,
		"-o", filepath.Join(rseqcDir, "gene_body_coverage"),
	); err != nil {
		utils.Warn("geneBody_coverage.py failed", zap.Error(err))
	}

	// Read distribution
	// Read distribution
	utils.ShowProgress("Analyzing read distribution")
	readDistFile := filepath.Join(rseqcDir, "read_distribution.txt")
	// Run command directly and capture output
	if output, err := utils.RunCommandWithOutput(ctx, "read_distribution.py", "-i", bamFile, "-r", bed12File); err != nil {
		utils.Warn("read_distribution.py failed", zap.Error(err))
	} else {
		// Write output to file
		if err := os.WriteFile(readDistFile, []byte(output), 0644); err != nil {
			utils.Warn("Failed to write read distribution output", zap.Error(err))
		}
	}

	// Infer experiment (strand specificity)
	// Infer experiment (strand specificity)
	utils.ShowProgress("Inferring experiment type")
	inferExpFile := filepath.Join(rseqcDir, "infer_experiment.txt")
	// Run command directly and capture output
	if output, err := utils.RunCommandWithOutput(ctx, "infer_experiment.py", "-i", bamFile, "-r", bed12File); err != nil {
		utils.Warn("infer_experiment.py failed", zap.Error(err))
	} else {
		// Write output to file
		if err := os.WriteFile(inferExpFile, []byte(output), 0644); err != nil {
			utils.Warn("Failed to write infer experiment output", zap.Error(err))
		}
	}

	// Junction saturation
	utils.ShowProgress("Analyzing junction saturation")
	if err := utils.RunCommand(ctx, "junction_saturation.py",
		"-i", bamFile,
		"-r", bed12File,
		"-o", filepath.Join(rseqcDir, "junction_saturation"),
	); err != nil {
		utils.Warn("junction_saturation.py failed", zap.Error(err))
	}

	utils.StepComplete(12, "RSeQC Quality Assessment", stepStart)
	utils.Info("RSeQC analysis complete", zap.String("output", rseqcDir))

	return nil
}

// convertGTFToBED12 converts GTF/GFF to BED12 format for RSeQC
func convertGTFToBED12(ctx context.Context, gtfFile, workDir, outputBed string) error {
	// GTF/GFF -> GenePred -> BED12
	genePredFile := filepath.Join(workDir, "reference.genepred")

	// Check if input is GFF and needs conversion
	var gtfForConversion string
	if filepath.Ext(gtfFile) == ".gff" || filepath.Ext(gtfFile) == ".gff3" {
		// Convert GFF to GTF using gffread
		gtfForConversion = filepath.Join(workDir, "reference.gtf")
		if err := utils.RunCommand(ctx, "gffread", gtfFile, "-T", "-o", gtfForConversion); err != nil {
			return fmt.Errorf("gffread conversion failed: %w", err)
		}
	} else {
		// Already GTF, use as is
		gtfForConversion = gtfFile
	}

	// Convert GTF to GenePred
	if err := utils.RunCommand(ctx, "gtfToGenePred", gtfForConversion, genePredFile); err != nil {
		return fmt.Errorf("gtfToGenePred failed: %w", err)
	}

	// Convert GenePred to BED12
	if err := utils.RunCommand(ctx, "genePredToBed", genePredFile, outputBed); err != nil {
		return fmt.Errorf("genePredToBed failed: %w", err)
	}

	// Clean up intermediate files
	// Clean up intermediate files
	os.Remove(genePredFile)
	if gtfForConversion != gtfFile {
		os.Remove(gtfForConversion)
	}

	return nil
}
