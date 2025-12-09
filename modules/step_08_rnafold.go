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

// Step07RNAfold predicts lncRNA secondary structures using RNAfold
func Step08RNAfold(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(8, "lncRNA Secondary Structure Prediction")

	// Create directories
	rnafoldDir := filepath.Join(cfg.OutputDir, "09_rnafold")
	psDir := filepath.Join(rnafoldDir, "ps_files")
	pngDir := filepath.Join(rnafoldDir, "png_files")

	if err := utils.CreateDirs(rnafoldDir, psDir, pngDir); err != nil {
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
	outputFile := filepath.Join(rnafoldDir, "lncrna_structures.out")
	if err := runRNAfold(ctx, transcriptsFa, outputFile); err != nil {
		return fmt.Errorf("RNAfold failed: %w", err)
	}

	// Organize PS files and convert to PNG
	utils.ShowProgress("Organizing files and converting PS to PNG")
	if err := organizeAndConvertPS(rnafoldDir, psDir, pngDir); err != nil {
		utils.Warn("PS to PNG conversion failed", zap.Error(err))
	}

	utils.StepComplete(7, "lncRNA Secondary Structure Prediction", stepStart)
	utils.Info("Structure prediction complete",
		zap.String("ps_files", psDir),
		zap.String("png_files", pngDir),
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
	cmd := exec.CommandContext(ctx, "RNAfold")
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = output

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	utils.Info("Running RNAfold")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("RNAfold failed: %w\nStderr: %s", err, stderr.String())
	}

	return nil
}

// organizeAndConvertPS moves PS files and converts them to PNG
func organizeAndConvertPS(rnafoldDir, psDir, pngDir string) error {
	// Find all .ps files in rnafold directory
	files, err := filepath.Glob(filepath.Join(rnafoldDir, "*.ps"))
	if err != nil {
		return err
	}

	for _, psFile := range files {
		baseName := filepath.Base(psFile)

		// Move PS file
		newPSPath := filepath.Join(psDir, baseName)
		if err := os.Rename(psFile, newPSPath); err != nil {
			utils.Warn(fmt.Sprintf("Failed to move %s", baseName), zap.Error(err))
			continue
		}

		// Convert to PNG using ImageMagick
		pngName := baseName[:len(baseName)-3] + ".png"
		pngPath := filepath.Join(pngDir, pngName)

		// magick ps_file -density 300 -background white -flatten png_file
		cmd := exec.Command("magick", newPSPath,
			"-density", "300",
			"-background", "white",
			"-flatten", pngPath)

		if err := cmd.Run(); err != nil {
			utils.Warn(fmt.Sprintf("Failed to convert %s to PNG", baseName), zap.Error(err))
		}
	}

	return nil
}
