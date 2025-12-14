package modules

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BioinformaticsOnLine/regis/config"
)

const (
	MaxTableRows      = 1000 // Limit shown rows (User requested increase)
	MaxCarouselImages = 500  // Limit carousel items (User requested increase)
)

// Link represents a hyperlink
type Link struct {
	Label string
	URL   string
}

// ReportImage represents an image in the report with a label
type ReportImage struct {
	Path     string
	Label    string
	Sequence string
	CPC2Data *CPC2Row
}

// CPC2Row represents a row in the CPC2 output table
type CPC2Row struct {
	ID               string
	TranscriptLength string
	PeptideLength    string
	FickettScore     string
	PI               string
	ORFIntegrity     string
	CodingProb       string
	Label            string
}

// ModuleRenderData holds all info needed to render a module card in the HTML report
type ModuleRenderData struct {
	ID          int
	Name        string
	Icon        string
	Duration    string
	Status      string
	Description []string
	Inputs      []string
	Outputs     []string
	Metrics     interface{}   // The specific metric struct
	Images      []ReportImage // Paths to images to display (e.g. RNAfold structures)
	TotalImages int           // Total available images (if > len(Images), show warning)
	ExtraLinks  []Link        // Extra links (e.g. RSeQC PDF)
	TableData   []CPC2Row     // For displaying tables (e.g. CPC2 output)
	TotalRows   int           // Total available rows (if > len(TableData), show warning)
}

// PipelineInfo holds information about the pipeline execution
type PipelineInfo struct {
	Mode       string
	DataType   string
	Species    string
	Parameters map[string]string
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// copyAssetsToReportDir copies necessary assets to the report directory to make it self-contained
func copyAssetsToReportDir(cfg *config.Config, reportDir string) error {
	assetsDir := filepath.Join(reportDir, "assets")

	// Create subdirectories
	dirs := []string{"rnafold", "rseqc", "multiqc", "igv", "fastqc", "logos"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(assetsDir, d), 0755); err != nil {
			return err
		}
	}

	// Helper to copy without failing the whole process
	safeCopy := func(src, dst string) {
		if err := copyFile(src, dst); err != nil {
			// Ignore errors for optional files
		}
	}

	// 1. Copy logos
	// 1. Copy logos
	logoFiles := []string{
		"regis-square-logo.png",
		"jnlab-logo_long_form.png",
	}
	for _, filename := range logoFiles {
		// Use cfg.AssetsDir to find the source file
		srcPath := filepath.Join(cfg.AssetsDir, "logo", filename)
		if _, err := os.Stat(srcPath); err == nil {
			safeCopy(srcPath, filepath.Join(assetsDir, "logos", filename))
		}
	}

	// 2. Copy RNAfold SVGs (All)
	svgDir := filepath.Join(cfg.OutputDir, "09_rnafold", "svg_files")
	files, _ := os.ReadDir(svgDir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".svg") {
			src := filepath.Join(svgDir, f.Name())
			dst := filepath.Join(assetsDir, "rnafold", f.Name())
			safeCopy(src, dst)
		}
	}

	// 3. Copy RSeQC PDF
	rseqcDir := filepath.Join(cfg.OutputDir, "13_rseqc")
	rseqcFiles, _ := os.ReadDir(rseqcDir)
	for _, f := range rseqcFiles {
		if strings.HasSuffix(f.Name(), ".pdf") {
			src := filepath.Join(rseqcDir, f.Name())
			dst := filepath.Join(assetsDir, "rseqc", f.Name())
			safeCopy(src, dst)
		}
	}

	// 4. Copy MultiQC Report
	multiqcSrc := filepath.Join(cfg.OutputDir, "15_multiqc", "lncrna_pipeline_report.html")
	multiqcDst := filepath.Join(assetsDir, "multiqc", "lncrna_pipeline_report.html")
	safeCopy(multiqcSrc, multiqcDst)

	// 5. Copy IGV Report
	igvSrc := filepath.Join(cfg.OutputDir, "14_igv_report", "lncrna_igv_report.html")
	igvDst := filepath.Join(assetsDir, "igv", "lncrna_igv_report.html")
	safeCopy(igvSrc, igvDst)

	// 6. Copy FastQC Reports
	fastqcDir := filepath.Join(cfg.OutputDir, "01_fastqc")
	fastqcFiles, _ := os.ReadDir(fastqcDir)
	for _, f := range fastqcFiles {
		if strings.HasSuffix(f.Name(), ".html") {
			src := filepath.Join(fastqcDir, f.Name())
			dst := filepath.Join(assetsDir, "fastqc", f.Name())
			safeCopy(src, dst)
		}
	}

	return nil
}

// parseCPC2Output reads the CPC2 output file and returns rows and a map
func parseCPC2Output(path string) ([]CPC2Row, map[string]CPC2Row, error) {
	lines, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	var rows []CPC2Row
	rowMap := make(map[string]CPC2Row)

	for i, line := range strings.Split(string(lines), "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header or empty
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		row := CPC2Row{
			ID:               fields[0],
			TranscriptLength: fields[1],
			PeptideLength:    fields[2],
			FickettScore:     fields[3],
			PI:               fields[4],
			ORFIntegrity:     fields[5],
			CodingProb:       fields[6],
			Label:            fields[7],
		}
		rows = append(rows, row)
		rowMap[row.ID] = row
	}
	return rows, rowMap, nil
}

// parseFasta reads a FASTA file into a map[header]sequence
func parseFasta(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	seqs := make(map[string]string)
	var currentID string
	var currentSeq strings.Builder

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			// Save previous
			if currentID != "" {
				seqs[currentID] = currentSeq.String()
			}
			// Start new
			currentID = strings.TrimPrefix(strings.Fields(line)[0], ">")
			currentSeq.Reset()
		} else {
			currentSeq.WriteString(line)
		}
	}
	// Save last
	if currentID != "" {
		seqs[currentID] = currentSeq.String()
	}
	return seqs, nil
}

// loadLogoAsBase64 loads an image file and converts it to base64 data URI
func loadLogoAsBase64(logoPath string) string {
	data, err := os.ReadFile(logoPath)
	if err != nil {
		return ""
	}
	ext := filepath.Ext(logoPath)
	mimeType := "image/png"
	if ext == ".jpg" || ext == ".jpeg" {
		mimeType = "image/jpeg"
	} else if ext == ".svg" {
		mimeType = "image/svg+xml"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
}

// getPipelineInfo extracts pipeline execution information from config
func getPipelineInfo(cfg *config.Config) PipelineInfo {
	params := make(map[string]string)

	// Add key parameters
	params["Input Files"] = filepath.Base(cfg.File1)
	if cfg.File2 != "" {
		params["Input Files"] += ", " + filepath.Base(cfg.File2)
	}
	if cfg.Reference != "" {
		params["Reference Genome"] = filepath.Base(cfg.Reference)
	}
	if cfg.GTF != "" {
		params["Annotation"] = filepath.Base(cfg.GTF)
	}
	params["CPU Threads"] = fmt.Sprintf("%d", cfg.Threads)

	// Add optional flags
	if cfg.EnableSortMeRNA {
		params["rRNA Filtering"] = "Enabled"
	}
	if cfg.SkipCPAT {
		params["CPAT"] = "Skipped (CPC2 only)"
	}

	return PipelineInfo{
		Mode:       cfg.Method,
		DataType:   cfg.DataType,
		Species:    cfg.Species,
		Parameters: params,
	}
}

// safeBase returns the base name of a path, or empty string if path is empty or "."
func safeBase(path string) string {
	if path == "" || path == "." {
		return "N/A"
	}
	return filepath.Base(path)
}

// getModuleDetails constructs the rich module data for the report
func getModuleDetails(s *PipelineSummary, cfg *config.Config) []ModuleRenderData {
	var modules []ModuleRenderData

	// Helper to find duration
	getDuration := func(name string) string {
		for _, t := range s.StepTimings {
			if strings.Contains(t.StepName, name) {
				return t.Duration
			}
		}
		return "N/A"
	}

	// 1. FastQC
	var fastqcLinks []Link
	fastqcDir := filepath.Join(cfg.OutputDir, "01_fastqc")
	if files, err := os.ReadDir(fastqcDir); err == nil {
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".html") {
				fastqcLinks = append(fastqcLinks, Link{
					Label: "View Report: " + f.Name(),
					URL:   "assets/fastqc/" + f.Name(),
				})
			}
		}
	}

	modules = append(modules, ModuleRenderData{
		ID:       1,
		Name:     "Quality Control with FastQC",
		Icon:     "🔍",
		Duration: getDuration("FastQC"),
		Status:   "Success",
		Description: []string{
			"Assessed per-base quality scores",
			"Checked adapter contamination",
			"Analyzed GC content distribution",
		},
		Inputs: []string{
			safeBase(cfg.File1),
			safeBase(cfg.File2),
		},
		Outputs: []string{
			"01_fastqc/*.html",
			"01_fastqc/*.zip",
		},
		Metrics:    s.FastQC,
		ExtraLinks: fastqcLinks,
	})

	// 2. Trimmomatic
	modules = append(modules, ModuleRenderData{
		ID:       2,
		Name:     "Adapter Trimming with Trimmomatic",
		Icon:     "✂️",
		Duration: getDuration("Trimmomatic"),
		Status:   "Success",
		Description: []string{
			"Removed Illumina adapter sequences",
			"Trimmed low-quality bases (Q<20)",
			"Filtered reads <36bp minimum length",
		},
		Inputs: []string{
			"Raw FASTQ files",
			"TruSeq3-PE adapter sequences",
		},
		Outputs: []string{
			"02_trimming/paired_1.fastq",
			"02_trimming/paired_2.fastq",
		},
		Metrics: s.Trimmomatic,
	})

	// 3. SortMeRNA
	modules = append(modules, ModuleRenderData{
		ID:       3,
		Name:     "rRNA Filtering with SortMeRNA",
		Icon:     "🧹",
		Duration: getDuration("SortMeRNA"),
		Status:   "Success",
		Description: []string{
			"Identified ribosomal RNA sequences",
			"Separated rRNA from mRNA/lncRNA reads",
			"Database: SILVA 138 + Rfam",
		},
		Inputs: []string{
			"Trimmed paired-end reads",
		},
		Outputs: []string{
			"03_sortmerna/filtered_fwd.fq",
			"03_sortmerna/filtered_rev.fq",
			"03_sortmerna/rrna_fwd.fq",
		},
		Metrics: s.SortMeRNA,
	})

	// 4. Alignment/Assembly (Mode Dependent)
	if cfg.Method == "denovo" {
		modules = append(modules, ModuleRenderData{
			ID:       4,
			Name:     "De novo Assembly with Trinity",
			Icon:     "🧬",
			Duration: getDuration("Trinity"),
			Status:   "Success",
			Description: []string{
				"Assembled transcripts without reference",
				"Generated de novo transcriptome",
			},
			Inputs: []string{
				"rRNA-filtered reads",
			},
			Outputs: []string{
				"05_assembly/transcripts.gtf",
				"05_assembly/Trinity.fasta",
			},
			Metrics: s.HISAT2,
		})
	} else {
		modules = append(modules, ModuleRenderData{
			ID:       4,
			Name:     "Reference-based Alignment with HISAT2",
			Icon:     "🧬",
			Duration: getDuration("HISAT2"),
			Status:   "Success",
			Description: []string{
				"Aligned reads to reference genome",
				"Converted SAM → BAM format",
				"Sorted and indexed BAM file",
				"Assembled transcripts with StringTie",
			},
			Inputs: []string{
				"rRNA-filtered reads",
				filepath.Base(cfg.Reference),
			},
			Outputs: []string{
				"04_alignment/aligned_sorted.bam",
				"05_assembly/transcripts.gtf",
			},
			Metrics: s.HISAT2,
		})
	}

	// 5. CPC2
	var cpc2Rows []CPC2Row
	cpc2File := filepath.Join(cfg.OutputDir, "06_cpc2", "cpc2_output.txt")
	allRows, _, err := parseCPC2Output(cpc2File)

	totalRows := 0
	if err == nil {
		totalRows = len(allRows)
		if totalRows > MaxTableRows {
			cpc2Rows = allRows[:MaxTableRows]
		} else {
			cpc2Rows = allRows
		}
	}

	modules = append(modules, ModuleRenderData{
		ID:       5,
		Name:     "Coding Potential with CPC2",
		Icon:     "💻",
		Duration: getDuration("CPC2"),
		Status:   "Success",
		Description: []string{
			"Machine learning classification (SVM)",
			"Analyzed Fickett score, pI, ORF integrity",
		},
		Inputs: []string{
			"06_cpc2/transcripts.fa",
		},
		Outputs: []string{
			"06_cpc2/cpc2_output.txt",
		},
		Metrics:   s.CPC2,
		TableData: cpc2Rows,
		TotalRows: totalRows,
	})

	// 6. CPAT
	modules = append(modules, ModuleRenderData{
		ID:       6,
		Name:     "Cross-Validation with CPAT",
		Icon:     "⚖️",
		Duration: getDuration("CPAT"),
		Status:   "Success",
		Description: []string{
			"Independent coding potential prediction",
			"Logistic regression with hexamer frequencies",
		},
		Inputs: []string{
			"06_cpc2/transcripts.fa",
		},
		Outputs: []string{
			"07_validation/cpat_output.ORF_prob.best.tsv",
			"07_validation/consensus_noncoding.txt",
		},
		Metrics: s.CPAT,
	})

	// 7. LncRNA Filtering
	modules = append(modules, ModuleRenderData{
		ID:       7,
		Name:     "lncRNA Filtering and Classification",
		Icon:     "🎯",
		Duration: getDuration("Filtering"),
		Status:   "Success",
		Description: []string{
			"Length >200nt, Coding Prob <0.5",
			"Consensus validation (CPC2 + CPAT)",
			"Classification with gffcompare",
			"Expression quantification (featureCounts)",
		},
		Inputs: []string{
			"Consensus noncoding transcripts",
			"Reference annotation",
		},
		Outputs: []string{
			"08_lncrna_analysis/filtered/lncrna.bed",
			"08_lncrna_analysis/novel_lncrnas/*.gtf",
			"08_lncrna_analysis/expression/lncrna_counts.txt",
		},
		Metrics: s.LncRNAFiltering,
	})

	// 8. RNAfold
	var rnaImages []ReportImage
	svgFolder := filepath.Join(cfg.OutputDir, "09_rnafold", "svg_files")
	files, _ := os.ReadDir(svgFolder)

	// Load sequences & CPC2 data for modal
	seqs, _ := parseFasta(filepath.Join(cfg.OutputDir, "06_cpc2", "transcripts.fa"))
	_, cpc2Map, _ := parseCPC2Output(filepath.Join(cfg.OutputDir, "06_cpc2", "cpc2_output.txt"))

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".svg") {
			// Extract Label (filename without extension, e.g. STRG.1.1)
			label := strings.TrimSuffix(f.Name(), "_ss.svg")

			img := ReportImage{
				Path:     "assets/rnafold/" + f.Name(),
				Label:    label,
				Sequence: seqs[label],
			}

			// Attach CPC2 data if available
			if row, ok := cpc2Map[label]; ok {
				img.CPC2Data = &row
			}

			rnaImages = append(rnaImages, img)
		}
	}

	// Limit images
	totalImages := len(rnaImages)
	if totalImages > MaxCarouselImages {
		rnaImages = rnaImages[:MaxCarouselImages]
	}

	modules = append(modules, ModuleRenderData{
		ID:       8,
		Name:     "Secondary Structure Prediction",
		Icon:     "🌀",
		Duration: getDuration("RNAfold"),
		Status:   "Success",
		Description: []string{
			"Predicted minimum free energy (MFE) structures",
			"Generated visualization images",
		},
		Inputs: []string{
			"06_cpc2/transcripts.fa",
		},
		Outputs: []string{
			"09_rnafold/lncrna_structures.out",
			"09_rnafold/svg_files/*.svg",
		},
		Metrics:     s.RNAfold,
		Images:      rnaImages,
		TotalImages: totalImages,
	})

	modules = append(modules, ModuleRenderData{
		ID:       8,
		Name:     "Secondary Structure Prediction",
		Icon:     "🌀",
		Duration: getDuration("RNAfold"),
		Status:   "Success",
		Description: []string{
			"Predicted minimum free energy (MFE) structures",
			"Generated visualization images",
		},
		Inputs: []string{
			"06_cpc2/transcripts.fa",
		},
		Outputs: []string{
			"09_rnafold/lncrna_structures.out",
			"09_rnafold/svg_files/*.svg",
		},
		Metrics: s.RNAfold,
		Images:  rnaImages,
	})

	// 9. LncTar
	modules = append(modules, ModuleRenderData{
		ID:       9,
		Name:     "LncTar Target Prediction",
		Icon:     "🎯",
		Duration: getDuration("LncTar"),
		Status:   "Success",
		Description: []string{
			"Thermodynamic modeling of RNA-RNA interactions",
			"Mode: Best candidates only",
		},
		Inputs: []string{
			"Best candidate lncRNAs",
			"All mRNA sequences",
		},
		Outputs: []string{
			"11_target_prediction/lnctar/best_candidates_targets.txt",
		},
		Metrics: s.LncTar,
	})

	// 10. IntaRNA
	modules = append(modules, ModuleRenderData{
		ID:       10,
		Name:     "IntaRNA Cross-Validation",
		Icon:     "🧪",
		Duration: getDuration("IntaRNA"),
		Status:   "Success",
		Description: []string{
			"Accessibility-based interaction prediction",
			"Independent validation of LncTar",
		},
		Inputs: []string{
			"Best candidate lncRNAs",
		},
		Outputs: []string{
			"11_target_prediction/intarna/best_candidates_targets.csv",
		},
		Metrics: s.IntaRNA,
	})

	// 11. Consensus
	modules = append(modules, ModuleRenderData{
		ID:       11,
		Name:     "Consensus Target Analysis",
		Icon:     "✅",
		Duration: getDuration("Consensus"),
		Status:   "Success",
		Description: []string{
			"Identified pairs predicted by both tools",
			"High-confidence target generation",
		},
		Inputs: []string{
			"LncTar predictions",
			"IntaRNA predictions",
		},
		Outputs: []string{
			"11_target_prediction/consensus_pairs.txt",
		},
		Metrics: s.Consensus,
	})

	// 12. Enrichment
	modules = append(modules, ModuleRenderData{
		ID:       12,
		Name:     "Enrichment Gene List Building",
		Icon:     "📊",
		Duration: getDuration("Enrichment"),
		Status:   "Success",
		Description: []string{
			"Found genes near lncRNA loci",
			"Mapped targets to reference genes",
			"Created background gene set",
		},
		Inputs: []string{
			"lncRNA BED file",
			"Reference GTF",
		},
		Outputs: []string{
			"12_enrichment/genes_associated_with_lncRNAs_combined.txt",
			"12_enrichment/all_genes_background.txt",
		},
		Metrics: s.Enrichment,
	})

	// 13. RSeQC
	var rseqcLink []Link
	rseqcDir := filepath.Join(cfg.OutputDir, "13_rseqc")
	rseqcFiles, _ := os.ReadDir(rseqcDir)
	for _, f := range rseqcFiles {
		if strings.HasSuffix(f.Name(), ".pdf") {
			rseqcLink = append(rseqcLink, Link{
				Label: "View Coverage Plot (PDF)",
				URL:   "assets/rseqc/" + f.Name(),
			})
			break
		}
	}

	modules = append(modules, ModuleRenderData{
		ID:       13,
		Name:     "RNA-seq Quality Assessment (RSeQC)",
		Icon:     "📉",
		Duration: getDuration("RSeQC"),
		Status:   "Partial Success",
		Description: []string{
			"Analyzed gene body coverage",
			"Assessed read distribution",
			"Inferred strand specificity",
		},
		Inputs: []string{
			"04_alignment/aligned_sorted.bam",
		},
		Outputs: []string{
			"13_rseqc/gene_body_coverage.*",
			"13_rseqc/read_distribution.txt",
		},
		Metrics:    s.RSeQC,
		ExtraLinks: rseqcLink,
	})

	// 14. IGV
	modules = append(modules, ModuleRenderData{
		ID:       14,
		Name:     "IGV Genome Browser Report",
		Icon:     "🖼️",
		Duration: getDuration("IGV"),
		Status:   "Success",
		Description: []string{
			"Created interactive HTML report",
			"Configured tracks: lncRNA, BAM, Reference",
		},
		Inputs: []string{
			"08_lncrna_analysis/filtered/lncrna.bed",
			"04_alignment/aligned_sorted.bam",
		},
		Outputs: []string{
			"14_igv_report/lncrna_igv_report.html",
		},
		Metrics: s.IGVReport,
	})

	// 15. MultiQC
	modules = append(modules, ModuleRenderData{
		ID:       15,
		Name:     "MultiQC Report Generation",
		Icon:     "📈",
		Duration: getDuration("MultiQC"),
		Status:   "Success",
		Description: []string{
			"Aggregated data from all tools",
			"Generated publication-ready QC report",
		},
		Inputs: []string{
			"All output directories",
		},
		Outputs: []string{
			"15_multiqc/lncrna_pipeline_report.html",
		},
		Metrics: s.MultiQC,
	})

	return modules
}

// generateHTMLDashboard creates a modern interactive HTML dashboard
func generateHTMLDashboard(summary *PipelineSummary, reportDir string, cfg *config.Config) error {
	htmlFile := filepath.Join(reportDir, "pipeline_summary.html")

	// Copy assets to report directory
	if err := copyAssetsToReportDir(cfg, reportDir); err != nil {
		fmt.Printf("Warning: Failed to copy assets to report directory: %v\n", err)
	}

	// Get pipeline info
	pipelineInfo := getPipelineInfo(cfg)

	// Prepare detailed module data
	modules := getModuleDetails(summary, cfg)

	// Prepare template data
	data := struct {
		*PipelineSummary
		Modules      []ModuleRenderData
		PipelineInfo PipelineInfo
	}{
		PipelineSummary: summary,
		Modules:         modules,
		PipelineInfo:    pipelineInfo,
	}

	// HTML template - Using DaisyUI for cleaner, more efficient code
	tmpl := getHTMLTemplate()

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(htmlFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}

// getHTMLTemplate returns the complete HTML template
func getHTMLTemplate() string {
	return `<!DOCTYPE html>
<html lang="en" class="scroll-smooth">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>REGIS Pipeline Report - {{.RunID}}</title>
    <link href="https://cdn.jsdelivr.net/npm/daisyui@3.7.3/dist/full.css" rel="stylesheet" type="text/css" />
    <script src="https://cdn.tailwindcss.com"></script>
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');
        body { font-family: 'Inter', sans-serif; }
        .timeline-line {
            position: absolute;
            left: 2rem;
            top: 0;
            bottom: 0;
            width: 2px;
            background-color: #e5e7eb;
            z-index: 0;
        }
        .break-all-paths {
            word-break: break-all;
        }
    </style>
</head>
<body class="bg-gray-50 min-h-screen font-sans text-gray-900">
    
    <!-- Header -->
    <header class="bg-white shadow-sm sticky top-0 z-50">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
            <div class="flex items-center justify-between">
                <div class="flex items-center space-x-4">
                    <div class="flex flex-col">
                        <h1 class="text-2xl font-bold text-gray-900 tracking-tight">REGIS Pipeline</h1>
                        <div class="text-xs text-gray-500 font-medium">RNA-seq Guided Identification System</div>
                    </div>
                    
                    <div class="h-8 w-px bg-gray-200 mx-2"></div>

                    <div>
                        <div class="flex items-center space-x-3 text-sm text-gray-500">
                            <span>Run ID: <span class="font-mono text-gray-700">{{.RunID}}</span></span>
                            <span>•</span>
                            <span>{{.Status}}</span>
                            <span>•</span>
                            <span>{{.TotalDuration}}</span>
                        </div>
                    </div>
                </div>
                <div class="flex items-center space-x-4 text-sm">
                    <a href="https://github.com/BioinformaticsOnLine/regis" target="_blank" class="text-blue-600 hover:text-blue-800 font-medium transition-colors">Documentation</a>
                    <a href="https://github.com/pranjalpruthi" target="_blank" class="text-gray-500 hover:text-gray-700 transition-colors">Developer</a>
                    <span class="text-gray-300">|</span>
                    <span class="text-gray-400">v1.0</span>
                </div>
            </div>
        </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
        
        <!-- Executive Summary Cards -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
            <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-5 hover:shadow-md transition-shadow group">
                <div class="text-sm font-medium text-gray-500 uppercase tracking-wide group-hover:text-blue-600 transition-colors">lncRNAs Discovered</div>
                <div class="text-3xl font-bold text-blue-600 mt-1">{{.TotalLncRNAs}}</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-5 hover:shadow-md transition-shadow group">
                <div class="text-sm font-medium text-gray-500 uppercase tracking-wide group-hover:text-purple-600 transition-colors">Novel Intergenic</div>
                <div class="text-3xl font-bold text-purple-600 mt-1">{{.NovelLncRNAs}}</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-5 hover:shadow-md transition-shadow group">
                <div class="text-sm font-medium text-gray-500 uppercase tracking-wide group-hover:text-green-600 transition-colors">Highly Expressed</div>
                <div class="text-3xl font-bold text-green-600 mt-1">{{.HighlyExpressed}}</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-5 hover:shadow-md transition-shadow group">
                <div class="text-sm font-medium text-gray-500 uppercase tracking-wide group-hover:text-orange-600 transition-colors">Best Candidates</div>
                <div class="text-3xl font-bold text-orange-600 mt-1">{{.BestCandidates}}</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-5 hover:shadow-md transition-shadow group">
                <div class="text-sm font-medium text-gray-500 uppercase tracking-wide group-hover:text-pink-600 transition-colors">Associated Genes</div>
                <div class="text-3xl font-bold text-pink-600 mt-1">{{.AssociatedGenes}}</div>
            </div>
        </div>

        <!-- Quick Links & Timing -->
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <!-- Quick Links -->
            <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-6">
                <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
                    <span class="bg-blue-100 text-blue-600 p-2 rounded-lg mr-3">🔗</span>
                    Quick Links
                </h2>
                <div class="space-y-3">
                    <a href="assets/multiqc/lncrna_pipeline_report.html" target="_blank" 
                       class="flex items-center justify-between p-3 bg-blue-50 hover:bg-blue-100 text-blue-700 rounded-lg transition-colors group border border-blue-100">
                        <span class="font-medium">MultiQC Dashboard</span>
                        <span class="text-blue-400 group-hover:text-blue-600 transition-colors">→</span>
                    </a>
                    <a href="assets/igv/lncrna_igv_report.html" target="_blank"
                       class="flex items-center justify-between p-3 bg-purple-50 hover:bg-purple-100 text-purple-700 rounded-lg transition-colors group border border-purple-100">
                        <span class="font-medium">IGV Genome Browser</span>
                        <span class="text-purple-400 group-hover:text-purple-600 transition-colors">→</span>
                    </a>
                    <div class="grid grid-cols-2 gap-3">
                        <a href="pipeline_summary.json" download
                           class="flex items-center justify-center p-3 bg-gray-50 hover:bg-green-50 text-gray-600 hover:text-green-700 rounded-lg transition-colors text-sm font-medium border border-gray-100 hover:border-green-200">
                            ⬇ JSON
                        </a>
                        <a href="pipeline_summary.md" download
                           class="flex items-center justify-center p-3 bg-gray-50 hover:bg-gray-100 text-gray-600 hover:text-gray-900 rounded-lg transition-colors text-sm font-medium border border-gray-100 hover:border-gray-200">
                            ⬇ Markdown
                        </a>
                    </div>
                </div>
            </div>

            <!-- Timing Chart -->
            <div class="lg:col-span-2 bg-white rounded-xl shadow-sm border border-gray-100 p-6">
                <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
                    <span class="bg-blue-100 text-blue-600 p-2 rounded-lg mr-3">⏱️</span>
                    Pipeline Timing
                </h2>
                <div class="h-64">
                    <canvas id="timingChart"></canvas>
                </div>
            </div>
        </div>

        <!-- lncRNA Classification Chart -->
        {{if .LncRNAFiltering}}
        {{if .LncRNAFiltering.AllClassCodes}}
        <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-6">
            <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
                <span class="bg-purple-100 text-purple-600 p-2 rounded-lg mr-3">🧬</span>
                lncRNA Classification (gffcompare)
            </h2>
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
                <!-- Chart -->
                <div class="flex items-center justify-center">
                    <div class="w-full max-w-md">
                        <canvas id="classCodeChart"></canvas>
                    </div>
                </div>
                <!-- Table -->
                <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-200 border border-gray-100 rounded-lg overflow-hidden">
                        <thead class="bg-gray-50">
                            <tr>
                                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Code</th>
                                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Count</th>
                                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Description</th>
                            </tr>
                        </thead>
                        <tbody class="bg-white divide-y divide-gray-200">
                            {{range $code, $info := .LncRNAFiltering.AllClassCodes}}
                            <tr class="hover:bg-gray-50 transition-colors">
                                <td class="px-6 py-4 whitespace-nowrap">
                                    <span class="px-2 py-1 text-sm font-mono font-semibold bg-blue-50 text-blue-700 rounded border border-blue-100">{{$info.Code}}</span>
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm font-semibold text-gray-900">{{$info.Count}}</td>
                                <td class="px-6 py-4 text-sm text-gray-700">{{$info.Description}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
        {{end}}
        {{end}}

        <!-- Detailed Stage-by-Stage Analysis -->
        <div class="space-y-6 relative">
            <h2 class="text-2xl font-bold text-gray-900 pl-2">Stage-by-Stage Analysis</h2>
            
            <div class="timeline-line hidden md:block"></div>

            {{range .Modules}}
            <div class="relative pl-0 md:pl-16 group">
                <div class="hidden md:flex absolute left-6 top-6 w-4 h-4 bg-white border-4 border-blue-500 rounded-full z-10 group-hover:scale-125 transition-transform shadow-sm"></div>

                <div class="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden hover:shadow-md transition-shadow">
                    <!-- Card Header -->
                    <div class="bg-gray-50/50 px-6 py-4 border-b border-gray-100 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                        <div class="flex items-center gap-4">
                            <span class="flex items-center justify-center w-10 h-10 bg-white rounded-lg shadow-sm text-xl border border-gray-100">
                                {{.Icon}}
                            </span>
                            <div>
                                <h3 class="text-lg font-bold text-gray-900">Stage {{.ID}}: {{.Name}}</h3>
                                <div class="flex items-center gap-3 text-sm text-gray-500 mt-1">
                                    <span class="flex items-center gap-1 bg-gray-100 px-2 py-0.5 rounded text-xs font-medium">
                                        ⏱️ {{.Duration}}
                                    </span>
                                    <span>•</span>
                                    <span class="flex items-center gap-1 {{if eq .Status "Success"}}text-green-700 bg-green-50 border border-green-100{{else}}text-yellow-700 bg-yellow-50 border border-yellow-100{{end}} px-2 py-0.5 rounded text-xs font-medium">
                                        {{if eq .Status "Success"}}✅{{else}}⚠️{{end}} {{.Status}}
                                    </span>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Card Body -->
                    <div class="p-6 grid grid-cols-1 lg:grid-cols-3 gap-8">
                        
                        <!-- Process & Description -->
                        <div class="lg:col-span-1 space-y-4">
                            <div>
                                <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">Process</h4>
                                <ul class="space-y-2">
                                    {{range .Description}}
                                    <li class="flex items-start gap-2 text-sm text-gray-700">
                                        <span class="text-blue-400 mt-0.5">•</span>
                                        {{.}}
                                    </li>
                                    {{end}}
                                </ul>
                            </div>
                            
                            {{if .ExtraLinks}}
                            <div class="pt-2 space-y-2">
                                {{range .ExtraLinks}}
                                <a href="{{.URL}}" target="_blank" class="inline-flex items-center px-3 py-1.5 bg-blue-50 text-blue-700 hover:bg-blue-100 rounded-md text-sm font-medium transition-colors">
                                    📄 {{.Label}}
                                </a>
                                {{end}}
                            </div>
                            {{end}}
                        </div>

                        <!-- Inputs & Outputs -->
                        <div class="lg:col-span-1 space-y-4">
                            <div>
                                <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">Input</h4>
                                <div class="space-y-1">
                                    {{range .Inputs}}
                                    <div class="text-sm text-gray-600 font-mono bg-gray-50 px-2 py-1 rounded border border-gray-100 break-all-paths" title="{{.}}">
                                        {{.}}
                                    </div>
                                    {{end}}
                                </div>
                            </div>
                            <div>
                                <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">Output</h4>
                                <div class="space-y-1">
                                    {{range .Outputs}}
                                    <div class="text-sm text-gray-600 font-mono bg-gray-50 px-2 py-1 rounded border border-gray-100 break-all-paths" title="{{.}}">
                                        {{.}}
                                    </div>
                                    {{end}}
                                </div>
                            </div>
                        </div>

                        <!-- Key Metrics -->
                        <div class="lg:col-span-1 bg-blue-50/50 rounded-lg p-4 border border-blue-100">
                            <h4 class="text-xs font-semibold text-blue-600 uppercase tracking-wider mb-3">Key Metrics</h4>
                            <div class="space-y-3">
                                {{if .Metrics}}
                                    {{if eq .Name "Quality Control with FastQC"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Sequences:</span> <span class="font-medium">{{.TotalSequences}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">GC Content:</span> <span class="font-medium">{{printf "%.1f" .GCContent}}%</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Adapter Trimming with Trimmomatic"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Input Pairs:</span> <span class="font-medium">{{.InputReadPairs}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Surviving:</span> <span class="font-medium">{{.BothSurviving}}</span></div>
                                        <div class="mb-1">
                                            <div class="flex justify-between text-sm mb-1"><span class="text-gray-600">Survival Rate:</span> <span class="font-medium">{{printf "%.1f" .SurvivalRate}}%</span></div>
                                            <div class="w-full bg-gray-200 rounded-full h-1.5">
                                                <div class="bg-green-500 h-1.5 rounded-full" style="width: {{.SurvivalRate}}%"></div>
                                            </div>
                                        </div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "rRNA Filtering with SortMeRNA"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Total Reads:</span> <span class="font-medium">{{.TotalReads}}</span></div>
                                        <div class="mb-1">
                                            <div class="flex justify-between text-sm mb-1"><span class="text-gray-600">rRNA Removed:</span> <span class="font-medium">{{printf "%.1f" .RRNAPercent}}%</span></div>
                                            <div class="w-full bg-gray-200 rounded-full h-1.5">
                                                <div class="bg-red-400 h-1.5 rounded-full" style="width: {{.RRNAPercent}}%"></div>
                                            </div>
                                        </div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Reference-based Alignment with HISAT2"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Total Pairs:</span> <span class="font-medium">{{.TotalReadPairs}}</span></div>
                                        {{if gt .ConcordantTotal 0}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Concordant:</span> <span class="font-medium">{{.ConcordantTotal}} ({{printf "%.1f" .ConcordantPercent}}%)</span></div>
                                        {{end}}
                                        <div class="mb-1">
                                            <div class="flex justify-between text-sm mb-1"><span class="text-gray-600">Alignment Rate:</span> <span class="font-medium">{{printf "%.2f" .OverallAlignmentRate}}%</span></div>
                                            <div class="w-full bg-gray-200 rounded-full h-1.5">
                                                <div class="bg-blue-500 h-1.5 rounded-full" style="width: {{.OverallAlignmentRate}}%"></div>
                                            </div>
                                        </div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Coding Potential with CPC2"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Total:</span> <span class="font-medium">{{.TotalTranscripts}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Coding:</span> <span class="font-medium text-red-600">{{.CodingTranscripts}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Noncoding:</span> <span class="font-medium text-green-600">{{.NoncodingTranscripts}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Cross-Validation with CPAT"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Consensus Noncoding:</span> <span class="font-medium text-green-600">{{.ConsensusNoncoding}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "lncRNA Filtering and Classification"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Final lncRNAs:</span> <span class="font-medium">{{.FinalLncRNAs}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Novel Intergenic:</span> <span class="font-medium">{{.NovelIntergenic}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Best Candidates:</span> <span class="font-medium text-orange-600">{{.BestCandidates}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Secondary Structure Prediction"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Structures:</span> <span class="font-medium">{{.TotalStructures}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "LncTar Target Prediction"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Targets:</span> <span class="font-medium">{{.TotalTargets}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "IntaRNA Cross-Validation"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Targets:</span> <span class="font-medium">{{.TotalTargets}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Consensus Target Analysis"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Consensus Pairs:</span> <span class="font-medium text-green-600">{{.ConsensusPairs}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "RSeQC Quality Assessment"}}
                                        {{with .Metrics}}
                                        {{if .ReadDistributionDone}}
                                            <div class="flex justify-between text-sm"><span class="text-gray-600">CDS Exons:</span> <span class="font-medium">{{.CDSExons}}</span></div>
                                            <div class="flex justify-between text-sm"><span class="text-gray-600">Introns:</span> <span class="font-medium">{{.Introns}}</span></div>
                                            <div class="flex justify-between text-sm"><span class="text-gray-600">TSS Up 1kb:</span> <span class="font-medium">{{.TSSUp1kb}}</span></div>
                                        {{else}}
                                            <div class="text-sm text-gray-500 italic">Metrics not available</div>
                                        {{end}}
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "Enrichment Gene List Building"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Associated Genes:</span> <span class="font-medium">{{.TotalAssociatedGenes}}</span></div>
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Background:</span> <span class="font-medium">{{.TotalBackgroundGenes}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "IGV Genome Browser Report"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Loci Visualized:</span> <span class="font-medium">{{.TotalLoci}}</span></div>
                                        {{end}}
                                    {{end}}

                                    {{if eq .Name "MultiQC Report Generation"}}
                                        {{with .Metrics}}
                                        <div class="flex justify-between text-sm"><span class="text-gray-600">Status:</span> <span class="font-medium">{{if .ReportGenerated}}Generated{{else}}Failed{{end}}</span></div>
                                        {{end}}
                                    {{end}}

                                {{else}}
                                    <div class="text-sm text-gray-500 italic">No metrics available</div>
                                {{end}}
                            </div>
                        </div>
                    </div>

                        <!-- CPC2 Table -->
                        {{if .TableData}}
                        <div class="lg:col-span-3 mt-6">
                            <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Analysis Results</h4>
                            
                             {{if gt .TotalRows (len .TableData)}}
                            <div class="alert alert-warning text-xs py-2 mb-2 flex justify-between items-center rounded-lg border border-yellow-200 bg-yellow-50 text-yellow-800">
                                <span>Showing first {{len .TableData}} of {{.TotalRows}} rows. Large datasets are truncated for performance.</span>
                                {{range .Outputs}}
                                    <span class="font-mono bg-white px-2 py-0.5 rounded border border-yellow-200">Full file: {{.}}</span>
                                {{end}}
                            </div>
                            {{end}}

                            <div class="overflow-x-auto max-h-96 border border-gray-100 rounded-lg">
                                <table class="min-w-full divide-y divide-gray-200 text-sm">
                                    <thead class="bg-gray-50 sticky top-0">
                                        <tr>
                                            <th class="px-4 py-2 text-left font-medium text-gray-500">ID</th>
                                            <th class="px-4 py-2 text-left font-medium text-gray-500">Length</th>
                                            <th class="px-4 py-2 text-left font-medium text-gray-500">Fickett</th>
                                            <th class="px-4 py-2 text-left font-medium text-gray-500">pI</th>
                                            <th class="px-4 py-2 text-left font-medium text-gray-500">Prob</th>
                                            <th class="px-4 py-2 text-left font-medium text-gray-500">Label</th>
                                        </tr>
                                    </thead>
                                    <tbody class="bg-white divide-y divide-gray-200">
                                        {{range .TableData}}
                                        <tr class="hover:bg-gray-50">
                                            <td class="px-4 py-2 font-mono text-gray-900">{{.ID}}</td>
                                            <td class="px-4 py-2 text-gray-600">{{.TranscriptLength}}</td>
                                            <td class="px-4 py-2 text-gray-600">{{.FickettScore}}</td>
                                            <td class="px-4 py-2 text-gray-600">{{printf "%.2s" .PI}}</td> <!-- truncate if needed -->
                                            <td class="px-4 py-2 text-gray-600">{{.CodingProb}}</td>
                                            <td class="px-4 py-2">
                                                <span class="px-2 py-0.5 rounded text-xs font-medium {{if eq .Label "noncoding"}}bg-green-100 text-green-800{{else}}bg-red-100 text-red-800{{end}}">
                                                    {{.Label}}
                                                </span>
                                            </td>
                                        </tr>
                                        {{end}}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                        {{end}}

                        <!-- Images (e.g. RNAfold) -->
                        {{if .Images}}
                    <div class="px-6 pb-6">
                        <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Structure Visualization Carousel</h4>
                        
                         {{if gt .TotalImages (len .Images)}}
                        <div class="alert alert-info text-xs py-2 mb-2 rounded-lg bg-blue-50 text-blue-800 border border-blue-200">
                           <span>Showing first {{len .Images}} of {{.TotalImages}} structures. Check <code>09_rnafold/svg_files/</code> for all.</span>
                        </div>
                        {{end}}

                        <div class="carousel carousel-center max-w-full p-4 space-x-4 bg-gray-50 rounded-box border border-gray-100">
                            {{range .Images}}
                            <div class="carousel-item flex flex-col items-center">
                                <div class="relative group cursor-pointer" 
                                     onclick='openImageModal("{{.Path}}", "{{.Label}}", "{{.Sequence}}", {{if .CPC2Data}}{"prob": "{{.CPC2Data.CodingProb}}", "label": "{{.CPC2Data.Label}}", "len": "{{.CPC2Data.TranscriptLength}}"}{{else}}null{{end}})'>
                                    <img src="{{.Path}}" class="rounded-box h-64 object-contain bg-white border border-gray-100 p-2 hover:scale-105 transition-transform" />
                                    <div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity bg-black/10 rounded-box">
                                        <span class="bg-black/75 text-white text-xs px-2 py-1 rounded">View Details</span>
                                    </div>
                                </div>
                                <span class="text-xs font-mono text-gray-500 mt-2 bg-gray-100 px-2 py-0.5 rounded border border-gray-200">{{.Label}}</span>
                            </div>
                            {{end}}
                        </div>
                        <p class="text-xs text-center text-gray-400 mt-2">Swipe or click to view full size</p>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>

        <!-- Pipeline Execution Breakdown Table -->
        <div class="bg-white rounded-xl shadow-sm border border-gray-100 p-6">
            <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
                <span class="bg-gray-100 text-gray-600 p-2 rounded-lg mr-3">📋</span>
                Pipeline Execution Breakdown
            </h2>
            <div class="overflow-x-auto">
                <table class="min-w-full divide-y divide-gray-200 border border-gray-100 rounded-lg overflow-hidden">
                    <thead class="bg-gray-50">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Step</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Module</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Duration</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200">
                        {{range $index, $module := .Modules}}
                        <tr class="hover:bg-gray-50 transition-colors">
                            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{{$module.ID}}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-700">{{$module.Name}}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-600">{{$module.Duration}}</td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {{if eq $module.Status "Success"}}bg-green-100 text-green-800{{else}}bg-yellow-100 text-yellow-800{{end}}">
                                    {{if eq $module.Status "Success"}}✓ Success{{else}}⚠️ {{$module.Status}}{{end}}
                                </span>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>

    </main>

    <!-- Footer -->
    <footer class="bg-white border-t border-gray-200 mt-12">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
            <div class="flex flex-col md:flex-row justify-between items-center gap-4">
                <div class="flex items-center gap-3">
                    <div class="text-sm font-bold text-gray-400">REGIS</div>
                    <p class="text-sm text-gray-500">JN Lab • Dev: Pranjal Pruthi</p>
                </div>
                <p class="text-sm text-gray-400">{{.EndTime.Format "2006-01-02 15:04:05"}}</p>
            </div>
        </div>
    </footer>

    <!-- Image Modal -->
    <dialog id="image_modal" class="modal backdrop-blur-sm">
        <form method="dialog" class="modal-box w-11/12 max-w-6xl h-[90vh] flex bg-white p-0 overflow-hidden shadow-2xl rounded-2xl">
            
            <!-- Left: Image -->
            <div class="w-2/3 h-full bg-gray-50 flex flex-col border-r border-gray-100">
                <div class="flex justify-between items-center p-4 border-b border-gray-100">
                    <h3 id="modal_label" class="font-bold text-lg font-mono text-gray-800"></h3>
                </div>
                <div class="flex-1 flex items-center justify-center p-8 overflow-auto">
                    <img id="modal_image" src="" alt="Structure" class="max-w-full max-h-full object-contain drop-shadow-lg" />
                </div>
                <div class="p-4 border-t border-gray-100 bg-white flex justify-end">
                    <a id="modal_download" href="" download class="btn btn-sm btn-primary">Download SVG</a>
                </div>
            </div>

            <!-- Right: Details -->
            <div class="w-1/3 h-full bg-white flex flex-col overflow-y-auto">
                <div class="flex justify-end p-2">
                    <button class="btn btn-sm btn-circle btn-ghost text-gray-500 hover:bg-gray-100">✕</button>
                </div>
                
                <div class="px-6 pb-6 space-y-6">
                    <!-- CPC2 Data -->
                    <div id="modal_cpc2_section" class="hidden">
                        <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">Coding Potential (CPC2)</h4>
                        <div class="bg-blue-50 rounded-lg p-4 space-y-2">
                            <div class="flex justify-between text-sm">
                                <span class="text-gray-600">Label:</span>
                                <span id="modal_cpc2_label" class="font-bold uppercase"></span>
                            </div>
                            <div class="flex justify-between text-sm">
                                <span class="text-gray-600">Probability:</span>
                                <span id="modal_cpc2_prob" class="font-mono"></span>
                            </div>
                            <div class="flex justify-between text-sm">
                                <span class="text-gray-600">Length:</span>
                                <span id="modal_cpc2_len" class="font-mono"></span>
                            </div>
                        </div>
                    </div>

                    <!-- Sequence -->
                    <div>
                        <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">Sequence (FASTA)</h4>
                        <div class="relative">
                            <pre id="modal_sequence" class="text-xs font-mono bg-gray-50 p-3 rounded-lg border border-gray-100 whitespace-pre-wrap break-all max-h-96 overflow-y-auto text-gray-600"></pre>
                            <button type="button" onclick="copySequence()" class="absolute top-2 right-2 p-1 bg-white border border-gray-200 rounded hover:bg-gray-50 text-gray-500" title="Copy Sequence">
                                📋
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </form>
        <form method="dialog" class="modal-backdrop">
            <button>close</button>
        </form>
    </dialog>

    <script>
        function openImageModal(src, label, sequence, cpc2Data) {
            document.getElementById('modal_image').src = src;
            document.getElementById('modal_label').innerText = label;
            document.getElementById('modal_download').href = src;
            document.getElementById('modal_sequence').innerText = sequence || "No sequence data available.";

            // Handle CPC2 Data
            const cpc2Section = document.getElementById('modal_cpc2_section');
            if (cpc2Data) {
                cpc2Section.classList.remove('hidden');
                document.getElementById('modal_cpc2_label').innerText = cpc2Data.label;
                document.getElementById('modal_cpc2_label').className = "font-bold uppercase " + (cpc2Data.label === "noncoding" ? "text-green-600" : "text-red-600");
                document.getElementById('modal_cpc2_prob').innerText = cpc2Data.prob;
                document.getElementById('modal_cpc2_len').innerText = cpc2Data.len + " nt";
            } else {
                cpc2Section.classList.add('hidden');
            }

            document.getElementById('image_modal').showModal();
        }

        function copySequence() {
            const seq = document.getElementById('modal_sequence').innerText;
            navigator.clipboard.writeText(seq);
        }
    </script>

    <!-- Charts JavaScript -->
    <script>
        // Timing Chart
        {{if .StepTimings}}
        const timingCtx = document.getElementById('timingChart');
        
        const formatDuration = (seconds) => {
            const m = Math.floor(seconds / 60);
            const s = seconds % 60;
            return m > 0 ? m + "m " + s + "s" : s + "s";
        };

        new Chart(timingCtx, {
            type: 'bar',
            data: {
                labels: [{{range .StepTimings}}'{{.StepName}}',{{end}}],
                datasets: [{
                    label: 'Duration',
                    data: [{{range .StepTimings}}{{.DurationSec}},{{end}}],
                    backgroundColor: 'rgba(59, 130, 246, 0.6)',
                    borderColor: 'rgb(59, 130, 246)',
                    borderWidth: 1,
                    borderRadius: 4,
                    hoverBackgroundColor: 'rgba(59, 130, 246, 0.8)'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(17, 24, 39, 0.9)',
                        padding: 12,
                        cornerRadius: 8,
                        callbacks: {
                            label: (context) => "Duration: " + formatDuration(context.raw)
                        }
                    }
                },
                scales: {
                    y: {
                        type: 'logarithmic',
                        title: { display: true, text: 'Seconds (Log Scale)' },
                        grid: { color: 'rgba(0, 0, 0, 0.05)' },
                        border: { display: false }
                    },
                    x: {
                        grid: { display: false },
                        border: { display: false }
                    }
                }
            }
        });
        {{end}}

        // Classification Chart
        {{if .LncRNAFiltering}}
        {{if .LncRNAFiltering.AllClassCodes}}
        const classCodeCtx = document.getElementById('classCodeChart');
        
        const classCodeLabels = [{{range $code, $info := .LncRNAFiltering.AllClassCodes}}'{{$info.Code}}',{{end}}];
        const classCodeData = [{{range $code, $info := .LncRNAFiltering.AllClassCodes}}{{$info.Count}},{{end}}];
        const classCodeColors = [
            '#3b82f6', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981',
            '#06b6d4', '#6366f1', '#f97316', '#14b8a6', '#a855f7',
            '#ef4444', '#84cc16', '#f43f5e'
        ];

        new Chart(classCodeCtx, {
            type: 'doughnut',
            data: {
                labels: classCodeLabels,
                datasets: [{
                    data: classCodeData,
                    backgroundColor: classCodeColors,
                    borderWidth: 2,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            padding: 15,
                            font: {
                                size: 12,
                                family: 'Inter'
                            }
                        }
                    },
                    tooltip: {
                        backgroundColor: 'rgba(17, 24, 39, 0.9)',
                        padding: 12,
                        cornerRadius: 8,
                        callbacks: {
                            label: function(context) {
                                const label = context.label || '';
                                const value = context.parsed || 0;
                                const total = context.dataset.data.reduce((a, b) => a + b, 0);
                                const percentage = ((value / total) * 100).toFixed(1);
                                return label + ': ' + value + ' (' + percentage + '%)';
                            }
                        }
                    }
                }
            }
        });
        {{end}}
        {{end}}
    </script>
</body>
</html>`
}
