package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step09IntaRNA predicts RNA-RNA interactions using IntaRNA (optional)
func Step10IntaRNA(ctx context.Context, cfg *config.Config) error {
	// Skip if not enabled
	if !cfg.EnableIntaRNA {
		utils.Info("IntaRNA disabled, skipping")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(10, "Cross-Validating Targets with IntaRNA")

	// Create directories
	// Bash: 10_target_prediction/intarna
	targetDir := filepath.Join(cfg.OutputDir, "11_target_prediction")
	intarnaDir := filepath.Join(targetDir, "intarna")
	if err := utils.CreateDirs(intarnaDir); err != nil {
		return fmt.Errorf("failed to create intarna directory: %w", err)
	}

	// Get input files
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")
	gtfDir := filepath.Join(cfg.OutputDir, "08_lncrna_analysis")
	expressionDir := filepath.Join(gtfDir, "expression")
	// Updated path to match new structure
	lnctarDir := filepath.Join(cfg.OutputDir, "11_target_prediction", "lnctar")

	// Determine input file and output file based on mode
	var intarnaInput, intarnaOutput string

	// Mode 1: Best candidates only (matching LncTar behavior)
	if cfg.IntaRNABestOnly {
		bestCandidatesFa := filepath.Join(lnctarDir, "best_candidates.fa")
		if utils.FileExists(bestCandidatesFa) {
			utils.ShowProgress("Running IntaRNA on BEST candidates only (novel + highly expressed)")
			intarnaInput = bestCandidatesFa
			intarnaOutput = filepath.Join(intarnaDir, "best_candidates_targets.csv")
		} else {
			utils.Warn("No best candidate lncRNAs found - skipping IntaRNA")
			return nil
		}
	} else if cfg.IntaRNAComprehensive {
		// Mode 2: All lncRNAs (comprehensive mode)
		lncRNAFa := filepath.Join(gtfDir, "filtered", "lncrna_filtered.fa")
		if utils.FileExists(lncRNAFa) {
			lncrnaCount, _ := countFastaSequences(lncRNAFa)
			utils.ShowProgress(fmt.Sprintf("Running comprehensive IntaRNA analysis (ALL %d lncRNAs)", lncrnaCount))
			intarnaInput = lncRNAFa
			intarnaOutput = filepath.Join(intarnaDir, "all_lncrna_targets.csv")
		} else {
			utils.Warn("No filtered lncRNAs found - skipping IntaRNA")
			return nil
		}
	} else {
		// Mode 3: Default - highly expressed lncRNAs only
		highlyExpressedFa := filepath.Join(expressionDir, "highly_expressed.fa")
		if utils.FileExists(highlyExpressedFa) {
			utils.ShowProgress("Running IntaRNA on highly expressed lncRNAs")
			intarnaInput = highlyExpressedFa
			intarnaOutput = filepath.Join(intarnaDir, "highly_expressed_targets.csv")
		} else {
			utils.Warn("No highly expressed lncRNAs found - skipping IntaRNA")
			return nil
		}
	}

	// Run IntaRNA if we have valid input
	if intarnaInput == "" || !utils.FileExists(intarnaInput) {
		return nil
	}

	lncCount, _ := countFastaSequences(intarnaInput)
	utils.Info(fmt.Sprintf("Analyzing %d lncRNAs with IntaRNA (using %d threads)", lncCount, cfg.Threads))

	// Optimization: Filter targets to include ONLY coding transcripts (mRNAs)
	// IntaRNA is slow ($O(N \cdot M)$), so searching against non-coding targets is wasteful.
	utils.ShowProgress("Optimizing IntaRNA: Filtering targets to coding transcripts only")
	cpc2Output := filepath.Join(cfg.OutputDir, "06_cpc2", "cpc2_output.txt")
	codingIDsFile := filepath.Join(intarnaDir, "coding_ids.txt")
	codingFa := filepath.Join(intarnaDir, "coding_transcripts.fa")

	if err := extractCodingIDs(cpc2Output, codingIDsFile); err != nil {
		utils.Warn("Failed to extracting coding IDs from CPC2 output - falling back to full transcriptome", zap.Error(err))
	} else {
		// Extract coding sequences
		if err := utils.ExtractSequences("Step10-Coding", codingIDsFile, transcriptsFa, codingFa); err != nil {
			utils.Warn("Failed to create coding transcripts subset - falling back to full transcriptome", zap.Error(err))
		} else {
			// Success: Switch target to coding transcripts
			transcriptsFa = codingFa
			targetCount, _ := countFastaSequences(codingFa)
			utils.Info(fmt.Sprintf("Target search space reduced to %d coding transcripts (optimized)", targetCount))
		}
	}

	// Copy input FASTA to IntaRNA directory for reference
	var copyDest string
	if cfg.IntaRNABestOnly {
		copyDest = filepath.Join(intarnaDir, "best_candidates.fa")
	} else if cfg.IntaRNAComprehensive {
		copyDest = filepath.Join(intarnaDir, "all_lncrnas.fa")
	} else {
		copyDest = filepath.Join(intarnaDir, "highly_expressed.fa")
	}
	utils.CopyFile(intarnaInput, copyDest)

	// Run IntaRNA with multi-threading, outputting in CSV format
	if err := utils.RunCommand(ctx, "IntaRNA",
		"-q", intarnaInput,
		"-t", transcriptsFa, // Now points to optimized coding_transcripts.fa if successful
		"--threads", fmt.Sprintf("%d", cfg.Threads),
		"--outMode", "C", // CSV output
		"--out", intarnaOutput,
	); err != nil {
		return fmt.Errorf("IntaRNA failed: %w", err)
	}

	// Parse results
	if utils.FileExists(intarnaOutput) {
		interactionCount, _ := countIntaRNAInteractions(intarnaOutput)
		utils.Info(fmt.Sprintf("Found %d potential interactions with IntaRNA", interactionCount))
		utils.Info("Results file", zap.String("file", intarnaOutput))
	}

	utils.StepComplete(9, "IntaRNA Target Prediction", stepStart)
	utils.Info("IntaRNA complete", zap.String("output", intarnaDir))

	return nil
}

// extractCodingIDs parses CPC2 output and extracts IDs of coding transcripts
func extractCodingIDs(cpc2Output, outputIDsFile string) error {
	lines, err := utils.ReadLines(cpc2Output)
	if err != nil {
		return err
	}

	var codingIDs []string
	// CPC2 output format: ID Length Peptide Fickett pI Integrity Coding_Prob Label
	// Label (last column) should be "coding"

	for i, line := range lines {
		// Skip header
		if i == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 8 {
			label := fields[7] // 8th column
			if label == "coding" {
				codingIDs = append(codingIDs, fields[0])
			}
		}
	}

	if len(codingIDs) == 0 {
		return fmt.Errorf("no coding transcripts found in CPC2 output")
	}

	return utils.WriteLines(outputIDsFile, codingIDs)
}

// countIntaRNAInteractions counts interactions in IntaRNA CSV output (skip header)
func countIntaRNAInteractions(outputFile string) (int, error) {
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
