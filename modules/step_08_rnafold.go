package modules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step08RNAfold predicts lncRNA secondary structures using RNAfold
func Step08RNAfold(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(8, "lncRNA Secondary Structure Prediction")

	// Create directories
	rnafoldDir := filepath.Join(cfg.OutputDir, "09_rnafold")
	// psDir := filepath.Join(rnafoldDir, "ps_files") // Deprecated
	// pngDir := filepath.Join(rnafoldDir, "png_files") // Deprecated
	svgDir := filepath.Join(rnafoldDir, "svg_files")

	if err := utils.CreateDirs(rnafoldDir, svgDir); err != nil {
		return fmt.Errorf("failed to create RNAfold directories: %w", err)
	}

	// Input file
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")

	if !utils.FileExists(transcriptsFa) {
		return fmt.Errorf("transcripts.fa not found: %s", transcriptsFa)
	}

	// Run RNAfold in its output directory
	utils.ShowProgress("Predicting lncRNA secondary structures")

	// Change to rnafold directory to keep output organized
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(rnafoldDir); err != nil {
		return fmt.Errorf("failed to change to RNAfold directory: %w", err)
	}

	// Run RNAfold with input redirection
	// Use --noPS to avoid generating PostScript files (we will generate SVGs with RNAplot)
	outputFile := filepath.Join(rnafoldDir, "lncrna_structures.out")
	if err := runRNAfold(ctx, transcriptsFa, outputFile); err != nil {
		return fmt.Errorf("RNAfold failed: %w", err)
	}

	// Generate SVGs using RNAplot
	utils.ShowProgress("Generating SVG visualizations with RNAplot")
	if err := generateSVGs(ctx, outputFile, svgDir); err != nil {
		utils.Warn("SVG generation failed", zap.Error(err))
	}

	utils.StepComplete(8, "lncRNA Secondary Structure Prediction", stepStart)
	utils.Info("Structure prediction complete",
		zap.String("svg_files", svgDir),
		zap.String("structures", outputFile))

	return nil
}

// runRNAfold runs RNAfold with input redirection
func runRNAfold(ctx context.Context, inputFile, outputFile string) error {
	// Read input file
	input, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Create output file
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()

	// Run RNAfold with stdin/stdout redirection
	// --noPS: Don't produce PS files automatically
	cmd := exec.CommandContext(ctx, "RNAfold", "--noPS")
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = output

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	utils.Info("Running RNAfold (MFE calculation)")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("RNAfold failed: %w\nStderr: %s", err, stderr.String())
	}

	return nil
}

// generateSVGs uses RNAplot to convert structure output to SVG
func generateSVGs(ctx context.Context, structureFile, svgDir string) error {
	// Read the structure file (contains Sequence + Structure + Energy)
	input, err := os.ReadFile(structureFile)
	if err != nil {
		return err
	}

	// We can feed the structure file directly to RNAplot
	// RNAplot -f svg
	cmd := exec.CommandContext(ctx, "RNAplot", "--output-format=svg")
	cmd.Stdin = bytes.NewReader(input)
	
	// RNAplot writes to current directory, so we should be in rnafoldDir already (from Step08RNAfold)
	
	utils.Info("Running RNAplot (SVG generation)")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("RNAplot failed: %w", err)
	}

	// Move all generated .svg files to svgDir
	files, err := filepath.Glob("*_ss.svg")
	if err != nil {
		return err
	}

	for _, file := range files {
		destPath := filepath.Join(svgDir, file)
		if err := os.Rename(file, destPath); err != nil {
			utils.Warn("Failed to move SVG file", zap.String("file", file), zap.Error(err))
		}
	}

	count := len(files)
	utils.Info(fmt.Sprintf("Generated %d SVG structure plots", count))

	return nil
}
