package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step08LncTar predicts lncRNA-mRNA interactions using LncTar (optional)
func Step09LncTar(ctx context.Context, cfg *config.Config) error {
	// Skip if not enabled
	if !cfg.EnableLncTar {
		utils.Info("💡 Tip: Use --lnctar flag to predict lncRNA-mRNA interactions (adds ~15-20 min)")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(9, "Predicting lncRNA-mRNA Interactions with LncTar")

	// Create directories
	// Bash: 10_target_prediction/lnctar
	targetDir := filepath.Join(cfg.OutputDir, "11_target_prediction")
	lnctarDir := filepath.Join(targetDir, "lnctar")
	if err := utils.CreateDirs(lnctarDir); err != nil {
		return fmt.Errorf("failed to create lnctar directory: %w", err)
	}

	// Check required files
	gtfDir := filepath.Join(cfg.OutputDir, "08_lncrna_analysis")
	lncRNAFa := filepath.Join(gtfDir, "filtered", "lncrna_filtered.fa")
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")

	if !utils.FileExists(lncRNAFa) || !utils.FileExists(transcriptsFa) {
		utils.Warn("Required files not found, skipping LncTar")
		return nil
	}

	utils.Info("Running LncTar to predict RNA-RNA interactions")

	// Verify LncTar script exists
	if !utils.FileExists(cfg.LncTarScript) {
		return fmt.Errorf("LncTar script not found: %s", cfg.LncTarScript)
	}

	utils.Info("Using bundled LncTar", zap.String("script", cfg.LncTarScript))

	// Set PERL5LIB to include LncTar directory
	lnctarScriptDir := filepath.Dir(cfg.LncTarScript)
	oldPerl5Lib := os.Getenv("PERL5LIB")
	newPerl5Lib := lnctarScriptDir
	if oldPerl5Lib != "" {
		newPerl5Lib = lnctarScriptDir + ":" + oldPerl5Lib
	}
	os.Setenv("PERL5LIB", newPerl5Lib)
	defer os.Setenv("PERL5LIB", oldPerl5Lib)

	// Run LncTar based on mode
	expressionDir := filepath.Join(gtfDir, "expression")

	// Mode 1: Best candidates only (fastest, most focused)
	if cfg.LncTarBestOnly {
		if err := runLncTarBestCandidates(ctx, cfg, lnctarDir, expressionDir, gtfDir, transcriptsFa, lnctarScriptDir); err != nil {
			utils.Warn("LncTar best candidates failed", zap.Error(err))
		}
	} else if utils.FileExists(filepath.Join(expressionDir, "highly_expressed.fa")) {
		// Mode 2: Highly expressed lncRNAs (default, fast and focused)
		if err := runLncTarHighlyExpressed(ctx, cfg, lnctarDir, expressionDir, transcriptsFa, lnctarScriptDir); err != nil {
			utils.Warn("LncTar highly expressed failed", zap.Error(err))
		}
	}

	// Mode 3: Comprehensive (all lncRNAs) - optional
	if cfg.LncTarComprehensive {
		if err := runLncTarComprehensive(ctx, cfg, lnctarDir, lncRNAFa, transcriptsFa, lnctarScriptDir); err != nil {
			utils.Warn("LncTar comprehensive failed", zap.Error(err))
		}
	} else {
		utils.Info("Tip: Use --lnctar-all to analyze all lncRNAs (including low expression)")
	}

	utils.StepComplete(8, "LncTar Target Prediction", stepStart)
	utils.Info("LncTar complete", zap.String("output", lnctarDir))

	return nil
}

// runLncTarBestCandidates runs LncTar on best candidates (novel + highly expressed)
func runLncTarBestCandidates(ctx context.Context, cfg *config.Config, lnctarDir, expressionDir, gtfDir, transcriptsFa, lnctarScriptDir string) error {
	bestCandidatesFile := filepath.Join(expressionDir, "best_candidates.txt")
	if !utils.FileExists(bestCandidatesFile) {
		utils.Warn("No best candidates available - skipping LncTar")
		utils.Info("Tip: Best candidates = highly expressed + novel lncRNAs")
		return nil
	}

	utils.ShowProgress("Running LncTar on BEST candidates only (novel + highly expressed)")

	// Extract best candidate sequences
	lncRNAFa := filepath.Join(gtfDir, "filtered", "lncrna_filtered.fa")
	bestCandidatesFa := filepath.Join(lnctarDir, "best_candidates.fa")

	if err := utils.ExtractSequences("Step09-Best", bestCandidatesFile, lncRNAFa, bestCandidatesFa); err != nil {
		return err
	}

	// Count sequences
	bestCount, err := countFastaSequences(bestCandidatesFa)
	if err != nil || bestCount == 0 {
		utils.Warn("No best candidate sequences found")
		return nil
	}

	utils.Info(fmt.Sprintf("Analyzing %d best candidate lncRNAs", bestCount))

	// Run LncTar from its directory
	outputFile := filepath.Join(lnctarDir, "best_candidates_targets.txt")
	if err := runLncTarPerl(ctx, lnctarScriptDir, cfg.LncTarScript, bestCandidatesFa, transcriptsFa, outputFile); err != nil {
		return err
	}

	// Parse results
	if utils.FileExists(outputFile) {
		interactionCount, _ := countLncTarInteractions(outputFile)
		utils.Info(fmt.Sprintf("Found %d potential interactions for best candidates", interactionCount))
	}

	return nil
}

// runLncTarHighlyExpressed runs LncTar on highly expressed lncRNAs
func runLncTarHighlyExpressed(ctx context.Context, cfg *config.Config, lnctarDir, expressionDir, transcriptsFa, lnctarScriptDir string) error {
	highlyExpressedFa := filepath.Join(expressionDir, "highly_expressed.fa")
	if !utils.FileExists(highlyExpressedFa) {
		return nil
	}

	utils.ShowProgress("Predicting targets for highly expressed lncRNAs")

	outputFile := filepath.Join(lnctarDir, "highly_expressed_targets.txt")
	if err := runLncTarPerl(ctx, lnctarScriptDir, cfg.LncTarScript, highlyExpressedFa, transcriptsFa, outputFile); err != nil {
		return err
	}

	// Parse results
	if utils.FileExists(outputFile) {
		interactionCount, _ := countLncTarInteractions(outputFile)
		utils.Info(fmt.Sprintf("Found %d potential lncRNA-mRNA interactions", interactionCount))
	}

	return nil
}

// runLncTarComprehensive runs LncTar on all filtered lncRNAs
func runLncTarComprehensive(ctx context.Context, cfg *config.Config, lnctarDir, lncRNAFa, transcriptsFa, lnctarScriptDir string) error {
	utils.ShowProgress("Running comprehensive LncTar analysis (--lnctar-all flag set)")

	lncrnaCount, err := countFastaSequences(lncRNAFa)
	if err != nil {
		return err
	}

	utils.Info(fmt.Sprintf("Analyzing ALL %d lncRNAs (this may take longer)", lncrnaCount))

	outputFile := filepath.Join(lnctarDir, "all_lncrna_targets.txt")
	if err := runLncTarPerl(ctx, lnctarScriptDir, cfg.LncTarScript, lncRNAFa, transcriptsFa, outputFile); err != nil {
		return err
	}

	// Parse results
	if utils.FileExists(outputFile) {
		allInteractionCount, _ := countLncTarInteractions(outputFile)
		utils.Info(fmt.Sprintf("Found %d total interactions (comprehensive analysis)", allInteractionCount))
	}

	return nil
}

// runLncTarPerl runs the LncTar Perl script
func runLncTarPerl(ctx context.Context, workDir, script, lncRNAFile, mRNAFile, outputFile string) error {
	// Change to LncTar directory to run the script (required for Perl module dependencies)
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		return err
	}

	// LncTar.pl -p 1 -l lncrna.fa -m mrna.fa -d -0.1 -s F -o output.txt
	return utils.RunCommand(ctx, "perl", filepath.Base(script),
		"-p", "1", // Type 1: separate lncRNA and mRNA files
		"-l", lncRNAFile,
		"-m", mRNAFile,
		"-d", "-0.1", // ndG threshold
		"-s", "F", // Standard free energy
		"-o", outputFile,
	)
}

// countFastaSequences counts sequences in a FASTA file
func countFastaSequences(fastaFile string) (int, error) {
	lines, err := utils.ReadLines(fastaFile)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, ">") {
			count++
		}
	}
	return count, nil
}

// countLncTarInteractions counts interactions in LncTar output (skip header)
func countLncTarInteractions(outputFile string) (int, error) {
	lines, err := utils.ReadLines(outputFile)
	if err != nil {
		return 0, err
	}

	// Skip header line
	if len(lines) > 1 {
		return len(lines) - 1, nil
	}
	return 0, nil
}
