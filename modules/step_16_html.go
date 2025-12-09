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

// Link represents a hyperlink
type Link struct {
	Label string
	URL   string
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
	Metrics     interface{} // The specific metric struct
	Images      []string    // Paths to images to display (e.g. RNAfold structures)
	ExtraLinks  []Link      // Extra links (e.g. RSeQC PDF)
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
	logoFiles := []string{
		"assets/logo/regis-square-logo.png",
		"assets/logo/jnlab-logo_long_form.png",
	}
	for _, logoFile := range logoFiles {
		if _, err := os.Stat(logoFile); err == nil {
			safeCopy(logoFile, filepath.Join(assetsDir, "logos", filepath.Base(logoFile)))
		}
	}

	// 2. Copy RNAfold PNGs (Top 4)
	pngDir := filepath.Join(cfg.OutputDir, "09_rnafold", "png_files")
	files, _ := os.ReadDir(pngDir)
	count := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".png") {
			src := filepath.Join(pngDir, f.Name())
			dst := filepath.Join(assetsDir, "rnafold", f.Name())
			safeCopy(src, dst)
			count++
			if count >= 4 {
				break
			}
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
		Metrics: s.CPC2,
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
	var rnaImages []string
	pngDir := filepath.Join(cfg.OutputDir, "09_rnafold", "png_files")
	files, _ := os.ReadDir(pngDir)
	count := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".png") {
			rnaImages = append(rnaImages, "assets/rnafold/"+f.Name())
			count++
			if count >= 4 {
				break
			}
		}
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
			"09_rnafold/png_files/*.png",
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

                    <!-- Images (e.g. RNAfold) -->
                    {{if .Images}}
                    <div class="px-6 pb-6">
                        <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Sample Visualizations</h4>
                        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                            {{range .Images}}
                            <div class="border border-gray-100 rounded-lg p-2 bg-gray-50 hover:shadow-md transition-shadow">
                                <img src="{{.}}" alt="Structure" class="w-full h-auto rounded hover:scale-105 transition-transform cursor-pointer">
                            </div>
                            {{end}}
                        </div>
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
