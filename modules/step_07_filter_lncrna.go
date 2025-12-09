package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step06FilterLncRNA filters lncRNAs by length/probability, classifies them, and quantifies expression
func Step07FilterLncRNA(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(7, "Processing GTF and Filtering lncRNAs")

	// Create directories
	gtfDir := filepath.Join(cfg.OutputDir, "08_lncrna_analysis")
	filteredDir := filepath.Join(gtfDir, "filtered")
	comparisonDir := filepath.Join(gtfDir, "comparison")
	intermediateDir := filepath.Join(gtfDir, "intermediate")
	novelDir := filepath.Join(gtfDir, "novel_lncrnas")
	expressionDir := filepath.Join(gtfDir, "expression")

	if err := utils.CreateDirs(gtfDir, filteredDir, comparisonDir, intermediateDir, novelDir, expressionDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	validationDir := filepath.Join(cfg.OutputDir, "07_validation")
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	assemblyDir := filepath.Join(cfg.OutputDir, "05_assembly")
	alignmentDir := filepath.Join(cfg.OutputDir, "04_alignment")

	// Step 1: Filter lncRNAs by consensus and length/probability
	utils.ShowProgress("Filtering lncRNAs by validation and quality criteria")
	if err := filterLncRNAs(validationDir, cpc2Dir, intermediateDir, filteredDir); err != nil {
		return fmt.Errorf("failed to filter lncRNAs: %w", err)
	}

	// Step 2: Extract lncRNA sequences using seqkit
	utils.ShowProgress("Extracting lncRNA sequences with seqkit")
	filteredIDsFile := filepath.Join(intermediateDir, "lncrna_filtered_ids.txt")
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")
	lncRNAFa := filepath.Join(filteredDir, "lncrna_filtered.fa")

	if err := utils.RunCommand(ctx, "seqkit", "grep",
		"-f", filteredIDsFile,
		transcriptsFa,
		"-o", lncRNAFa,
	); err != nil {
		return fmt.Errorf("seqkit failed: %w", err)
	}

	// Step 3: Extract lncRNA GTF (only for reference mode)
	transcriptsGtf := filepath.Join(assemblyDir, "transcripts.gtf")
	lncRNAGtf := filepath.Join(filteredDir, "lncrna.gtf")

	if utils.FileExists(transcriptsGtf) {
		if err := extractLncRNAGTF(filteredIDsFile, transcriptsGtf, lncRNAGtf); err != nil {
			return fmt.Errorf("failed to extract lncRNA GTF: %w", err)
		}

		// Step 4: Compare with reference using gffcompare (reference mode only)
		if cfg.Method == "reference" {
			utils.ShowProgress("Comparing lncRNAs with reference annotations")
			if err := runGffcompare(ctx, cfg.GTF, lncRNAGtf, comparisonDir); err != nil {
				utils.Warn("gffcompare failed, skipping classification", zap.Error(err))
			} else {
				// Extract novel lncRNAs by class code
				if err := classifyLncRNAs(comparisonDir, novelDir, lncRNAFa); err != nil {
					utils.Warn("lncRNA classification failed", zap.Error(err))
				}
			}

			// Convert GTF to BED for visualization and enrichment
			if err := convertGTFToBED(ctx, lncRNAGtf, filteredDir, intermediateDir); err != nil {
				utils.Warn("GTF to BED conversion failed", zap.Error(err))
			}
		}

		// Step 5: Expression analysis with featureCounts (reference mode only)
		if cfg.Method == "reference" {
			bamFile := filepath.Join(alignmentDir, "aligned_sorted.bam")
			if utils.FileExists(bamFile) {
				utils.ShowProgress("Quantifying lncRNA expression")
				if err := quantifyExpression(ctx, cfg, lncRNAGtf, bamFile, expressionDir, novelDir, lncRNAFa); err != nil {
					utils.Warn("Expression quantification failed", zap.Error(err))
				}
			}
		}
	}

	utils.StepComplete(6, "Processing GTF and Filtering lncRNAs", stepStart)
	utils.Info("lncRNA filtering complete", zap.String("output", filteredDir))

	return nil
}

// filterLncRNAs filters lncRNAs based on consensus and quality criteria
func filterLncRNAs(validationDir, cpc2Dir, intermediateDir, filteredDir string) error {
	consensusFile := filepath.Join(validationDir, "consensus_noncoding.txt")
	cpc2Output := filepath.Join(cpc2Dir, "cpc2_output.txt")

	if utils.FileExists(consensusFile) {
		// Use consensus predictions
		utils.Info("Using multi-tool consensus predictions for filtering")

		// Read consensus IDs
		consensusIDs, err := utils.ReadLines(consensusFile)
		if err != nil {
			return err
		}

		// Filter CPC2 output by length (>200) and probability (<0.5)
		cpc2Lines, err := utils.ReadLines(cpc2Output)
		if err != nil {
			return err
		}

		var filtered []string
		var filteredIDs []string
		filtered = append(filtered, cpc2Lines[0]) // Add header

		for i, line := range cpc2Lines {
			if i == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				id := fields[0]
				length, _ := strconv.Atoi(fields[1])
				prob, _ := strconv.ParseFloat(fields[6], 64)

				if fields[7] == "noncoding" && length > 200 && prob < 0.5 {
					// Check if in consensus
					for _, cid := range consensusIDs {
						if id == cid {
							filtered = append(filtered, line)
							filteredIDs = append(filteredIDs, id)
							break
						}
					}
				}
			}
		}

		// Write filtered results
		if err := utils.WriteLines(filepath.Join(filteredDir, "lncrna_filtered.txt"), filtered); err != nil {
			return err
		}
		if err := utils.WriteLines(filepath.Join(intermediateDir, "lncrna_filtered_ids.txt"), filteredIDs); err != nil {
			return err
		}

		utils.Info(fmt.Sprintf("Filtered to %d high-confidence lncRNAs (>200nt, prob<0.5, validated)", len(filteredIDs)))

	} else {
		// CPC2-only filtering
		utils.Info("Using CPC2-only predictions for filtering")

		cpc2Lines, err := utils.ReadLines(cpc2Output)
		if err != nil {
			return err
		}

		var filtered []string
		var filteredIDs []string
		filtered = append(filtered, cpc2Lines[0]) // Add header

		for i, line := range cpc2Lines {
			if i == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				length, _ := strconv.Atoi(fields[1])
				prob, _ := strconv.ParseFloat(fields[6], 64)

				if fields[7] == "noncoding" && length > 200 && prob < 0.5 {
					filtered = append(filtered, line)
					filteredIDs = append(filteredIDs, fields[0])
				}
			}
		}

		if err := utils.WriteLines(filepath.Join(filteredDir, "lncrna_filtered.txt"), filtered); err != nil {
			return err
		}
		if err := utils.WriteLines(filepath.Join(intermediateDir, "lncrna_filtered_ids.txt"), filteredIDs); err != nil {
			return err
		}

		utils.Info(fmt.Sprintf("Filtered to %d high-confidence lncRNAs (>200nt, prob<0.5)", len(filteredIDs)))
	}

	return nil
}

// extractLncRNAGTF extracts lncRNA entries from GTF file
func extractLncRNAGTF(idsFile, inputGTF, outputGTF string) error {
	ids, err := utils.ReadLines(idsFile)
	if err != nil {
		return err
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	gtfLines, err := utils.ReadLines(inputGTF)
	if err != nil {
		return err
	}

	var lncRNALines []string
	for _, line := range gtfLines {
		// Check if line contains any of the lncRNA IDs
		for id := range idSet {
			if strings.Contains(line, id) {
				lncRNALines = append(lncRNALines, line)
				break
			}
		}
	}

	return utils.WriteLines(outputGTF, lncRNALines)
}

// runGffcompare runs gffcompare to classify lncRNAs
func runGffcompare(ctx context.Context, refGTF, lncRNAGTF, outputDir string) error {
	outputPrefix := filepath.Join(outputDir, "gffcompare")

	return utils.RunCommand(ctx, "gffcompare",
		"-r", refGTF,
		"-o", outputPrefix,
		lncRNAGTF,
	)
}

// classifyLncRNAs extracts different classes of lncRNAs
func classifyLncRNAs(comparisonDir, novelDir, lncRNAFa string) error {
	annotatedGTF := filepath.Join(comparisonDir, "gffcompare.annotated.gtf")
	if !utils.FileExists(annotatedGTF) {
		return fmt.Errorf("annotated GTF not found")
	}

	utils.ShowProgress("Extracting lncRNAs by classification")

	// First, count ALL class codes (like bash version)
	if err := countAllClassCodes(annotatedGTF, novelDir); err != nil {
		utils.Warn("Failed to count class codes", zap.Error(err))
	}

	// Then extract specific class codes
	classes := map[string]string{
		"u": "novel_intergenic",
		"x": "antisense",
		"i": "intronic",
		"=": "known",
	}

	for code, name := range classes {
		if err := extractClassCode(annotatedGTF, code, novelDir, name, lncRNAFa); err != nil {
			utils.Warn(fmt.Sprintf("Failed to extract class %s", code), zap.Error(err))
		}
	}

	return nil
}

// countAllClassCodes counts and reports all gffcompare class codes
func countAllClassCodes(annotatedGTF, novelDir string) error {
	lines, err := utils.ReadLines(annotatedGTF)
	if err != nil {
		return err
	}

	// Count each class code
	classCounts := make(map[string]int)
	for _, line := range lines {
		if strings.Contains(line, "class_code") {
			// Extract class code: class_code "X"
			if idx := strings.Index(line, `class_code "`); idx != -1 {
				start := idx + 12 // len(`class_code "`)
				if start < len(line) {
					code := string(line[start])
					classCounts[code]++
				}
			}
		}
	}

	// Sort by count (descending)
	type classCount struct {
		code  string
		count int
	}
	var counts []classCount
	for code, count := range classCounts {
		counts = append(counts, classCount{code, count})
	}
	// Sort by count descending
	for i := 0; i < len(counts); i++ {
		for j := i + 1; j < len(counts); j++ {
			if counts[j].count > counts[i].count {
				counts[i], counts[j] = counts[j], counts[i]
			}
		}
	}

	// Write class code counts file
	countsFile := filepath.Join(novelDir, "class_code_counts.txt")
	var countLines []string
	for _, cc := range counts {
		countLines = append(countLines, fmt.Sprintf("%d %s", cc.count, cc.code))
	}
	if err := utils.WriteLines(countsFile, countLines); err != nil {
		return err
	}

	// Display detailed classification
	utils.Info("📋 Detailed Classification (gffcompare class codes):")
	for _, cc := range counts {
		desc := getClassCodeDescription(cc.code)
		utils.Info(fmt.Sprintf("  • Class '%s': %d - %s", cc.code, cc.count, desc))
	}

	return nil
}

// getClassCodeDescription returns human-readable description for class code
func getClassCodeDescription(code string) string {
	descriptions := map[string]string{
		"=": "Complete match (known lncRNA)",
		"c": "Contained in reference exon",
		"j": "Potentially novel isoform (at least one splice junction shared)",
		"e": "Single exon transfrag overlapping reference exon",
		"i": "Intronic (fully contained within reference intron)",
		"o": "Generic exonic overlap",
		"p": "Possible polymerase run-on",
		"r": "Repeat (soft-masked)",
		"u": "Intergenic (novel, unknown)",
		"x": "Antisense (exonic overlap on opposite strand)",
		"s": "Antisense intronic",
		"m": "Multi-exon with retained intron(s)",
		"n": "Retained intron(s), all introns matched",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Other"
}

// extractClassCode extracts lncRNAs with specific class code
func extractClassCode(annotatedGTF, classCode, outputDir, name, lncRNAFa string) error {
	lines, err := utils.ReadLines(annotatedGTF)
	if err != nil {
		return err
	}

	searchStr := fmt.Sprintf("class_code \"%s\"", classCode)
	var gtfLines []string
	var ids []string

	for _, line := range lines {
		if strings.Contains(line, searchStr) {
			gtfLines = append(gtfLines, line)

			// Extract transcript_id
			if idx := strings.Index(line, "transcript_id \""); idx != -1 {
				start := idx + 15
				if end := strings.Index(line[start:], "\""); end != -1 {
					ids = append(ids, line[start:start+end])
				}
			}
		}
	}

	// Write GTF
	gtfFile := filepath.Join(outputDir, name+".gtf")
	if err := utils.WriteLines(gtfFile, gtfLines); err != nil {
		return err
	}

	// Write IDs
	idsFile := filepath.Join(outputDir, name+"_ids.txt")
	if err := utils.WriteLines(idsFile, ids); err != nil {
		return err
	}

	utils.Info(fmt.Sprintf("Found %d %s lncRNAs", len(ids), name))

	return nil
}

// quantifyExpression runs featureCounts and filters by expression
func quantifyExpression(ctx context.Context, cfg *config.Config, gtfFile, bamFile, expressionDir, novelDir, lncRNAFa string) error {
	countsFile := filepath.Join(expressionDir, "lncrna_counts.txt")

	// Build featureCounts command
	args := []string{
		"-T", strconv.Itoa(cfg.Threads),
		"-t", "exon",
		"-g", "transcript_id",
		"-a", gtfFile,
		"-o", countsFile,
	}

	if cfg.DataType == "paired" {
		args = append(args, "-p")
	}

	args = append(args, bamFile)

	if err := utils.RunCommand(ctx, "featureCounts", args...); err != nil {
		return err
	}

	// Parse counts and filter by expression
	return parseAndFilterCounts(countsFile, expressionDir, novelDir, lncRNAFa)
}

// parseAndFilterCounts parses featureCounts output and filters by expression level
func parseAndFilterCounts(countsFile, expressionDir, novelDir, lncRNAFa string) error {
	lines, err := utils.ReadLines(countsFile)
	if err != nil {
		return err
	}

	// Skip first 2 lines (header and summary)
	var simpleCounts []string
	var highlyExpressed []string
	var expressed []string

	for i, line := range lines {
		if i < 2 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 7 {
			id := fields[0]
			count := fields[6]
			simpleCounts = append(simpleCounts, fmt.Sprintf("%s\t%s", id, count))

			countVal, _ := strconv.Atoi(count)
			if countVal >= 10 {
				highlyExpressed = append(highlyExpressed, id)
			}
			if countVal >= 5 {
				expressed = append(expressed, id)
			}
		}
	}

	// Write files
	if err := utils.WriteLines(filepath.Join(expressionDir, "lncrna_counts_simple.txt"), simpleCounts); err != nil {
		return err
	}
	if err := utils.WriteLines(filepath.Join(expressionDir, "highly_expressed_ids.txt"), highlyExpressed); err != nil {
		return err
	}
	if err := utils.WriteLines(filepath.Join(expressionDir, "expressed_ids.txt"), expressed); err != nil {
		return err
	}

	utils.Info(fmt.Sprintf("Highly expressed (≥10 reads): %d", len(highlyExpressed)))
	utils.Info(fmt.Sprintf("Expressed (≥5 reads): %d", len(expressed)))

	// Extract highly expressed sequences
	if len(highlyExpressed) > 0 {
		highlyExpressedFa := filepath.Join(expressionDir, "highly_expressed.fa")
		highlyExpressedIdsFile := filepath.Join(expressionDir, "highly_expressed_ids.txt")

		// Use seqkit to extract sequences
		if err := utils.RunCommand(context.Background(), "seqkit", "grep",
			"-f", highlyExpressedIdsFile,
			lncRNAFa,
			"-o", highlyExpressedFa,
		); err != nil {
			utils.Warn("Failed to extract highly expressed sequences", zap.Error(err))
		}
	}

	// Find best candidates (highly expressed + novel intergenic)
	novelIntergenicFile := filepath.Join(novelDir, "novel_intergenic_ids.txt")
	if utils.FileExists(novelIntergenicFile) && len(highlyExpressed) > 0 {
		novelIDs, err := utils.ReadLines(novelIntergenicFile)
		if err == nil {
			// Find intersection
			novelSet := make(map[string]bool)
			for _, id := range novelIDs {
				novelSet[id] = true
			}

			var bestCandidates []string
			for _, id := range highlyExpressed {
				if novelSet[id] {
					bestCandidates = append(bestCandidates, id)
				}
			}

			if len(bestCandidates) > 0 {
				bestCandidatesFile := filepath.Join(expressionDir, "best_candidates.txt")
				if err := utils.WriteLines(bestCandidatesFile, bestCandidates); err != nil {
					utils.Warn("Failed to write best candidates", zap.Error(err))
				} else {
					utils.Info(fmt.Sprintf("🎯 Best candidates (highly expressed + novel): %d", len(bestCandidates)))
				}
			}
		}
	}

	return nil
}

// convertGTFToBED converts GTF to BED format using UCSC tools
func convertGTFToBED(ctx context.Context, gtfFile, outputDir, intermediateDir string) error {
	genePredFile := filepath.Join(intermediateDir, "lncrna.genepred")
	bedFile := filepath.Join(outputDir, "lncrna.bed")

	// Step 1: GTF to genePred
	if err := utils.RunCommand(ctx, "gtfToGenePred", gtfFile, genePredFile); err != nil {
		return fmt.Errorf("gtfToGenePred failed: %w", err)
	}

	// Step 2: genePred to BED
	if err := utils.RunCommand(ctx, "genePredToBed", genePredFile, bedFile); err != nil {
		return fmt.Errorf("genePredToBed failed: %w", err)
	}

	// Clean up intermediate file
	utils.RunCommand(ctx, "rm", "-f", genePredFile)

	return nil
}
