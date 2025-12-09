package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/regis-go/config"
	"github.com/yourusername/regis-go/utils"
	"go.uber.org/zap"
)

// Step10Consensus performs consensus analysis of LncTar and IntaRNA predictions
func Step11Consensus(ctx context.Context, cfg *config.Config) error {
	// Skip if neither tool is enabled
	if !cfg.EnableLncTar && !cfg.EnableIntaRNA {
		utils.Info("No target prediction tools enabled, skipping consensus")
		return nil
	}

	// Skip if only one tool is enabled
	if !cfg.EnableLncTar || !cfg.EnableIntaRNA {
		utils.Info("Consensus requires both LncTar and IntaRNA, skipping")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(11, "Cross-Tool Consensus Analysis")

	// Create directories
	// Bash: 10_target_prediction
	consensusDir := filepath.Join(cfg.OutputDir, "11_target_prediction")
	if err := utils.CreateDirs(consensusDir); err != nil {
		return fmt.Errorf("failed to create consensus directory: %w", err)
	}

	// Input directories (updated to match new structure)
	lnctarDir := filepath.Join(consensusDir, "lnctar")
	intarnaDir := filepath.Join(consensusDir, "intarna")

	// Determine which files to compare based on mode
	var lnctarFile, intarnaFile, consensusMode string

	if cfg.LncTarBestOnly && cfg.IntaRNABestOnly {
		lnctarFile = filepath.Join(lnctarDir, "best_candidates_targets.txt")
		intarnaFile = filepath.Join(intarnaDir, "best_candidates_targets.csv")
		consensusMode = "best_candidates"
	} else if cfg.LncTarComprehensive && cfg.IntaRNAComprehensive {
		lnctarFile = filepath.Join(lnctarDir, "all_lncrna_targets.txt")
		intarnaFile = filepath.Join(intarnaDir, "all_lncrna_targets.csv")
		consensusMode = "comprehensive"
	} else {
		lnctarFile = filepath.Join(lnctarDir, "highly_expressed_targets.txt")
		intarnaFile = filepath.Join(intarnaDir, "highly_expressed_targets.csv")
		consensusMode = "highly_expressed"
	}

	// Check if both files exist
	if !utils.FileExists(lnctarFile) || !utils.FileExists(intarnaFile) {
		utils.Warn("Both LncTar and IntaRNA output files required for consensus")
		return nil
	}

	utils.ShowProgress("Comparing predictions from LncTar and IntaRNA")

	// Extract lncRNA-target pairs from both tools
	lnctarPairs, err := extractLncTarPairs(lnctarFile)
	if err != nil {
		return fmt.Errorf("failed to extract LncTar pairs: %w", err)
	}

	intarnaPairs, err := extractIntaRNAPairs(intarnaFile)
	if err != nil {
		return fmt.Errorf("failed to extract IntaRNA pairs: %w", err)
	}

	// Find consensus (interactions predicted by both tools)
	consensusPairs := findConsensusPairs(lnctarPairs, intarnaPairs)

	// Output files
	lnctarPairsFile := filepath.Join(consensusDir, "lnctar_pairs.tmp")
	intarnaPairsFile := filepath.Join(consensusDir, "intarna_pairs.tmp")
	consensusPairsFile := filepath.Join(consensusDir, "consensus_pairs.txt")
	summaryFile := filepath.Join(consensusDir, "consensus_summary.txt")

	if err := writePairs(lnctarPairsFile, lnctarPairs); err != nil {
		return err
	}
	if err := writePairs(intarnaPairsFile, intarnaPairs); err != nil {
		return err
	}
	if err := writePairs(consensusPairsFile, consensusPairs); err != nil {
		return err
	}

	// Calculate statistics
	lnctarCount := len(lnctarPairs)
	intarnaCount := len(intarnaPairs)
	consensusCount := len(consensusPairs)

	var consensusPctLnctar, consensusPctIntarna float64
	if lnctarCount > 0 {
		consensusPctLnctar = float64(consensusCount) / float64(lnctarCount) * 100
	}
	if intarnaCount > 0 {
		consensusPctIntarna = float64(consensusCount) / float64(intarnaCount) * 100
	}

	// Count unique lncRNAs and targets in consensus
	var consensusLncs, consensusTargets int
	if consensusCount > 0 {
		consensusLncs = countUniqueLncRNAs(consensusPairs)
		consensusTargets = countUniqueTargets(consensusPairs)
	}

	// Generate consensus report
	report := generateConsensusReport(consensusMode, lnctarCount, intarnaCount, consensusCount,
		consensusPctLnctar, consensusPctIntarna, consensusLncs, consensusTargets)
	if err := utils.WriteLines(summaryFile, []string{report}); err != nil {
		return err
	}

	utils.StepComplete(10, "Consensus Target Analysis", stepStart)
	utils.Info("Consensus analysis complete",
		zap.Int("lnctar_pairs", lnctarCount),
		zap.Int("intarna_pairs", intarnaCount),
		zap.Int("consensus_pairs", consensusCount),
		zap.Float64("agreement_pct", consensusPctLnctar))

	return nil
}

// extractLncTarPairs extracts lncRNA-target pairs from LncTar output
// LncTar format: Query \t Length \t Target \t ...
func extractLncTarPairs(lnctarFile string) ([]string, error) {
	lines, err := utils.ReadLines(lnctarFile)
	if err != nil {
		return nil, err
	}

	pairSet := make(map[string]bool)
	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			pair := fields[0] + "\t" + fields[2]
			pairSet[pair] = true
		}
	}

	// Convert to sorted slice
	pairs := make([]string, 0, len(pairSet))
	for pair := range pairSet {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)

	return pairs, nil
}

// extractIntaRNAPairs extracts lncRNA-target pairs from IntaRNA CSV output
// IntaRNA format: id1;start1;end1;id2;start2;end2;...
func extractIntaRNAPairs(intarnaFile string) ([]string, error) {
	lines, err := utils.ReadLines(intarnaFile)
	if err != nil {
		return nil, err
	}

	pairSet := make(map[string]bool)
	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		fields := strings.Split(line, ";")
		if len(fields) >= 4 {
			pair := fields[0] + "\t" + fields[3]
			pairSet[pair] = true
		}
	}

	// Convert to sorted slice
	pairs := make([]string, 0, len(pairSet))
	for pair := range pairSet {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)

	return pairs, nil
}

// findConsensusPairs finds intersection of two sorted pair lists
func findConsensusPairs(lnctarPairs, intarnaPairs []string) []string {
	pairSet := make(map[string]bool)
	for _, pair := range intarnaPairs {
		pairSet[pair] = true
	}

	var consensus []string
	for _, pair := range lnctarPairs {
		if pairSet[pair] {
			consensus = append(consensus, pair)
		}
	}

	return consensus
}

// writePairs writes pairs to file
func writePairs(filename string, pairs []string) error {
	return utils.WriteLines(filename, pairs)
}

// countUniqueLncRNAs counts unique lncRNAs in pairs
func countUniqueLncRNAs(pairs []string) int {
	lncSet := make(map[string]bool)
	for _, pair := range pairs {
		fields := strings.Split(pair, "\t")
		if len(fields) >= 1 {
			lncSet[fields[0]] = true
		}
	}
	return len(lncSet)
}

// countUniqueTargets counts unique targets in pairs
func countUniqueTargets(pairs []string) int {
	targetSet := make(map[string]bool)
	for _, pair := range pairs {
		fields := strings.Split(pair, "\t")
		if len(fields) >= 2 {
			targetSet[fields[1]] = true
		}
	}
	return len(targetSet)
}

// generateConsensusReport generates a formatted consensus report
func generateConsensusReport(mode string, lnctarCount, intarnaCount, consensusCount int,
	consensusPctLnctar, consensusPctIntarna float64, consensusLncs, consensusTargets int) string {

	var report strings.Builder

	report.WriteString("╔════════════════════════════════════════════════════════════════════╗\n")
	report.WriteString("║          CONSENSUS TARGET PREDICTION REPORT                        ║\n")
	report.WriteString("║          Cross-Validation: LncTar + IntaRNA                        ║\n")
	report.WriteString("╚════════════════════════════════════════════════════════════════════╝\n")
	report.WriteString("\n")
	report.WriteString(fmt.Sprintf("Mode: %s\n", mode))
	report.WriteString("\n")
	report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	report.WriteString("TOOL COMPARISON\n")
	report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	report.WriteString("\n")
	report.WriteString("Individual Tool Predictions:\n")
	report.WriteString(fmt.Sprintf("  • LncTar:  %d unique lncRNA-target pairs\n", lnctarCount))
	report.WriteString(fmt.Sprintf("  • IntaRNA: %d unique lncRNA-target pairs\n", intarnaCount))
	report.WriteString("\n")
	report.WriteString("Consensus Predictions (both tools agree):\n")
	report.WriteString(fmt.Sprintf("  • High-confidence pairs: %d\n", consensusCount))
	report.WriteString(fmt.Sprintf("  • Agreement with LncTar:  %.1f%%\n", consensusPctLnctar))
	report.WriteString(fmt.Sprintf("  • Agreement with IntaRNA: %.1f%%\n", consensusPctIntarna))
	report.WriteString("\n")

	if consensusCount > 0 {
		report.WriteString("Consensus Statistics:\n")
		report.WriteString(fmt.Sprintf("  • Unique lncRNAs: %d\n", consensusLncs))
		report.WriteString(fmt.Sprintf("  • Unique targets: %d\n", consensusTargets))
		report.WriteString("\n")
	}

	report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	report.WriteString("INTERPRETATION\n")
	report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	report.WriteString("\n")
	report.WriteString("Consensus predictions represent interactions identified by BOTH tools.\n")
	report.WriteString("These high-confidence predictions have been validated by two independent\n")
	report.WriteString("algorithms and are more likely to represent true biological interactions.\n")

	return report.String()
}
