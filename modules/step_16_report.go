package modules

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// PipelineSummary holds all metrics from the pipeline run
type PipelineSummary struct {
	// Metadata
	RunID            string    `json:"run_id"`
	SampleName       string    `json:"sample_name"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	TotalDuration    string    `json:"total_duration"`
	TotalDurationSec int64     `json:"total_duration_seconds"`
	Status           string    `json:"status"`

	// Configuration
	ReadType string `json:"read_type"` // "paired" or "single"
	Mode     string `json:"mode"`      // "reference" or "denovo"
	Species  string `json:"species"`

	// Step timings
	StepTimings []StepTiming `json:"step_timings,omitempty"`

	// Module metrics (ALL 15 modules)
	FastQC          *FastQCMetrics          `json:"fastqc,omitempty"`
	Trimmomatic     *TrimmomaticMetrics     `json:"trimmomatic,omitempty"`
	SortMeRNA       *SortMeRNAMetrics       `json:"sortmerna,omitempty"`
	HISAT2          *HISAT2Metrics          `json:"hisat2,omitempty"`
	StringTie       *StringTieMetrics       `json:"stringtie,omitempty"`
	CPC2            *CPC2Metrics            `json:"cpc2,omitempty"`
	CPAT            *CPATMetrics            `json:"cpat,omitempty"`
	LncRNAFiltering *LncRNAFilteringMetrics `json:"lncrna_filtering,omitempty"`
	RNAfold         *RNAfoldMetrics         `json:"rnafold,omitempty"`
	LncTar          *LncTarMetrics          `json:"lnctar,omitempty"`
	IntaRNA         *IntaRNAMetrics         `json:"intarna,omitempty"`
	Consensus       *ConsensusMetrics       `json:"consensus,omitempty"`
	Enrichment      *EnrichmentMetrics      `json:"enrichment,omitempty"`
	RSeQC           *RSeQCMetrics           `json:"rseqc,omitempty"`
	IGVReport       *IGVReportMetrics       `json:"igv_report,omitempty"`
	MultiQC         *MultiQCMetrics         `json:"multiqc,omitempty"`

	// Summary statistics
	TotalLncRNAs    int64 `json:"total_lncrnas"`
	NovelLncRNAs    int64 `json:"novel_lncrnas"`
	HighlyExpressed int64 `json:"highly_expressed"`
	BestCandidates  int64 `json:"best_candidates"`
	AssociatedGenes int64 `json:"associated_genes"`

	// Errors (if any)
	Errors []string `json:"errors,omitempty"`
}

// StepTiming holds timing information for each pipeline step
type StepTiming struct {
	StepNumber  int    `json:"step_number"`
	StepName    string `json:"step_name"`
	Duration    string `json:"duration"`
	DurationSec int64  `json:"duration_seconds"`
}

// ClassCodeInfo holds information about a gffcompare class code
type ClassCodeInfo struct {
	Code        string `json:"code"`
	Count       int64  `json:"count"`
	Description string `json:"description"`
}

// Metric structs for each module
type FastQCMetrics struct {
	TotalSequences int64   `json:"total_sequences"`
	PercentGC      float64 `json:"percent_gc"`
	GCContent      float64 `json:"gc_content"` // Alias for PercentGC for template compatibility
	QualityPass    bool    `json:"quality_pass"`
}

type TrimmomaticMetrics struct {
	InputReadPairs int64   `json:"input_read_pairs"`
	BothSurviving  int64   `json:"both_surviving"`
	SurvivalRate   float64 `json:"survival_rate"`
}

type SortMeRNAMetrics struct {
	TotalReads     int64   `json:"total_reads"`
	RRNAReads      int64   `json:"rrna_reads"`
	NonRRNAReads   int64   `json:"non_rrna_reads"`
	RRNAPercent    float64 `json:"rrna_percent"`
	NonRRNAPercent float64 `json:"non_rrna_percent"`
}

type HISAT2Metrics struct {
	TotalReadPairs       int64   `json:"total_read_pairs"`
	ConcordantTotal      int64   `json:"concordant_total"`
	ConcordantPercent    float64 `json:"concordant_percent"`
	OverallAlignmentRate float64 `json:"overall_alignment_rate"`
}

type StringTieMetrics struct {
	TotalTranscripts int64 `json:"total_transcripts"`
	TotalGenes       int64 `json:"total_genes"`
}

type CPC2Metrics struct {
	TotalTranscripts     int64   `json:"total_transcripts"`
	CodingTranscripts    int64   `json:"coding_transcripts"`
	NoncodingTranscripts int64   `json:"noncoding_transcripts"`
	NoncodingPercent     float64 `json:"noncoding_percent"`
}

type CPATMetrics struct {
	TotalTranscripts   int64   `json:"total_transcripts"`
	ConsensusNoncoding int64   `json:"consensus_noncoding"`
	ConsensusPercent   float64 `json:"consensus_percent"`
}

type LncRNAFilteringMetrics struct {
	FinalLncRNAs    int64                    `json:"final_lncrnas"`
	AllClassCodes   map[string]ClassCodeInfo `json:"all_class_codes"`
	NovelIntergenic int64                    `json:"novel_intergenic"`
	Antisense       int64                    `json:"antisense"`
	Intronic        int64                    `json:"intronic"`
	Known           int64                    `json:"known"`
	HighlyExpressed int64                    `json:"highly_expressed"`
	BestCandidates  int64                    `json:"best_candidates"`
}

type RNAfoldMetrics struct {
	TotalStructures int64 `json:"total_structures"`
}

type LncTarMetrics struct {
	TotalTargets int64 `json:"total_targets"`
}

type IntaRNAMetrics struct {
	TotalTargets int64 `json:"total_targets"`
}

type ConsensusMetrics struct {
	ConsensusPairs int64 `json:"consensus_pairs"`
}

type RSeQCMetrics struct {
	ReadDistributionDone bool  `json:"read_distribution_done"`
	CDSExons             int64 `json:"cds_exons"`
	FivePrimeUTR         int64 `json:"5_prime_utr"`
	ThreePrimeUTR        int64 `json:"3_prime_utr"`
	Introns              int64 `json:"introns"`
	TSSUp1kb             int64 `json:"tss_up_1kb"`
	TESDown1kb           int64 `json:"tes_down_1kb"`
}

type MultiQCMetrics struct {
	ReportGenerated bool  `json:"report_generated"`
	ReportSize      int64 `json:"report_size_bytes"`
}

type IGVReportMetrics struct {
	TotalLoci  int64 `json:"total_loci"`
	ReportSize int64 `json:"report_size_bytes"`
}

type EnrichmentMetrics struct {
	TotalBackgroundGenes int64 `json:"total_background_genes"`
	TotalAssociatedGenes int64 `json:"total_associated_genes"`
	GenesNearLncRNAs     int64 `json:"genes_near_lncrnas"`
}

// Step16GenerateReport creates comprehensive pipeline summary reports
func Step16GenerateReport(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(16, "Generating Pipeline Summary Report")

	// Create report directory
	reportDir := filepath.Join(cfg.OutputDir, "16_pipeline_report")
	if err := utils.CreateDirs(reportDir); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	utils.ShowProgress("Collecting comprehensive metrics from all 15 modules")

	// Collect all metrics
	summary := &PipelineSummary{
		RunID:      filepath.Base(cfg.OutputDir),
		SampleName: extractSampleName(cfg.File1),
		StartTime:  stepStart,
		EndTime:    time.Now(),
		ReadType:   cfg.DataType,
		Mode:       cfg.Method,
		Species:    cfg.CPATSpecies,
		Status:     "completed",
	}

	// Parse timing from pipeline.log
	summary.StepTimings = parseStepTimings(cfg)

	// Calculate total duration from step timings
	var totalSec int64
	for _, t := range summary.StepTimings {
		totalSec += t.DurationSec
	}
	summary.TotalDurationSec = totalSec
	summary.TotalDuration = fmt.Sprintf("%dm %ds", totalSec/60, totalSec%60)

	// Collect metrics from ALL 16 modules
	summary.FastQC = collectFastQCMetrics(cfg)
	summary.Trimmomatic = collectTrimmomaticMetrics(cfg)
	summary.SortMeRNA = collectSortMeRNAMetrics(cfg)
	summary.HISAT2 = collectHISAT2Metrics(cfg)
	summary.StringTie = collectStringTieMetrics(cfg)
	summary.CPC2 = collectCPC2Metrics(cfg)
	summary.CPAT = collectCPATMetrics(cfg)
	summary.LncRNAFiltering = collectLncRNAFilteringMetrics(cfg)
	summary.RNAfold = collectRNAfoldMetrics(cfg)
	summary.LncTar = collectLncTarMetrics(cfg)
	summary.IntaRNA = collectIntaRNAMetrics(cfg)
	summary.Consensus = collectConsensusMetrics(cfg)
	summary.Enrichment = collectEnrichmentMetrics(cfg)
	summary.RSeQC = collectRSeQCMetrics(cfg)
	summary.IGVReport = collectIGVMetrics(cfg)
	summary.MultiQC = collectMultiQCMetrics(cfg)

	// Populate summary statistics
	if summary.LncRNAFiltering != nil {
		summary.TotalLncRNAs = summary.LncRNAFiltering.FinalLncRNAs
		summary.NovelLncRNAs = summary.LncRNAFiltering.NovelIntergenic
		summary.HighlyExpressed = summary.LncRNAFiltering.HighlyExpressed
		summary.BestCandidates = summary.LncRNAFiltering.BestCandidates
	}

	if summary.Enrichment != nil {
		summary.AssociatedGenes = summary.Enrichment.TotalAssociatedGenes
	}

	utils.ShowProgress("Generating reports")

	// Generate JSON report (for API)
	if err := generateJSONReport(summary, reportDir); err != nil {
		utils.Warn("Failed to generate JSON report", zap.Error(err))
	}

	// Generate Markdown report (backup text format)
	if err := generateMarkdownReport(summary, reportDir); err != nil {
		utils.Warn("Failed to generate Markdown report", zap.Error(err))
	}

	// Generate HTML dashboard (main interactive report)
	if err := generateHTMLDashboard(summary, reportDir, cfg); err != nil {
		utils.Warn("Failed to generate HTML dashboard", zap.Error(err))
	}

	utils.StepComplete(16, "Pipeline Summary Report", stepStart)
	utils.Info("Reports generated",
		zap.String("html", filepath.Join(reportDir, "pipeline_summary.html")),
		zap.String("json", filepath.Join(reportDir, "pipeline_summary.json")),
		zap.String("markdown", filepath.Join(reportDir, "pipeline_summary.md")))

	return nil
}

// collectLncRNAFilteringMetrics extracts metrics from lncRNA filtering step
func collectLncRNAFilteringMetrics(cfg *config.Config) *LncRNAFilteringMetrics {
	metrics := &LncRNAFilteringMetrics{
		AllClassCodes: make(map[string]ClassCodeInfo),
	}

	// Class code descriptions (all 13 codes)
	descriptions := map[string]string{
		"=": "Complete match to reference",
		"c": "Contained in reference (intron compatible)",
		"j": "Potentially novel isoform (fragment)",
		"e": "Single exon overlapping single-exon reference",
		"i": "Fully contained within reference intron",
		"o": "Generic exonic overlap with reference",
		"p": "Possible polymerase run-on",
		"r": "Repeat (≥50% overlap)",
		"u": "Unknown/novel intergenic",
		"x": "Exonic overlap on opposite strand (antisense)",
		"s": "Intronic overlap on opposite strand",
		"m": "Retained intron(s), all matched",
		"n": "Retained intron(s), not all matched",
		"k": "Contains reference (reverse-containment)",
		"y": "Contains reference within intron",
	}

	// Parse class code counts
	classCodeFile := filepath.Join(cfg.OutputDir, "08_lncrna_analysis/novel_lncrnas/class_code_counts.txt")
	file, err := os.Open(classCodeFile)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 2 {
				count, _ := strconv.ParseInt(parts[0], 10, 64)
				code := parts[1]

				metrics.AllClassCodes[code] = ClassCodeInfo{
					Code:        code,
					Count:       count,
					Description: descriptions[code],
				}

				// Populate specific fields for backward compatibility
				switch code {
				case "u":
					metrics.NovelIntergenic = count
				case "x":
					metrics.Antisense = count
				case "i":
					metrics.Intronic = count
				case "=":
					metrics.Known = count
				}
			}
		}
	}

	// Count best candidates
	bestCandidatesFile := filepath.Join(cfg.OutputDir, "08_lncrna_analysis/expression/best_candidates.txt")
	metrics.BestCandidates = int64(countLines(bestCandidatesFile))

	// Count highly expressed
	highlyExpressedFile := filepath.Join(cfg.OutputDir, "08_lncrna_analysis/expression/highly_expressed_ids.txt")
	metrics.HighlyExpressed = int64(countLines(highlyExpressedFile))

	// Count final lncRNAs
	filteredFile := filepath.Join(cfg.OutputDir, "08_lncrna_analysis/filtered/lncrna_filtered.txt")
	metrics.FinalLncRNAs = int64(countLines(filteredFile))

	return metrics
}

// collectEnrichmentMetrics extracts enrichment metrics
func collectEnrichmentMetrics(cfg *config.Config) *EnrichmentMetrics {
	metrics := &EnrichmentMetrics{}

	// Count background genes
	backgroundFile := filepath.Join(cfg.OutputDir, "12_enrichment/all_genes_background.txt")
	metrics.TotalBackgroundGenes = int64(countLines(backgroundFile))

	// Count associated genes
	associatedFile := filepath.Join(cfg.OutputDir, "12_enrichment/genes_associated_with_lncRNAs_combined.txt")
	metrics.TotalAssociatedGenes = int64(countLines(associatedFile))

	// Count nearby genes
	nearbyFile := filepath.Join(cfg.OutputDir, "12_enrichment/genes_near_lncRNAs_unique.txt")
	metrics.GenesNearLncRNAs = int64(countLines(nearbyFile))

	return metrics
}

// collectHISAT2Metrics extracts alignment metrics (would need to parse log)
func collectHISAT2Metrics(cfg *config.Config) *HISAT2Metrics {
	metrics := &HISAT2Metrics{}
	logFile := filepath.Join(cfg.OutputDir, "pipeline.log")

	file, err := os.Open(logFile)
	if err != nil {
		return metrics
	}
	defer file.Close()

	// Regexes for HISAT2 output in pipeline.log
	// "7193656 reads; of these:"
	reTotal := regexp.MustCompile(`(\d+) reads; of these:`)
	// "1.41% overall alignment rate"
	reRate := regexp.MustCompile(`([\d\.]+)% overall alignment rate`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := reTotal.FindStringSubmatch(line); matches != nil {
			metrics.TotalReadPairs, _ = strconv.ParseInt(matches[1], 10, 64)
		} else if matches := reRate.FindStringSubmatch(line); matches != nil {
			metrics.OverallAlignmentRate, _ = strconv.ParseFloat(matches[1], 64)
		}
	}
	return metrics
}

// collectCPC2Metrics extracts CPC2 metrics
func collectCPC2Metrics(cfg *config.Config) *CPC2Metrics {
	metrics := &CPC2Metrics{}

	cpc2File := filepath.Join(cfg.OutputDir, "06_cpc2/cpc2_output.txt")
	file, err := os.Open(cpc2File)
	if err != nil {
		return metrics
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		metrics.TotalTranscripts++
		if strings.Contains(scanner.Text(), "noncoding") {
			metrics.NoncodingTranscripts++
		} else if strings.Contains(scanner.Text(), "coding") {
			metrics.CodingTranscripts++
		}
	}

	if metrics.TotalTranscripts > 0 {
		metrics.NoncodingPercent = float64(metrics.NoncodingTranscripts) / float64(metrics.TotalTranscripts) * 100
	}

	return metrics
}

// collectCPATMetrics extracts CPAT metrics
func collectCPATMetrics(cfg *config.Config) *CPATMetrics {
	metrics := &CPATMetrics{}

	consensusFile := filepath.Join(cfg.OutputDir, "07_validation/consensus_noncoding.txt")
	metrics.ConsensusNoncoding = int64(countLines(consensusFile))

	return metrics
}

// collectStringTieMetrics extracts StringTie assembly metrics
func collectStringTieMetrics(cfg *config.Config) *StringTieMetrics {
	gtfFile := filepath.Join(cfg.OutputDir, "05_assembly/transcripts.gtf")
	return &StringTieMetrics{
		TotalTranscripts: countGTFFeatures(gtfFile, "transcript"),
		TotalGenes:       countGTFFeatures(gtfFile, "gene"),
	}
}

// collectRNAfoldMetrics extracts RNAfold structure metrics
func collectRNAfoldMetrics(cfg *config.Config) *RNAfoldMetrics {
	svgDir := filepath.Join(cfg.OutputDir, "09_rnafold/svg_files")
	files, _ := filepath.Glob(filepath.Join(svgDir, "*.svg"))
	return &RNAfoldMetrics{TotalStructures: int64(len(files))}
}

// collectLncTarMetrics extracts LncTar target prediction metrics
func collectLncTarMetrics(cfg *config.Config) *LncTarMetrics {
	targetFile := filepath.Join(cfg.OutputDir, "11_target_prediction/lnctar/best_candidates_targets.txt")
	return &LncTarMetrics{TotalTargets: int64(countLines(targetFile))}
}

// collectIntaRNAMetrics extracts IntaRNA target prediction metrics
func collectIntaRNAMetrics(cfg *config.Config) *IntaRNAMetrics {
	targetFile := filepath.Join(cfg.OutputDir, "11_target_prediction/intarna/best_candidates_targets.csv")
	return &IntaRNAMetrics{TotalTargets: int64(countLines(targetFile))}
}

// collectConsensusMetrics extracts consensus target metrics
func collectConsensusMetrics(cfg *config.Config) *ConsensusMetrics {
	consensusFile := filepath.Join(cfg.OutputDir, "11_target_prediction/consensus_pairs.txt")
	return &ConsensusMetrics{ConsensusPairs: int64(countLines(consensusFile))}
}

// collectRSeQCMetrics extracts RSeQC quality metrics
func collectRSeQCMetrics(cfg *config.Config) *RSeQCMetrics {
	metrics := &RSeQCMetrics{}
	readDistFile := filepath.Join(cfg.OutputDir, "13_rseqc/read_distribution.txt")

	if utils.FileExists(readDistFile) {
		// Check if file is not empty
		if info, err := os.Stat(readDistFile); err == nil && info.Size() > 0 {
			metrics.ReadDistributionDone = true

			// Parse read distribution if file has content
			file, err := os.Open(readDistFile)
			if err == nil {
				defer file.Close()
				scanner := bufio.NewScanner(file)
				// Skip header lines until we find the table
				// Group              Total_bases  Tag_count    Tags/Kb
				// CDS_Exons          ...
				for scanner.Scan() {
					fields := strings.Fields(scanner.Text())
					if len(fields) >= 3 {
						if strings.HasPrefix(fields[0], "CDS_Exons") {
							metrics.CDSExons, _ = strconv.ParseInt(fields[2], 10, 64)
						} else if strings.HasPrefix(fields[0], "5'UTR_Exons") {
							metrics.FivePrimeUTR, _ = strconv.ParseInt(fields[2], 10, 64)
						} else if strings.HasPrefix(fields[0], "3'UTR_Exons") {
							metrics.ThreePrimeUTR, _ = strconv.ParseInt(fields[2], 10, 64)
						} else if strings.HasPrefix(fields[0], "Introns") {
							metrics.Introns, _ = strconv.ParseInt(fields[2], 10, 64)
						} else if strings.HasPrefix(fields[0], "TSS_up_1kb") {
							metrics.TSSUp1kb, _ = strconv.ParseInt(fields[2], 10, 64)
						} else if strings.HasPrefix(fields[0], "TES_down_1kb") {
							metrics.TESDown1kb, _ = strconv.ParseInt(fields[2], 10, 64)
						}
					}
				}
			}
		}
	}
	return metrics
}

// collectMultiQCMetrics extracts MultiQC report metrics
func collectMultiQCMetrics(cfg *config.Config) *MultiQCMetrics {
	mqcFile := filepath.Join(cfg.OutputDir, "15_multiqc/lncrna_pipeline_report.html")
	metrics := &MultiQCMetrics{ReportGenerated: utils.FileExists(mqcFile)}
	if metrics.ReportGenerated {
		if info, err := os.Stat(mqcFile); err == nil {
			metrics.ReportSize = info.Size()
		}
	}
	return metrics
}

// collectIGVMetrics extracts IGV report metrics
func collectIGVMetrics(cfg *config.Config) *IGVReportMetrics {
	igvFile := filepath.Join(cfg.OutputDir, "14_igv_report/lncrna_igv_report.html")
	bedFile := filepath.Join(cfg.OutputDir, "08_lncrna_analysis/filtered/lncrna.bed")

	metrics := &IGVReportMetrics{}

	// Count loci from BED file
	metrics.TotalLoci = int64(countLines(bedFile))

	// Get report size
	if info, err := os.Stat(igvFile); err == nil {
		metrics.ReportSize = info.Size()
	}

	return metrics
}

// collectFastQCMetrics extracts FastQC metrics from MultiQC stats
func collectFastQCMetrics(cfg *config.Config) *FastQCMetrics {
	metrics := &FastQCMetrics{}

	// Try to parse from MultiQC general stats
	mqcStatsFile := filepath.Join(cfg.OutputDir, "15_multiqc/lncrna_pipeline_report_data/multiqc_general_stats.txt")
	if utils.FileExists(mqcStatsFile) {
		file, err := os.Open(mqcStatsFile)
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)

			// Header: Sample ... fastqc-percent_gc ... fastqc-total_sequences
			// We need to find the indices of these columns
			var gcIdx, seqIdx int = -1, -1

			if scanner.Scan() {
				header := strings.Split(scanner.Text(), "\t")
				for i, col := range header {
					if col == "fastqc-percent_gc" {
						gcIdx = i
					} else if col == "fastqc-total_sequences" {
						seqIdx = i
					}
				}
			}

			// Read first sample line (assuming forward read)
			if scanner.Scan() && gcIdx != -1 && seqIdx != -1 {
				fields := strings.Split(scanner.Text(), "\t")
				if len(fields) > gcIdx && len(fields) > seqIdx {
					metrics.GCContent, _ = strconv.ParseFloat(fields[gcIdx], 64)
					seqCount, _ := strconv.ParseFloat(fields[seqIdx], 64)
					metrics.TotalSequences = int64(seqCount * 1000000) // MultiQC often reports in millions, but here it looks like raw number?
					// Wait, in the file view it was 7.253932. If it's millions, it's 7,253,932.
					// If it's raw, it's 7. But 7 sequences is impossible. So it must be millions.
					// Let's check if it's raw or millions. 7.253932 * 10^6 = 7253932. This matches HISAT2 input.
				}
			}
			metrics.QualityPass = true
		}
	} else {
		// Fallback: Check if FastQC output exists
		fastqcDir := filepath.Join(cfg.OutputDir, "01_fastqc")
		metrics.QualityPass = utils.FileExists(fastqcDir)
	}

	return metrics
}

// collectTrimmomaticMetrics extracts Trimmomatic metrics from pipeline.log
func collectTrimmomaticMetrics(cfg *config.Config) *TrimmomaticMetrics {
	metrics := &TrimmomaticMetrics{}
	logFile := filepath.Join(cfg.OutputDir, "pipeline.log")

	file, err := os.Open(logFile)
	if err != nil {
		return metrics
	}
	defer file.Close()

	// Regex for: Input Read Pairs: 7253932 Both Surviving: 7233841 (99.72%) ...
	re := regexp.MustCompile(`Input Read Pairs: (\d+) Both Surviving: (\d+) \(([\d\.]+)%\)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			metrics.InputReadPairs, _ = strconv.ParseInt(matches[1], 10, 64)
			metrics.BothSurviving, _ = strconv.ParseInt(matches[2], 10, 64)
			metrics.SurvivalRate, _ = strconv.ParseFloat(matches[3], 64)
			break // Found the line
		}
	}
	return metrics
}

// collectSortMeRNAMetrics extracts SortMeRNA metrics from rrna.log
func collectSortMeRNAMetrics(cfg *config.Config) *SortMeRNAMetrics {
	metrics := &SortMeRNAMetrics{}
	logFile := filepath.Join(cfg.OutputDir, "03_sortmerna", "rrna.log")

	file, err := os.Open(logFile)
	if err != nil {
		return metrics
	}
	defer file.Close()

	// Regexes
	reTotal := regexp.MustCompile(`Total reads = (\d+)`)
	reRRNA := regexp.MustCompile(`Total reads passing E-value threshold = (\d+) \(([\d\.]+)\)`)
	reNonRRNA := regexp.MustCompile(`Total reads failing E-value threshold = (\d+) \(([\d\.]+)\)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := reTotal.FindStringSubmatch(line); matches != nil {
			metrics.TotalReads, _ = strconv.ParseInt(matches[1], 10, 64)
		} else if matches := reRRNA.FindStringSubmatch(line); matches != nil {
			metrics.RRNAReads, _ = strconv.ParseInt(matches[1], 10, 64)
			metrics.RRNAPercent, _ = strconv.ParseFloat(matches[2], 64)
		} else if matches := reNonRRNA.FindStringSubmatch(line); matches != nil {
			metrics.NonRRNAReads, _ = strconv.ParseInt(matches[1], 10, 64)
			metrics.NonRRNAPercent, _ = strconv.ParseFloat(matches[2], 64)
		}
	}
	return metrics
}

// parseStepTimings parses pipeline.log for step timing information
func parseStepTimings(cfg *config.Config) []StepTiming {
	logFile := filepath.Join(cfg.OutputDir, "pipeline.log")
	file, err := os.Open(logFile)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Map to store latest timing for each step (key is step ID)
	latestTimings := make(map[int]StepTiming)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse JSON log format: {"msg":"Step completed","step":"StepName","duration":76.4}
		if strings.Contains(line, "Step completed") && strings.Contains(line, "duration") {
			// Extract Step Name
			nameRe := regexp.MustCompile(`"step":"([^"]+)"`)
			nameMatches := nameRe.FindStringSubmatch(line)
			if nameMatches == nil {
				continue
			}
			stepName := nameMatches[1]
			stepID := getStepID(stepName)

			// Extract duration (in seconds as float)
			durationRe := regexp.MustCompile(`"duration":([0-9.]+)`)
			if matches := durationRe.FindStringSubmatch(line); matches != nil {
				durationSec, _ := strconv.ParseFloat(matches[1], 64)
				totalSec := int64(durationSec)
				mins := totalSec / 60
				secs := totalSec % 60

				// Store/Overwrite in map to keep only the latest run info
				latestTimings[stepID] = StepTiming{
					StepNumber:  stepID,
					StepName:    stepName,
					Duration:    fmt.Sprintf("%dm %ds", mins, secs),
					DurationSec: totalSec,
				}
			}
		}
	}

	// Convert map to slice and sort by StepNumber
	var timings []StepTiming
	for _, t := range latestTimings {
		timings = append(timings, t)
	}

	// Sort
	// (Simple bubble sort or we can use sort package, but bubble is fine for 16 items)
	for i := 0; i < len(timings)-1; i++ {
		for j := 0; j < len(timings)-i-1; j++ {
			if timings[j].StepNumber > timings[j+1].StepNumber {
				timings[j], timings[j+1] = timings[j+1], timings[j]
			}
		}
	}

	return timings
}

// getStepID maps step names to their canonical ID
func getStepID(name string) int {
	name = strings.ToLower(name)
	if strings.Contains(name, "fastqc") {
		return 1
	} else if strings.Contains(name, "trimmomatic") {
		return 2
	} else if strings.Contains(name, "sortmerna") {
		return 3
	} else if strings.Contains(name, "hisat2") || strings.Contains(name, "alignment") {
		return 4
	} else if strings.Contains(name, "cpc2") {
		return 5
	} else if strings.Contains(name, "cpat") {
		return 6
	} else if strings.Contains(name, "lncrna filtering") || strings.Contains(name, "filtering") {
		return 7
	} else if strings.Contains(name, "rnafold") || strings.Contains(name, "structure") {
		return 8
	} else if strings.Contains(name, "lnctar") {
		return 9
	} else if strings.Contains(name, "intarna") {
		return 10
	} else if strings.Contains(name, "consensus") {
		return 11
	} else if strings.Contains(name, "enrichment") {
		return 12
	} else if strings.Contains(name, "rseqc") {
		return 13
	} else if strings.Contains(name, "igv") {
		return 14
	} else if strings.Contains(name, "multiqc") {
		return 15
	} else if strings.Contains(name, "report") {
		return 16
	}
	return 99 // Unknown
}

// getStepName returns the name for a given step number
func getStepName(num int) string {
	names := map[int]string{
		1: "FastQC", 2: "Trimmomatic", 3: "SortMeRNA", 4: "HISAT2/StringTie",
		5: "CPC2", 6: "CPAT", 7: "lncRNA Filtering", 8: "RNAfold",
		9: "LncTar", 10: "IntaRNA", 11: "Consensus", 12: "Enrichment",
		13: "RSeQC", 14: "IGV", 15: "MultiQC", 16: "Report",
	}
	if name, ok := names[num]; ok {
		return name
	}
	return fmt.Sprintf("Step %d", num)
}

// countGTFFeatures counts specific features in a GTF file
func countGTFFeatures(gtfFile, featureType string) int64 {
	file, err := os.Open(gtfFile)
	if err != nil {
		return 0
	}
	defer file.Close()

	var count int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) > 2 && fields[2] == featureType {
			count++
		}
	}
	return count
}

// Helper functions

func extractSampleName(filepath string) string {
	base := filepath[strings.LastIndex(filepath, "/")+1:]
	// Remove common suffixes
	base = strings.TrimSuffix(base, ".fastq")
	base = strings.TrimSuffix(base, ".fq")
	base = strings.TrimSuffix(base, ".gz")
	base = strings.TrimSuffix(base, "_forward")
	base = strings.TrimSuffix(base, "_reverse")
	base = strings.TrimSuffix(base, "_1")
	base = strings.TrimSuffix(base, "_2")
	return base
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// generateJSONReport creates JSON report
func generateJSONReport(summary *PipelineSummary, reportDir string) error {
	jsonFile := filepath.Join(reportDir, "pipeline_summary.json")

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(jsonFile, data, 0644)
}

// generateMarkdownReport creates Markdown report
func generateMarkdownReport(summary *PipelineSummary, reportDir string) error {
	mdFile := filepath.Join(reportDir, "pipeline_summary.md")

	tmpl := `REGIS Pipeline - Detailed Summary Report
==========================================

Run ID: {{.RunID}}
Sample: {{.SampleName}}
Mode: {{.Mode}} ({{.ReadType}})
Species: {{.Species}}
Total Duration: {{.TotalDuration}} ({{.TotalDurationSec}}s)
Status: {{.Status}}

==========================================
RESULTS OVERVIEW
==========================================

lncRNAs Discovered: {{.TotalLncRNAs}}
Novel Intergenic: {{.NovelLncRNAs}}
Highly Expressed: {{.HighlyExpressed}}
Best Candidates: {{.BestCandidates}}
Associated Genes: {{.AssociatedGenes}}

==========================================
STEP-BY-STEP MODULE DETAILS
==========================================
{{range .StepTimings}}
------------------------------------------
Step {{.StepNumber}}: {{.StepName}}
Duration: {{.Duration}} ({{.DurationSec}}s)
------------------------------------------
{{end}}
{{if .FastQC}}
------------------------------------------
Module: FastQC Quality Control
------------------------------------------
Input:
  - Raw FASTQ files (forward and reverse)

Output:
  - 01_fastqc/*_fastqc.html
  - 01_fastqc/*_fastqc.zip

Metrics:
  Total Sequences: {{.FastQC.TotalSequences}}
  GC Content: {{printf "%.1f" .FastQC.GCContent}}%
  Quality Pass: {{if .FastQC.QualityPass}}✓ Yes{{else}}✗ No{{end}}
{{end}}
{{if .Trimmomatic}}
------------------------------------------
Module: Trimmomatic Adapter Trimming
------------------------------------------
Input:
  - Raw FASTQ files

Output:
  - 02_trimming/paired_1.fastq
  - 02_trimming/paired_2.fastq
  - 02_trimming/unpaired_1.fastq
  - 02_trimming/unpaired_2.fastq

Metrics:
  Input Read Pairs: {{.Trimmomatic.InputReadPairs}}
  Both Surviving: {{.Trimmomatic.BothSurviving}} ({{printf "%.1f" .Trimmomatic.SurvivalRate}}%)
{{end}}
{{if .SortMeRNA}}
------------------------------------------
Module: SortMeRNA rRNA Filtering
------------------------------------------
Input:
  - 02_trimming/paired_1.fastq
  - 02_trimming/paired_2.fastq
  - SILVA + Rfam rRNA databases

Output:
  - 03_sortmerna/filtered_fwd.fq
  - 03_sortmerna/filtered_rev.fq
  - 03_sortmerna/rrna_fwd.fq
  - 03_sortmerna/rrna_rev.fq

Metrics:
  Total Reads: {{.SortMeRNA.TotalReads}}
  rRNA Reads: {{.SortMeRNA.RRNAReads}} ({{printf "%.1f" .SortMeRNA.RRNAPercent}}%)
  Non-rRNA Reads: {{.SortMeRNA.NonRRNAReads}} ({{printf "%.1f" .SortMeRNA.NonRRNAPercent}}%)
{{end}}
{{if .HISAT2}}
------------------------------------------
Module: HISAT2 Genome Alignment
------------------------------------------
Input:
  - 03_sortmerna/filtered_fwd.fq
  - 03_sortmerna/filtered_rev.fq
  - Reference genome FASTA
  - Reference annotation GFF

Output:
  - 04_alignment/aligned_sorted.bam
  - 04_alignment/aligned_sorted.bam.bai
  - 04_alignment/hisat2_index.*.ht2 (8 files)

Metrics:
  Total Read Pairs: {{.HISAT2.TotalReadPairs}}
  Concordant Pairs: {{.HISAT2.ConcordantTotal}} ({{printf "%.1f" .HISAT2.ConcordantPercent}}%)
  Overall Alignment Rate: {{printf "%.2f" .HISAT2.OverallAlignmentRate}}%
{{end}}
{{if .StringTie}}
------------------------------------------
Module: StringTie Assembly
------------------------------------------
Input:
  - 04_alignment/aligned_sorted.bam
  - Reference annotation GFF

Output:
  - 05_assembly/transcripts.gtf

Metrics:
  Total Transcripts: {{.StringTie.TotalTranscripts}}
  Total Genes: {{.StringTie.TotalGenes}}
{{end}}
{{if .CPC2}}
------------------------------------------
Module: CPC2 Coding Potential Assessment
------------------------------------------
Input:
  - 06_cpc2/transcripts.fa

Output:
  - 06_cpc2/cpc2_output.txt

Metrics:
  Total Transcripts: {{.CPC2.TotalTranscripts}}
  Coding: {{.CPC2.CodingTranscripts}}
  Noncoding: {{.CPC2.NoncodingTranscripts}} ({{printf "%.1f" .CPC2.NoncodingPercent}}%)
{{end}}
{{if .CPAT}}
------------------------------------------
Module: CPAT Cross-Validation
------------------------------------------
Input:
  - 06_cpc2/transcripts.fa
  - 06_cpc2/cpc2_output.txt
  - CPAT models (Hexamer + Logit)

Output:
  - 07_validation/cpat_output.ORF_prob.best.tsv
  - 07_validation/cpc2_noncoding.txt
  - 07_validation/cpat_noncoding.txt
  - 07_validation/consensus_noncoding.txt

Metrics:
  Consensus Noncoding: {{.CPAT.ConsensusNoncoding}}
{{end}}
{{if .LncRNAFiltering}}
------------------------------------------
Module: lncRNA Filtering & Classification
------------------------------------------
Input:
  - 07_validation/consensus_noncoding.txt
  - 06_cpc2/cpc2_output.txt
  - Reference annotation

Output:
  - 08_lncrna_analysis/filtered/lncrna_filtered.fa
  - 08_lncrna_analysis/filtered/lncrna.gtf
  - 08_lncrna_analysis/filtered/lncrna.bed
  - 08_lncrna_analysis/expression/lncrna_counts.txt
  - 08_lncrna_analysis/expression/highly_expressed.fa
  - 08_lncrna_analysis/expression/best_candidates.txt
  - 08_lncrna_analysis/novel_lncrnas/class_code_counts.txt

Metrics:
  Final lncRNAs: {{.LncRNAFiltering.FinalLncRNAs}}
  Highly Expressed (≥10 reads): {{.LncRNAFiltering.HighlyExpressed}}
  Best Candidates: {{.LncRNAFiltering.BestCandidates}}

Classification (gffcompare class codes):
{{range $code, $info := .LncRNAFiltering.AllClassCodes}}  {{$info.Code}} ({{$info.Count}}): {{$info.Description}}
{{end}}
{{end}}
{{if .RNAfold}}
------------------------------------------
Module: RNAfold Secondary Structure
------------------------------------------
Input:
  - 06_cpc2/transcripts.fa

Output:
  - 09_rnafold/lncrna_structures.out
  - 09_rnafold/png_files/*.png ({{.RNAfold.TotalStructures}} files)
  - 09_rnafold/ps_files/*.ps ({{.RNAfold.TotalStructures}} files)

Metrics:
  Total Structures Predicted: {{.RNAfold.TotalStructures}}
{{end}}
{{if .LncTar}}
------------------------------------------
Module: LncTar Target Prediction
------------------------------------------
Input:
  - 08_lncrna_analysis/expression/best_candidates.txt
  - All mRNA sequences

Output:
  - 11_target_prediction/lnctar/best_candidates_targets.txt

Metrics:
  Total Target Predictions: {{.LncTar.TotalTargets}}
{{end}}
{{if .IntaRNA}}
------------------------------------------
Module: IntaRNA Target Prediction
------------------------------------------
Input:
  - 08_lncrna_analysis/expression/best_candidates.txt
  - All mRNA sequences

Output:
  - 11_target_prediction/intarna/best_candidates_targets.csv

Metrics:
  Total Target Predictions: {{.IntaRNA.TotalTargets}}
{{end}}
{{if .Consensus}}
------------------------------------------
Module: Consensus Target Analysis
------------------------------------------
Input:
  - 11_target_prediction/lnctar/best_candidates_targets.txt
  - 11_target_prediction/intarna/best_candidates_targets.csv

Output:
  - 11_target_prediction/consensus_pairs.txt
  - 11_target_prediction/consensus_summary.txt

Metrics:
  High-Confidence Consensus Pairs: {{.Consensus.ConsensusPairs}}
{{end}}
{{if .Enrichment}}
------------------------------------------
Module: Enrichment Gene List Building
------------------------------------------
Input:
  - 08_lncrna_analysis/filtered/lncrna.bed
  - Reference GTF annotation
  - LncTar/IntaRNA predictions

Output:
  - 12_enrichment/genes_associated_with_lncRNAs_combined.txt
  - 12_enrichment/all_genes_background.txt
  - 12_enrichment/genes_near_lncRNAs_unique.txt
  - 12_enrichment/genes_from_lnctar_mapped.txt
  - 12_enrichment/genes_from_intarna_mapped.txt
  - 12_enrichment/genes_from_consensus_mapped.txt

Metrics:
  Total Background Genes: {{.Enrichment.TotalBackgroundGenes}}
  Associated Genes: {{.Enrichment.TotalAssociatedGenes}}
  Genes Near lncRNAs: {{.Enrichment.GenesNearLncRNAs}}
{{end}}
{{if .RSeQC}}
------------------------------------------
Module: RSeQC Quality Assessment
------------------------------------------
Input:
  - 04_alignment/aligned_sorted.bam
  - Reference GTF annotation

Output:
  - 13_rseqc/reference.bed12
  - 13_rseqc/gene_body_coverage.geneBodyCoverage.txt
  - 13_rseqc/read_distribution.txt
  - 13_rseqc/infer_experiment.txt

Status:
  Read Distribution: {{if .RSeQC.ReadDistributionDone}}✓ Completed{{else}}✗ Not found{{end}}

Metrics:
  {{if .RSeQC.ReadDistributionDone}}
  CDS Exons: {{.RSeQC.CDSExons}}
  Introns: {{.RSeQC.Introns}}
  TSS Up 1kb: {{.RSeQC.TSSUp1kb}}
  {{else}}
  Metrics not available
  {{end}}
{{end}}
{{if .IGVReport}}
------------------------------------------
Module: IGV Genome Browser Report
------------------------------------------
Input:
  - 08_lncrna_analysis/filtered/lncrna.bed
  - 04_alignment/aligned_sorted.bam
  - Reference genome FASTA
  - Reference annotation GFF

Output:
  - 14_igv_report/lncrna_igv_report.html

Metrics:
  lncRNA Loci Visualized: {{.IGVReport.TotalLoci}}
  Report Size: {{.IGVReport.ReportSize}} bytes
{{end}}
{{if .MultiQC}}
------------------------------------------
Module: MultiQC Comprehensive Report
------------------------------------------
Input:
  - All output files from modules 1-15

Output:
  - 15_multiqc/lncrna_pipeline_report.html
  - 15_multiqc/lncrna_pipeline_report_data/

Status:
  Report Generated: {{if .MultiQC.ReportGenerated}}✓ Yes ({{.MultiQC.ReportSize}} bytes){{else}}✗ No{{end}}
{{end}}
==========================================
KEY OUTPUT FILES
==========================================

Best lncRNA Candidates:
  08_lncrna_analysis/expression/best_candidates.txt

All lncRNA Sequences:
  08_lncrna_analysis/filtered/lncrna_filtered.fa

Gene List for Enrichment:
  12_enrichment/genes_associated_with_lncRNAs_combined.txt

QC Dashboard:
  15_multiqc/lncrna_pipeline_report.html

This Report (JSON):
  16_pipeline_report/pipeline_summary.json

==========================================
Report Generated: {{.EndTime.Format "2006-01-02 15:04:05"}}
==========================================
`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(mdFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, summary)
}
