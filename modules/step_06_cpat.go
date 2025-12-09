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

// Step05CPAT runs CPAT validation and creates consensus predictions
func Step06CPAT(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(6, "Cross-Validation with CPAT")

	// Create validation directory
	validationDir := filepath.Join(cfg.OutputDir, "07_validation")
	if err := utils.CreateDirs(validationDir); err != nil {
		return fmt.Errorf("failed to create validation directory: %w", err)
	}

	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")
	cpc2Output := filepath.Join(cpc2Dir, "cpc2_output.txt")

	// Check validation mode
	if cfg.ValidationMode == "cpc2-only" {
		utils.Info(fmt.Sprintf("Using CPC2-only validation (validation mode: %s)", cfg.ValidationMode))
		utils.ShowProgress("Skipping CPAT")

		// Extract CPC2 noncoding predictions
		if err := extractCPC2Noncoding(cpc2Output, validationDir); err != nil {
			return fmt.Errorf("failed to extract CPC2 predictions: %w", err)
		}

		// Copy CPC2 results as consensus
		cpc2Noncoding := filepath.Join(validationDir, "cpc2_noncoding.txt")
		consensusNoncoding := filepath.Join(validationDir, "consensus_noncoding.txt")
		if err := utils.CopyFile(cpc2Noncoding, consensusNoncoding); err != nil {
			return fmt.Errorf("failed to copy consensus file: %w", err)
		}

	} else if cfg.ValidationMode == "consensus" {
		utils.ShowProgress(fmt.Sprintf("Running CPAT for %s", cfg.CPATSpecies))
		utils.Info("Using bundled CPAT models",
			zap.String("hexamer", cfg.CPATHexamerFile),
			zap.String("logit", cfg.CPATLogitFile))

		// Run CPAT
		cpatOutput := filepath.Join(validationDir, "cpat_output")

		if err := utils.RunCommandInDir(ctx, validationDir, "cpat.py",
			"-x", cfg.CPATHexamerFile,
			"-d", cfg.CPATLogitFile,
			"-g", transcriptsFa,
			"-o", "cpat_output",
		); err != nil {
			return fmt.Errorf("CPAT failed: %w", err)
		}

		// Extract noncoding predictions from both tools
		if err := extractCPC2Noncoding(cpc2Output, validationDir); err != nil {
			return fmt.Errorf("failed to extract CPC2 predictions: %w", err)
		}

		cpatResultFile := cpatOutput + ".ORF_prob.best.tsv"
		if err := extractCPATNoncoding(cpatResultFile, validationDir); err != nil {
			return fmt.Errorf("failed to extract CPAT predictions: %w", err)
		}

		// Find consensus (intersection of both tools)
		utils.ShowProgress("Finding 2-way consensus (CPC2 + CPAT)")
		if err := findConsensus(validationDir); err != nil {
			return fmt.Errorf("failed to find consensus: %w", err)
		}
	}

	// Count results
	consensusFile := filepath.Join(validationDir, "consensus_noncoding.txt")
	consensusIDs, err := utils.ReadLines(consensusFile)
	if err != nil {
		return fmt.Errorf("failed to read consensus file: %w", err)
	}

	utils.StepComplete(5, "Cross-Validation with CPAT", stepStart)
	utils.Info(fmt.Sprintf("Found %d high-confidence lncRNAs", len(consensusIDs)),
		zap.String("mode", cfg.ValidationMode))

	return nil
}

// extractCPC2Noncoding extracts noncoding transcript IDs from CPC2 output
func extractCPC2Noncoding(cpc2File, outputDir string) error {
	lines, err := utils.ReadLines(cpc2File)
	if err != nil {
		return err
	}

	var noncoding []string
	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		fields := strings.Fields(line)
		if len(fields) >= 8 && fields[7] == "noncoding" {
			noncoding = append(noncoding, fields[0])
		}
	}

	// Sort for consistency with bash version
	// Note: Go's sort is different from bash sort, but we'll keep it simple
	outFile := filepath.Join(outputDir, "cpc2_noncoding.txt")
	return utils.WriteLines(outFile, noncoding)
}

// extractCPATNoncoding extracts noncoding transcript IDs from CPAT output
// CPAT threshold: coding_prob < 0.364
func extractCPATNoncoding(cpatFile, outputDir string) error {
	lines, err := utils.ReadLines(cpatFile)
	if err != nil {
		return err
	}

	var noncoding []string
	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		fields := strings.Fields(line)
		if len(fields) >= 11 {
			// Column 11 (index 10) is Coding_prob
			var codingProb float64
			fmt.Sscanf(fields[10], "%f", &codingProb)
			if codingProb < 0.364 {
				noncoding = append(noncoding, fields[0])
			}
		}
	}

	outFile := filepath.Join(outputDir, "cpat_noncoding.txt")
	return utils.WriteLines(outFile, noncoding)
}

// findConsensus finds intersection of CPC2 and CPAT predictions
func findConsensus(validationDir string) error {
	cpc2File := filepath.Join(validationDir, "cpc2_noncoding.txt")
	cpatFile := filepath.Join(validationDir, "cpat_noncoding.txt")
	consensusFile := filepath.Join(validationDir, "consensus_noncoding.txt")

	cpc2IDs, err := utils.ReadLines(cpc2File)
	if err != nil {
		return err
	}

	cpatIDs, err := utils.ReadLines(cpatFile)
	if err != nil {
		return err
	}

	// Create map for faster lookup
	cpatSet := make(map[string]bool)
	for _, id := range cpatIDs {
		cpatSet[id] = true
	}

	// Find intersection
	var consensus []string
	for _, id := range cpc2IDs {
		if cpatSet[id] {
			consensus = append(consensus, id)
		}
	}

	return utils.WriteLines(consensusFile, consensus)
}
