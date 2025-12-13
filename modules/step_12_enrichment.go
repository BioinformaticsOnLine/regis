package modules

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step11Enrichment builds gene lists for enrichment analysis
// This implements the build_enrichment_gene_lists() function from regis.sh
func Step12Enrichment(ctx context.Context, cfg *config.Config) error {
	// Skip for de novo mode
	if cfg.Method != "reference" {
		utils.Info("Enrichment requires reference mode, skipping")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(12, "Building Enrichment Gene Lists")

	// Create directories
	// Create directories
	// Bash: 11_enrichment
	enrichmentDir := filepath.Join(cfg.OutputDir, "12_enrichment")
	enrichmentIntermediateDir := filepath.Join(enrichmentDir, "intermediate")
	if err := utils.CreateDirs(enrichmentDir, enrichmentIntermediateDir); err != nil {
		return fmt.Errorf("failed to create enrichment directories: %w", err)
	}

	utils.ShowProgress("Building enrichment gene lists")

	gtfDir := filepath.Join(cfg.OutputDir, "08_lncrna_analysis")
	filteredDir := filepath.Join(gtfDir, "filtered")
	lncRNABed := filepath.Join(filteredDir, "lncrna.bed")
	expressionDir := filepath.Join(gtfDir, "expression")
	bestCandidatesFile := filepath.Join(expressionDir, "best_candidates.txt")
	assemblyDir := filepath.Join(cfg.OutputDir, "05_assembly")

	// Updated paths for target prediction tools
	lnctarDir := filepath.Join(cfg.OutputDir, "11_target_prediction", "lnctar")
	intarnaDir := filepath.Join(cfg.OutputDir, "11_target_prediction", "intarna")
	targetPredictionDir := filepath.Join(cfg.OutputDir, "11_target_prediction")

	// Check if required files exist
	if !utils.FileExists(lncRNABed) {
		utils.Warn("lncRNA BED file not found, skipping enrichment")
		return nil
	}

	// Step 1: Build gene BED from GTF
	genesBed := filepath.Join(enrichmentIntermediateDir, "genes_for_enrichment.bed")
	if err := buildGeneBedFromGTF(ctx, cfg.GTF, genesBed); err != nil {
		utils.Warn("Failed to build gene BED", zap.Error(err))
		return nil
	}

	if !utils.FileExists(genesBed) {
		utils.Warn("No gene features could be parsed from annotation")
		return nil
	}

	// Step 2: Focus on best lncRNA candidates when available
	lncBestBed := filepath.Join(enrichmentIntermediateDir, "lncrna_best_candidates.bed")
	if utils.FileExists(bestCandidatesFile) {
		// grep -Ff best_candidates.txt lncrna.bed > lncrna_best_candidates.bed
		if err := filterBestCandidates(bestCandidatesFile, lncRNABed, lncBestBed); err != nil {
			utils.Warn("Failed to filter best candidates", zap.Error(err))
		}
	}
	// If no best candidates, use all lncRNAs
	if !utils.FileExists(lncBestBed) {
		utils.CopyFile(lncRNABed, lncBestBed)
	}

	// Step 3: Map each lncRNA to nearest gene using bedtools
	// Sort both BED files first to avoid chromosome ordering issues
	nearestGenesBed := filepath.Join(enrichmentIntermediateDir, "lncrna_nearest_genes.bed")
	lncBestBedSorted := filepath.Join(enrichmentIntermediateDir, "lncrna_best_candidates_sorted.bed")
	genesBedSorted := filepath.Join(enrichmentIntermediateDir, "genes_for_enrichment_sorted.bed")

	// Sort BED files by chromosome and position
	sortCmd1 := fmt.Sprintf("sort -k1,1 -k2,2n \"%s\" > \"%s\"", lncBestBed, lncBestBedSorted)
	sortCmd2 := fmt.Sprintf("sort -k1,1 -k2,2n \"%s\" > \"%s\"", genesBed, genesBedSorted)

	if err := utils.RunCommand(ctx, "bash", "-c", sortCmd1); err != nil {
		utils.Warn("Failed to sort lncRNA BED", zap.Error(err))
		return nil
	}
	if err := utils.RunCommand(ctx, "bash", "-c", sortCmd2); err != nil {
		utils.Warn("Failed to sort genes BED", zap.Error(err))
		return nil
	}

	// Run bedtools closest with sorted files
	bedtoolsCmd := fmt.Sprintf("bedtools closest -a \"%s\" -b \"%s\" -d > \"%s\" 2>&1 || true", lncBestBedSorted, genesBedSorted, nearestGenesBed)
	if err := utils.RunCommand(ctx, "bash", "-c", bedtoolsCmd); err != nil {
		utils.Warn("bedtools closest failed", zap.Error(err))
		return nil
	}

	if !utils.FileExists(nearestGenesBed) {
		utils.Warn("No nearest gene assignments could be made")
		return nil
	}



	// Step 4: Extract genes near lncRNAs
	genesNearLncRNAsIntermediate := filepath.Join(enrichmentIntermediateDir, "genes_near_lncRNAs.txt")
	genesNearLncRNAs := filepath.Join(enrichmentDir, "genes_near_lncRNAs_unique.txt")
	if err := extractNearestGenes(nearestGenesBed, genesNearLncRNAsIntermediate, genesNearLncRNAs); err != nil {
		utils.Warn("Failed to extract nearest genes", zap.Error(err))
	}

	// Step 5: Extract target genes from LncTar if available
	genesFromLncTar := filepath.Join(enrichmentDir, "genes_from_lnctar_mapped.txt")
	if cfg.EnableLncTar {
		if err := extractLncTarTargetGenes(ctx, lnctarDir, assemblyDir, gtfDir, enrichmentIntermediateDir, genesFromLncTar); err != nil {
			utils.Warn("Failed to extract LncTar target genes", zap.Error(err))
		}
	}

	// Step 6: Extract target genes from IntaRNA if available
	genesFromIntaRNA := filepath.Join(enrichmentDir, "genes_from_intarna_mapped.txt")
	if cfg.EnableIntaRNA {
		if err := extractIntaRNATargetGenes(ctx, intarnaDir, assemblyDir, gtfDir, enrichmentIntermediateDir, genesFromIntaRNA); err != nil {
			utils.Warn("Failed to extract IntaRNA target genes", zap.Error(err))
		}
	}

	// Step 7: Extract consensus target genes if available
	genesFromConsensus := filepath.Join(enrichmentDir, "genes_from_consensus_mapped.txt")
	consensusFile := filepath.Join(targetPredictionDir, "consensus_pairs.txt")
	if cfg.EnableLncTar && cfg.EnableIntaRNA && utils.FileExists(consensusFile) {
		if err := extractConsensusTargetGenes(ctx, consensusFile, assemblyDir, gtfDir, enrichmentIntermediateDir, genesFromConsensus); err != nil {
			utils.Warn("Failed to extract consensus target genes", zap.Error(err))
		}
	}

	// Step 8: Extract all genes from reference GTF (background for enrichment)
	backgroundGenes := filepath.Join(enrichmentDir, "all_genes_background.txt")
	if err := extractAllGenesFromGTF(ctx, cfg.GTF, backgroundGenes); err != nil {
		utils.Warn("Failed to extract background genes", zap.Error(err))
	}

	// Step 9: Combine all gene sources
	combinedGenes := filepath.Join(enrichmentDir, "genes_associated_with_lncRNAs_combined.txt")
	if err := combineGeneLists(genesNearLncRNAs, genesFromLncTar, genesFromIntaRNA, genesFromConsensus, combinedGenes); err != nil {
		utils.Warn("Failed to combine gene lists", zap.Error(err))
	}

	// Print summary
	backgroundCount := countLines(backgroundGenes)
	nearCount := countLines(genesNearLncRNAs)
	lnctarCount := countLines(genesFromLncTar)
	intarnaCount := countLines(genesFromIntaRNA)
	consensusCount := countLines(genesFromConsensus)
	combinedCount := countLines(combinedGenes)

	utils.Info(fmt.Sprintf("✓ Background genes (all): %d", backgroundCount))
	utils.Info(fmt.Sprintf("✓ Nearest genes (bedtools): %d", nearCount))
	if lnctarCount > 0 {
		utils.Info(fmt.Sprintf("✓ LncTar target genes: %d", lnctarCount))
	}
	if intarnaCount > 0 {
		utils.Info(fmt.Sprintf("✓ IntaRNA target genes: %d", intarnaCount))
	}
	if consensusCount > 0 {
		utils.Info(fmt.Sprintf("⭐ Consensus target genes (high-confidence): %d", consensusCount))
	}
	utils.Info(fmt.Sprintf("✓ Combined gene list: %d unique genes", combinedCount))

	utils.StepComplete(11, "Enrichment Gene Lists", stepStart)
	utils.Info("📊 For getENRICH:")
	utils.Info("  Background genes", zap.String("file", backgroundGenes))
	utils.Info("  Genes of interest", zap.String("file", combinedGenes))
	if consensusCount > 0 {
		utils.Info("⭐ HIGH-CONFIDENCE", zap.String("file", genesFromConsensus))
	}

	return nil
}

// filterBestCandidates filters BED file to keep only lines matching IDs in filter file
// Implements: grep -Ff best_candidates.txt lncrna.bed > output.bed
func filterBestCandidates(filterFile, inputBed, outputBed string) error {
	// Read filter IDs
	filterIDs, err := utils.ReadLines(filterFile)
	if err != nil {
		return err
	}

	// Build set of IDs to keep
	idSet := make(map[string]bool)
	for _, id := range filterIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			idSet[id] = true
		}
	}

	// Read input BED and filter
	inputLines, err := utils.ReadLines(inputBed)
	if err != nil {
		return err
	}

	var outputLines []string
	for _, line := range inputLines {
		// BED format: column 4 is the ID
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			if idSet[fields[3]] {
				outputLines = append(outputLines, line)
			}
		}
	}

	return utils.WriteLines(outputBed, outputLines)
}

// buildGeneBedFromGTF builds a BED file of genes from GTF using native Go parsing
func buildGeneBedFromGTF(ctx context.Context, gtfFile, outputBed string) error {
	file, err := os.Open(gtfFile)
	if err != nil {
		return err
	}
	defer file.Close()

	out, err := os.Create(outputBed)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := bufio.NewWriter(out)
	defer writer.Flush()

	scanner := bufio.NewScanner(file)
	// Use a large buffer for long lines often found in GFF
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	seen := make(map[string]bool)
	seenGenes := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 9 {
			continue
		}

		// Filter for gene features
		featureType := fields[2]
		if featureType != "gene" && featureType != "transcript" && featureType != "mRNA" {
			continue
		}

		// Avoid processing exact duplicate lines
		if seen[line] {
			continue
		}
		seen[line] = true

		// Parse attributes to find ID/Name
		attributes := fields[8]
		geneID := parseGeneID(attributes)

		if geneID != "" && !seenGenes[geneID] {
			seenGenes[geneID] = true
			// Write BED: chr, start-1, end, name, score, strand
			// GFF is 1-based, BED is 0-based start
			start, _ := strconv.Atoi(fields[3])
			bedStart := start - 1
			end := fields[4]
			strand := fields[6]
			fmt.Fprintf(writer, "%s\t%d\t%s\t%s\t.\t%s\n", fields[0], bedStart, end, geneID, strand)
		}
	}

	return scanner.Err()
}

// extractNearestGenes extracts unique genes from bedtools closest output
func extractNearestGenes(inputBed, intermediateFile, outputFile string) error {
	// awk 'BEGIN{OFS="\t"} $16!="." {print $4,$16,$NF}' > intermediate
	// awk '$2!="." {print $2}' | sort -u > output

	// Read bedtools output
	lines, err := utils.ReadLines(inputBed)
	if err != nil {
		return err
	}

	// Extract gene names (column 16 in bedtools closest output, 0-indexed: 15)
	geneSet := make(map[string]bool)
	var intermediateLines []string

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 16 && fields[15] != "." {
			// Column 4 (lncRNA), column 16 (gene), last column (distance)
			lncRNA := fields[3]
			gene := fields[15]
			distance := fields[len(fields)-1]
			intermediateLines = append(intermediateLines, fmt.Sprintf("%s\t%s\t%s", lncRNA, gene, distance))
			geneSet[gene] = true
		}
	}

	// Write intermediate file
	if err := utils.WriteLines(intermediateFile, intermediateLines); err != nil {
		return err
	}

	// Write unique genes
	var genes []string
	for gene := range geneSet {
		genes = append(genes, gene)
	}

	return utils.WriteLines(outputFile, genes)
}

// extractLncTarTargetGenes extracts and maps LncTar target genes
func extractLncTarTargetGenes(ctx context.Context, lnctarDir, assemblyDir, gtfDir, intermediateDir, outputFile string) error {
	// Collect all LncTar target transcript IDs
	lnctarTargetsCombined := filepath.Join(intermediateDir, "genes_from_lnctar_targets.txt")
	var allTargets []string

	// Check all possible LncTar output files
	lnctarFiles := []string{
		filepath.Join(lnctarDir, "best_candidates_targets.txt"),
		filepath.Join(lnctarDir, "highly_expressed_targets.txt"),
		filepath.Join(lnctarDir, "all_lncrna_targets.txt"),
	}

	for _, lnctarFile := range lnctarFiles {
		if utils.FileExists(lnctarFile) {
			// tail -n +2 (skip header) | awk '{print $3}' (column 3 is target)
			lines, err := utils.ReadLines(lnctarFile)
			if err != nil {
				continue
			}
			for i, line := range lines {
				if i == 0 {
					continue // Skip header
				}
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					allTargets = append(allTargets, fields[2])
				}
			}
		}
	}

	if len(allTargets) == 0 {
		return nil
	}

	// Write combined targets
	utils.WriteLines(lnctarTargetsCombined, allTargets)

	// Map transcript IDs to gene IDs
	return mapTranscriptsToGenes(ctx, lnctarTargetsCombined, assemblyDir, gtfDir, intermediateDir, outputFile, "lnctar")
}

// extractIntaRNATargetGenes extracts and maps IntaRNA target genes
func extractIntaRNATargetGenes(ctx context.Context, intarnaDir, assemblyDir, gtfDir, intermediateDir, outputFile string) error {
	// Collect all IntaRNA target transcript IDs
	intarnaTargetsCombined := filepath.Join(intermediateDir, "genes_from_intarna_targets.txt")
	var allTargets []string

	// Check all possible IntaRNA output files
	intarnaFiles := []string{
		filepath.Join(intarnaDir, "best_candidates_targets.csv"),
		filepath.Join(intarnaDir, "highly_expressed_targets.csv"),
		filepath.Join(intarnaDir, "all_lncrna_targets.csv"),
	}

	for _, intarnaFile := range intarnaFiles {
		if utils.FileExists(intarnaFile) {
			// tail -n +2 (skip header) | awk -F';' '{print $4}' (column 4 is target)
			lines, err := utils.ReadLines(intarnaFile)
			if err != nil {
				continue
			}
			for i, line := range lines {
				if i == 0 {
					continue // Skip header
				}
				fields := strings.Split(line, ";")
				if len(fields) >= 4 {
					allTargets = append(allTargets, fields[3])
				}
			}
		}
	}

	if len(allTargets) == 0 {
		return nil
	}

	// Write combined targets
	utils.WriteLines(intarnaTargetsCombined, allTargets)

	// Map transcript IDs to gene IDs
	return mapTranscriptsToGenes(ctx, intarnaTargetsCombined, assemblyDir, gtfDir, intermediateDir, outputFile, "intarna")
}

// extractConsensusTargetGenes extracts and maps consensus target genes
func extractConsensusTargetGenes(ctx context.Context, consensusFile, assemblyDir, gtfDir, intermediateDir, outputFile string) error {
	// Extract target IDs from consensus file (column 2)
	consensusTargetsTmp := filepath.Join(intermediateDir, "consensus_targets_tmp.txt")

	lines, err := utils.ReadLines(consensusFile)
	if err != nil {
		return err
	}

	var targets []string
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 {
			targets = append(targets, fields[1])
		}
	}

	if len(targets) == 0 {
		return nil
	}

	utils.WriteLines(consensusTargetsTmp, targets)

	// Map transcript IDs to gene IDs
	return mapTranscriptsToGenes(ctx, consensusTargetsTmp, assemblyDir, gtfDir, intermediateDir, outputFile, "consensus")
}

// mapTranscriptsToGenes maps StringTie transcript IDs to reference gene IDs
// This is a 2-step process:
// 1. Map transcript → StringTie gene (using assembly GTF)
// 2. Map StringTie gene → reference gene (using gffcompare tracking)
func mapTranscriptsToGenes(ctx context.Context, transcriptFile, assemblyDir, gtfDir, intermediateDir, outputFile, prefix string) error {
	stringtieGenesTmp := filepath.Join(intermediateDir, fmt.Sprintf("stringtie_genes_%s_tmp.txt", prefix))
	assemblyGTF := filepath.Join(assemblyDir, "transcripts.gtf")

	// Step 1: Map transcripts to StringTie genes
	transcripts, err := utils.ReadLines(transcriptFile)
	if err != nil {
		return err
	}

	var stringtieGenes []string
	if utils.FileExists(assemblyGTF) {
		for _, transcriptID := range transcripts {
			// grep -m1 "transcript_id \"$transcript_id\"" assembly.gtf | extract gene_id
			gene, err := extractGeneFromGTF(assemblyGTF, transcriptID)
			if err == nil && gene != "" {
				stringtieGenes = append(stringtieGenes, gene)
			}
		}
	}

	if len(stringtieGenes) == 0 {
		return nil
	}

	utils.WriteLines(stringtieGenesTmp, stringtieGenes)

	// Step 2: Map StringTie genes to reference genes using gffcompare tracking
	comparisonDir := filepath.Join(gtfDir, "comparison")
	trackingFile := filepath.Join(comparisonDir, "gffcompare.tracking")

	if !utils.FileExists(trackingFile) {
		utils.Warn("No gffcompare tracking file found")
		return nil
	}

	// Read tracking file
	trackingLines, err := utils.ReadLines(trackingFile)
	if err != nil {
		return err
	}

	// For each unique StringTie gene, find its reference gene from tracking file
	// This matches the bash script: sort -u | while read strg_gene; do grep -E "[:|]${strg_gene}[|]" ...
	uniqueStringtieGenes := make(map[string]bool)
	for _, gene := range stringtieGenes {
		uniqueStringtieGenes[gene] = true
	}

	var refGenes []string
	geneSet := make(map[string]bool)

	for strgGene := range uniqueStringtieGenes {
		// Search for the gene in tracking file
		// Bash: grep -E "[:|]${strg_gene}[|]" tracking_file
		// Pattern: [:|]STRG.123[|] to avoid partial matches
		refGene := findReferenceGeneInTracking(trackingLines, strgGene)

		if refGene != "" && !geneSet[refGene] {
			refGenes = append(refGenes, refGene)
			geneSet[refGene] = true
		}
	}

	return utils.WriteLines(outputFile, refGenes)
}

// findReferenceGeneInTracking searches tracking file for a StringTie gene and returns its reference gene
// Implements: grep -E "[:|]${strg_gene}[|]" tracking_file | awk to extract ref gene
func findReferenceGeneInTracking(trackingLines []string, strgGene string) string {
	// Search pattern: [:|]STRG.123[|]
	// This ensures we match the gene surrounded by delimiters to avoid partial matches
	for _, line := range trackingLines {
		// Check if line contains the pattern [:|]strgGene[|]
		if matchesTrackingPattern(line, strgGene) {
			// Extract reference gene from column 3 (0-indexed: 2)
			fields := strings.Split(line, "\t")
			if len(fields) >= 3 {
				refGene := fields[2]

				// Handle different formats:
				// 1. "gene-ID|rna-ID" -> extract gene-ID
				// 2. "gene-ID" -> use as is
				// 3. "-" -> skip (no reference)
				if refGene != "-" && refGene != "" {
					// Split by | and take first part
					parts := strings.Split(refGene, "|")
					geneID := parts[0]

					// Only return if it is not empty or "-"
					if geneID != "" && geneID != "-" {
						return geneID
					}
				}
			}
			break // Found the line, no need to continue
		}
	}

	return "" // No reference match - novel gene
}

// matchesTrackingPattern checks if line contains [:|]gene[|] pattern
// Implements grep -E "[:|]${strg_gene}[|]"
func matchesTrackingPattern(line, gene string) bool {
	// Pattern: [:|]STRG.123[|]
	// The gene must be preceded by : or | and followed by |
	pattern1 := ":" + gene + "|"
	pattern2 := "|" + gene + "|"

	return strings.Contains(line, pattern1) || strings.Contains(line, pattern2)
}

// extractGeneFromGTF extracts gene_id for a given transcript_id from GTF
func extractGeneFromGTF(gtfFile, transcriptID string) (string, error) {
	file, err := os.Open(gtfFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	searchStr := fmt.Sprintf(`transcript_id "%s"`, transcriptID)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, searchStr) {
			// Extract gene_id
			if idx := strings.Index(line, `gene_id "`); idx != -1 {
				start := idx + len(`gene_id "`)
				end := strings.Index(line[start:], `"`)
				if end != -1 {
					return line[start : start+end], nil
				}
			}
			break
		}
	}

	return "", scanner.Err()
}

// combineGeneLists combines all gene sources into a single unique list
func combineGeneLists(genesNear, genesLncTar, genesIntaRNA, genesConsensus, outputFile string) error {
	geneSet := make(map[string]bool)

	// Read all gene files
	files := []string{genesNear, genesLncTar, genesIntaRNA, genesConsensus}
	for _, file := range files {
		if utils.FileExists(file) {
			lines, err := utils.ReadLines(file)
			if err != nil {
				continue
			}
			for _, gene := range lines {
				if gene != "" && gene != "." {
					geneSet[gene] = true
				}
			}
		}
	}

	// Write unique genes
	var genes []string
	for gene := range geneSet {
		genes = append(genes, gene)
	}

	return utils.WriteLines(outputFile, genes)
}

// extractAllGenesFromGTF extracts all unique gene IDs from GTF for background gene list using native Go
func extractAllGenesFromGTF(ctx context.Context, gtfFile, outputFile string) error {
	file, err := os.Open(gtfFile)
	if err != nil {
		return err
	}
	defer file.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := bufio.NewWriter(out)
	defer writer.Flush()

	scanner := bufio.NewScanner(file)
	// Use a large buffer
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	seenGenes := make(map[string]bool)
	var genes []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 9 {
			continue
		}

		featureType := fields[2]
		if featureType != "gene" && featureType != "transcript" && featureType != "mRNA" {
			continue
		}

		geneID := parseGeneID(fields[8])
		if geneID != "" && !seenGenes[geneID] {
			seenGenes[geneID] = true
			genes = append(genes, geneID)
		}
	}

	// Sort genes for consistent output
	// Using a simple sort is fast enough for gene lists
	sort.Strings(genes)
	for _, gene := range genes {
		fmt.Fprintln(writer, gene)
	}

	return scanner.Err()
}

// parseGeneID tries to extract a meaningful ID from GFF/GTF attributes
// It handles both GTF style (key "value") and GFF3 style (key=value)
func parseGeneID(attributes string) string {
	// Strategy: Split by semicolon, clean whitespace, check common keys
	parts := strings.Split(attributes, ";")
	
	// Pre-allocate map for lookups if needed, but iterating is often faster for small sets
	// Priority list of keys to look for
	targetKeys := []string{"gene_id", "transcript_id", "locus_tag", "Name", "ID", "gene_name"}
	
	vals := make(map[string]string)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for GTF style: key "value"
		if strings.Contains(part, "\"") {
			firstSpace := strings.Index(part, " ")
			if firstSpace > 0 {
				key := part[:firstSpace]
				val := strings.Trim(part[firstSpace+1:], "\" \t")
				vals[key] = val
			}
		} else if strings.Contains(part, "=") {
			// Check for GFF3 style: key=value
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				vals[kv[0]] = kv[1]
			}
		}
	}

	// Return first matching key from priority list
	for _, key := range targetKeys {
		if v, ok := vals[key]; ok {
			// Special handling for "ID=gene-XYZ" which is common in GFF
			if key == "ID" && strings.HasPrefix(v, "gene-") {
				return strings.TrimPrefix(v, "gene-")
			}
			return v
		}
	}
	
	return ""
}

// countLines counts non-empty lines in a file
func countLines(filename string) int {
	if !utils.FileExists(filename) {
		return 0
	}
	lines, err := utils.ReadLines(filename)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
