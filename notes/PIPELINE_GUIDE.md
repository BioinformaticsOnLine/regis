# 📚 REGIS Pipeline - Complete Guide (v1.0.5)

## 🎯 **Executive Summary**

REGIS is a **state-of-the-art bioinformatics pipeline** for identifying and characterizing long non-coding RNAs (lncRNAs) from RNA-seq data. Built entirely in **Go** with a modern Terminal User Interface (TUI), it combines **12+ powerful tools** into a single, automated workflow that takes raw sequencing reads and produces high-confidence lncRNA candidates with comprehensive quality metrics, secondary structures, target predictions, and interactive HTML reports.

**Version:** 1.0.5  
**Language:** Go  
**Distribution:** Conda package (`jitendralab` channel)  
**License:** GPL-3.0  

---

## 🌟 **What's New in v1.0.5?**

### **Go Implementation Benefits**
- ✅ **Modern TUI** - Real-time progress tracking with animated interface
- ✅ **System Monitoring** - Live CPU/Memory usage display
- ✅ **REST API Server** - Programmatic job submission via `/api/v1`
- ✅ **Slurm Integration** - HPC cluster support with job queuing
- ✅ **Persistent Job Queue** - SQLite-based job management
- ✅ **Interactive Mode** - TUI wizard for parameter collection
- ✅ **JSON Configuration** - Support for config files (local_job.json, slurm_job.json)
- ✅ **HTML Report Generation** - Comprehensive visual pipeline summary
- ✅ **Graceful Shutdown** - Ctrl+C with process cleanup
- ✅ **Fixed Asset Paths** - Correct conda environment asset resolution

### **Pipeline Enhancements**
- ✅ **SortMeRNA** - rRNA filtering with SILVA database (optional)
- ✅ **Separate FastQC & fastp** - More granular QC control
- ✅ **17 Pipeline Steps** - From dependency check through HTML report
- ✅ **Cross-tool Consensus** - LncTar + IntaRNA validation
- ✅ **Enrichment Gene Lists** - getENRICH-ready output

---

## 📋 **Table of Contents**

1. [Architecture Overview](#architecture-overview)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Command-Line Options](#command-line-options)
5. [Interactive TUI Mode](#interactive-tui-mode)
6. [Configuration Files](#configuration-files)
7. [Pipeline Steps](#pipeline-steps)
8. [API Server Mode](#api-server-mode)
9. [Slurm Integration](#slurm-integration)
10. [Input Files](#input-files)
11. [Output Structure](#output-structure)
12. [Validation Modes](#validation-modes)
13. [Target Prediction](#target-prediction)
14. [Interpreting Results](#interpreting-results)
15. [Troubleshooting](#troubleshooting)
16. [Performance Tips](#performance-tips)
17. [Citation](#citation)

---

## 🏗️ **Architecture Overview**

### **Project Structure**

```
regis-go/
├── main.go                    # CLI entry point, flag parsing, TUI launch
├── version/
│   └── version.go             # Version constant (1.0.5)
├── config/
│   └── config.go              # Koanf-based configuration with layered loading
├── modules/                   # Pipeline step implementations
│   ├── pipeline.go            # PipelineRunner for headless execution
│   ├── slurm.go               # Slurm job script generation
│   ├── step_00_dependencies.go # Tool availability checks
│   ├── step_01_qc_fastqc.go    # FastQC quality control
│   ├── step_02_trim_fastp.go       # Quality trimming (fastp)
│   ├── step_03_sortmerna.go    # rRNA filtering (optional)
│   ├── step_04_align_assembly.go # HISAT2/Trinity
│   ├── step_05_cpc2.go         # Coding potential (CPC2)
│   ├── step_06_cpat.go         # Cross-validation (CPAT)
│   ├── step_07_filter_lncrna.go # Classification & expression
│   ├── step_08_rnafold.go      # Secondary structure
│   ├── step_09_lnctar.go       # LncTar target prediction
│   ├── step_10_intarna.go      # IntaRNA target prediction
│   ├── step_11_consensus.go    # Cross-tool consensus
│   ├── step_12_enrichment.go   # Gene list building
│   ├── step_13_rseqc.go        # RNA-seq QC metrics
│   ├── step_14_igv.go          # IGV genome browser
│   ├── step_15_multiqc.go      # Aggregate QC report
│   └── step_16_report.go       # HTML pipeline summary
├── api/                       # REST API server
│   ├── server.go              # Fiber-based HTTP server
│   ├── routes.go              # API route definitions
│   ├── handlers/
│   │   ├── job_handlers.go    # Job submission & status
│   │   ├── result_handlers.go # Results & downloads
│   │   ├── queue.go           # Persistent job queue
│   │   └── cleanup_worker.go  # Job retention cleanup
│   └── db/                    # SQLite database layer
├── tui/                       # Terminal User Interface
│   ├── model.go               # Bubble Tea model
│   ├── forms.go               # Interactive parameter forms
│   └── banner.go              # Help display
├── utils/                     # Shared utilities
│   ├── command.go             # Command execution with logging
│   ├── validation.go          # Config validation & asset finding
│   ├── logger.go              # Zap-based logging
│   ├── file.go                # File operations
│   ├── seq.go                 # Sequence utilities
│   └── signal.go              # Signal handling
├── docs/
│   └── docs.go                # Swagger API documentation
├── assets/                    # Bundled resources
│   ├── models/                # CPAT models (Human, Mouse, Fly, Zebrafish)
│   └── lnctar/                # LncTar Perl script & modules
├── local_job.json             # Example local job configuration
├── slurm_job.json             # Example Slurm job configuration
└── meta.yaml                  # Conda package recipe
```

### **Key Technologies**

| Component | Technology | Purpose |
|-----------|------------|---------|
| CLI | `spf13/pflag` | Flag parsing with normalization |
| Config | `koanf` | Layered config (defaults → file → env → flags) |
| TUI | `charmbracelet/bubbletea` | Real-time interactive interface |
| API | `gofiber/fiber` | Fast HTTP server |
| Database | `gorm` + SQLite | Job persistence |
| Queue | `maragu.dev/goqite` | Persistent job queue |
| Logging | `uber-go/zap` | Structured logging |
| Validation | `go-playground/validator` | API input validation |

---

## 🚀 **Installation**

### **Method 1: Conda Install (Recommended)**

```bash
# Install from jitendralab channel
conda install -c jitendralab -c bioconda -c conda-forge regis

# Verify installation
regis --version
# Output: regis version 1.0.5
```

### **Method 2: From Source**

```bash
# Clone repository
git clone https://github.com/BioinformaticsOnLine/regis.git
cd regis

# Build with Go
go build -ldflags="-X github.com/BioinformaticsOnLine/regis/version.Version=1.0.5" -o regis .

# Run
./regis --help
```

### **Verify Installation**

```bash
# Check version
regis --version

# Show help with banner
regis --help

# Launch interactive mode (no arguments)
regis
```

---

## ⚡ **Quick Start**

### **1. Reference-Based Analysis (Paired-End)**

```bash
regis -t paired -m reference \
  -r genome.fasta \
  -g annotation.gtf \
  --f1 reads_R1.fastq.gz \
  --f2 reads_R2.fastq.gz \
  -o output_directory \
  -s Fly
```

### **2. Reference-Based Analysis (Single-End)**

```bash
regis -t single -m reference \
  -r genome.fasta \
  -g annotation.gtf \
  --f1 reads.fastq.gz \
  -o output_directory \
  -s Human
```

### **3. De Novo Assembly (No Reference)**

```bash
regis -t paired -m denovo \
  --f1 reads_R1.fastq.gz \
  --f2 reads_R2.fastq.gz \
  -o output_directory
```

### **4. With rRNA Filtering (Recommended)**

```bash
regis -t paired -m reference \
  -r genome.fasta \
  -g annotation.gtf \
  --f1 reads_R1.fastq.gz \
  --f2 reads_R2.fastq.gz \
  -o output_directory \
  -s Fly \
  --sortmerna
```

### **5. With Target Prediction (Cross-Validation)**

```bash
regis -t paired -m reference \
  -r genome.fasta \
  -g annotation.gtf \
  --f1 reads_R1.fastq.gz \
  --f2 reads_R2.fastq.gz \
  -o output_directory \
  -s Fly \
  --lnctar-best --intarna-best
```

### **6. Using Configuration File**

```bash
regis --config local_job.json
```

---

## 🛠️ **Command-Line Options**

### **Required Options**

| Flag | Values | Description |
|------|--------|-------------|
| `-t, --data-type` | `single` or `paired` | Data type (single-end or paired-end) |
| `-m, --method` | `reference` or `denovo` | Analysis method |
| `-o, --output-dir` | `/path/to/output` | Output directory (will be created) |
| `--f1` | `/path/to/file.fastq` | First FASTQ file (or only file for single-end) |

### **Conditional Options**

| Flag | Required For | Description |
|------|-------------|-------------|
| `--f2` | Paired-end data | Second FASTQ file |
| `-r, --reference` | Reference mode | Reference genome (FASTA format) |
| `-g, --gtf` | Reference mode | Annotation file (GTF or GFF format) |

### **Optional Options**

| Flag | Default | Description |
|------|---------|-------------|
| `-s, --species` | None (CPC2-only) | Species name for CPAT validation |
| `-e, --email` | None | User email (required for API) |
| `-c, --threads` | Auto-detect | Number of CPU cores to use |
| `--config` | None | Path to JSON configuration file |
| `--report` | None | Generate report from existing output directory |

### **Validation Options**

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-cpat` | False | Force CPC2-only mode (skip CPAT) |
| `--cpat-hex` | Bundled | Custom CPAT hexamer model file |
| `--cpat-logit` | Bundled | Custom CPAT logit model file |

### **rRNA Filtering**

| Flag | Default | Description |
|------|---------|-------------|
| `--sortmerna` | False | Enable rRNA filtering with SortMeRNA |

### **LncTar Target Prediction**

| Flag | Default | Description |
|------|---------|-------------|
| `--lnctar` | False | Enable LncTar (highly expressed lncRNAs) |
| `--lnctar-best` | False | Run LncTar on BEST candidates only (fastest) |
| `--lnctar-highly` | False | Run LncTar on highly expressed lncRNAs |
| `--lnctar-all` | False | Run LncTar on all lncRNAs (comprehensive) |

### **IntaRNA Target Prediction**

| Flag | Default | Description |
|------|---------|-------------|
| `--intarna` | False | Enable IntaRNA (highly expressed lncRNAs) |
| `--intarna-best` | False | Run IntaRNA on best candidates only (fastest) |
| `--intarna-highly` | False | Run IntaRNA on highly expressed lncRNAs |
| `--intarna-all` | False | Run IntaRNA on all lncRNAs (comprehensive) |

### **Species Options for CPAT**

Model organisms with built-in CPAT support:
- `Human` - Homo sapiens (GRCh38)
- `Mouse` - Mus musculus (GRCm39)
- `Fly` - Drosophila melanogaster (dm6)
- `Zebrafish` - Danio rerio (GRCz11)

For non-model organisms, CPAT will be skipped automatically.

---

## 🖥️ **Interactive TUI Mode**

When launched without arguments, REGIS enters interactive mode with a TUI wizard:

```bash
regis
# Launches: "Launching interactive mode..."
```

### **TUI Features**

1. **Parameter Forms** - Step-by-step configuration input
2. **File Browser** - Select input files interactively
3. **Progress Tracking** - Real-time step completion status
4. **System Metrics** - Live CPU and memory usage
5. **Log Panel** - Scrollable command output
6. **Keyboard Controls**:
   - `t` - Terminate pipeline
   - `l` - Toggle fullscreen log
   - `h` - Show help
   - `q` - Quit (after completion)
   - `Ctrl+C` - Graceful shutdown with countdown

### **Pipeline Metadata Display**

The TUI shows:
- Start time
- Data type (single/paired)
- Method (reference/denovo)
- CPU cores
- Species
- Validation mode
- LncTar mode
- IntaRNA mode
- SortMeRNA status
- Original command

---

## 📄 **Configuration Files**

### **Local Job Configuration (local_job.json)**

```json
{
    "email": "user@example.com",
    "method": "reference",
    "data_type": "paired",
    "threads": 8,
    "reference": "/path/to/genome.fna",
    "gtf": "/path/to/annotation.gff",
    "file1": "/path/to/reads_R1.fastq",
    "file2": "/path/to/reads_R2.fastq",
    "enable_lnctar": true,
    "lnctar_best_only": true,
    "enable_intarna": false,
    "enable_sortmerna": false,
    "execution_mode": "local"
}
```

### **Slurm Job Configuration (slurm_job.json)**

```json
{
    "email": "user@example.com",
    "data_type": "paired",
    "method": "reference",
    "reference": "/path/to/genome.fna",
    "gtf": "/path/to/annotation.gff",
    "file1": "/path/to/reads_R1.fastq",
    "file2": "/path/to/reads_R2.fastq",
    "enable_lnctar": true,
    "lnctar_best_only": true,
    "enable_intarna": true,
    "intarna_best_only": true,
    "execution_mode": "slurm",
    "slurm": {
        "partition": "compute",
        "nodes": 1,
        "cpus": 40,
        "memory": "120G",
        "extra_script": [
            "export PATH=\"/path/to/tools:$PATH\"",
            "eval \"$(conda shell.bash hook)\"",
            "conda activate regis"
        ]
    }
}
```

### **Configuration Priority**

Configuration is loaded in layers (later overrides earlier):
1. **Defaults** - Built-in default values
2. **Config File** - JSON/YAML configuration file
3. **Environment Variables** - `REGIS_*` prefixed vars
4. **CLI Flags** - Command-line arguments

---

## 🔬 **Pipeline Steps**

REGIS v1.0.5 executes **17 steps** (Step 0-16):

### **Step 0: Dependency Check**

**Module:** `step_00_dependencies.go`

**What it does:**
- Verifies all required external tools are available
- Checks tool versions
- Validates CPAT model files exist
- Confirms LncTar script is accessible

**Tools Checked:**
- Core: `fastqc`, `fastp`, `gffcompare`, `RNAfold`, `bedtools`
- Reference: `hisat2`, `samtools`, `stringtie`, `gffread`
- De Novo: `Trinity`
- Validation: `cpc2`, `cpat.py`
- Optional: `sortmerna`, `IntaRNA`, `perl` (for LncTar)

**Time:** < 1 minute

---

### **Step 1: Quality Control with FastQC**

**Module:** `step_01_qc_fastqc.go`

**Tool:** FastQC ≥0.12.1

**What it does:**
- Analyzes raw read quality
- Generates per-base quality scores
- Detects adapter contamination
- Creates HTML quality report

**Output:**
- `01_fastqc/` - FastQC HTML and ZIP files

**Time:** 2-5 minutes

---

### **Step 2: Quality Trimming with fastp**

**Module:** `step_02_trim_fastp.go`

**Tool:** fastp ≥0.23.4

**What it does:**
- Auto-detects and removes adapter sequences
- Quality trims reads (fastp defaults)
- Drops reads shorter than 36 bp (`-l 36`)
- Writes JSON and HTML QC reports

**Output:**
- `02_trimming/paired_1.fastq` - Trimmed forward reads (paired-end)
- `02_trimming/paired_2.fastq` - Trimmed reverse reads (paired-end)
- `02_trimming/unpaired_1.fastq` / `unpaired_2.fastq` - Unpaired reads (paired-end)
- `02_trimming/trimmed.fastq` - Trimmed reads (single-end)
- `02_trimming/fastp_report.json` / `fastp_report.html` - Trimming statistics

**Time:** 2-8 minutes

---

### **Step 3: rRNA Filtering with SortMeRNA (Optional)**

**Module:** `step_03_sortmerna.go`

**Tool:** SortMeRNA ≥4.3.4

**What it does:**
- Downloads SILVA rRNA databases (if not present)
- Filters out rRNA sequences
- Improves downstream assembly quality

**When it runs:** Only if `--sortmerna` flag is provided

**Output:**
- `03_sortmerna/non_rRNA_R1.fastq.gz` - Non-rRNA forward reads
- `03_sortmerna/non_rRNA_R2.fastq.gz` - Non-rRNA reverse reads
- `03_sortmerna/rRNA_reads/` - Filtered rRNA reads

**Time:** 10-30 minutes (first run downloads ~1GB databases)

---

### **Step 4: Alignment/Assembly**

**Module:** `step_04_align_assembly.go`

#### **Reference Mode (HISAT2 + StringTie)**

**Tools:** HISAT2 ≥2.2.1, StringTie ≥2.1.7, Samtools ≥1.22.1

**What it does:**
- Builds HISAT2 genome index
- Aligns reads to reference genome
- Sorts and indexes alignments
- Assembles transcripts with StringTie
- Extracts sequences with gffread

**Output:**
- `04_alignment/hisat2_index/` - HISAT2 index files
- `04_alignment/aligned_sorted.bam` - Sorted alignments
- `04_alignment/aligned_sorted.bam.bai` - BAM index
- `05_assembly/stringtie_transcripts.gtf` - Assembled transcripts
- `05_assembly/transcripts.fa` - Transcript sequences

#### **De Novo Mode (Trinity)**

**Tool:** Trinity ≥2.15.2

**What it does:**
- Assembles transcripts without reference
- Uses normalized reads for efficiency

**Output:**
- `05_assembly/Trinity.fasta` - Assembled transcripts

**Time:** 15-120 minutes (depending on mode and data size)

---

### **Step 5: Coding Potential with CPC2**

**Module:** `step_05_cpc2.go`

**Tool:** CPC2 (standalone)

**What it does:**
- Predicts coding potential using SVM
- Model-free (works for any organism)
- Features: ORF length, coverage, Fickett score, hexamer bias

**Output:**
- `06_cpc2/cpc2_output.txt` - Predictions for all transcripts

**Time:** 5-15 minutes

---

### **Step 6: Cross-Validation with CPAT**

**Module:** `step_06_cpat.go`

**Tool:** CPAT (Coding Potential Assessment Tool)

**What it does:**
- Predicts coding potential using logistic regression
- Uses species-specific k-mer models
- Calculates ORF size, coverage, Fickett score, hexamer usage
- Threshold: coding_prob < 0.364 = noncoding

**When it runs:** Only for model organisms (Human, Mouse, Fly, Zebrafish)

**Bundled Models:**
- `Human_Hexamer.tsv`, `Human_logitModel.RData`
- `Mouse_Hexamer.tsv`, `Mouse_logitModel.RData`
- `Fly_Hexamer.tsv`, `Fly_logitModel.RData`
- `Zebrafish_Hexamer.tsv`, `Zebrafish_logitModel.RData`

**Output:**
- `07_cpat/cpat_output.ORF_prob.best.tsv` - Detailed predictions
- `07_cpat/cpat_noncoding.txt` - Noncoding IDs
- `07_cpat/consensus_noncoding.txt` - 2-tool consensus

**Time:** 5-10 minutes

---

### **Step 7: lncRNA Filtering & Classification**

**Module:** `step_07_filter_lncrna.go`

**Tools:** gffcompare, seqkit, bedtools, featureCounts

**What it does:**

1. **Consensus Validation** - Finds transcripts predicted as noncoding by BOTH CPC2 and CPAT
2. **Quality Filtering** - Length > 200 nt, coding probability < 0.5
3. **Classification (gffcompare):**
   - `u` - Novel intergenic (between genes)
   - `x` - Antisense (opposite strand)
   - `i` - Intronic (within intron)
   - `=` - Known (matches annotation)
   - And more class codes with counts
4. **Expression Quantification** - featureCounts for read counts
5. **Candidate Selection** - Highly expressed (≥10 reads) + novel

**Output:**
- `08_lncrna_analysis/filtered/lncrna_filtered.fa` - High-confidence lncRNAs
- `08_lncrna_analysis/filtered/lncrna.gtf` - lncRNA annotations
- `08_lncrna_analysis/filtered/lncrna.bed` - BED format
- `08_lncrna_analysis/novel_lncrnas/` - Classified by type
- `08_lncrna_analysis/expression/` - Expression counts
- `08_lncrna_analysis/comparison/` - gffcompare output
- `08_lncrna_analysis/summary_report.txt` - Complete summary

**Time:** 5-10 minutes

---

### **Step 8: Secondary Structure Prediction**

**Module:** `step_08_rnafold.go`

**Tool:** RNAfold (ViennaRNA Package)

**What it does:**
- Predicts minimum free energy (MFE) structures
- Generates PostScript structure diagrams
- Converts to PNG images (ImageMagick)

**Output:**
- `09_rnafold/lncrna_structures.out` - Dot-bracket notation
- `09_rnafold/ps_files/*.ps` - PostScript diagrams
- `09_rnafold/png_files/*.png` - PNG images

**Time:** 2-5 minutes

---

### **Step 9: LncTar Target Prediction (Optional)**

**Module:** `step_09_lnctar.go`

**Tool:** LncTar (bundled Perl script)

**What it does:**
- Predicts lncRNA-mRNA interactions
- Based on thermodynamic stability
- Calculates dG (binding energy) and ndG (normalized)

**Modes:**
- `--lnctar-best` - Best candidates only (~2-5 min)
- `--lnctar` - Highly expressed (~15-20 min)
- `--lnctar-all` - All lncRNAs (~45-70 min)

**Output:**
- `10_lnctar/best_candidates_targets.txt`
- `10_lnctar/highly_expressed_targets.txt`
- `10_lnctar/all_lncrna_targets.txt`
- `10_lnctar/lnctar_summary.txt`

---

### **Step 10: IntaRNA Target Prediction (Optional)**

**Module:** `step_10_intarna.go`

**Tool:** IntaRNA (C++ binary)

**What it does:**
- Accessibility-based interaction prediction
- Multi-threaded (faster than LncTar)
- Complementary algorithm for cross-validation

**Modes:**
- `--intarna-best` - Best candidates only (~2-5 min)
- `--intarna` - Highly expressed (~5-10 min)
- `--intarna-all` - All lncRNAs (~15-25 min)

**Output:**
- `11_intarna/best_candidates_targets.csv`
- `11_intarna/highly_expressed_targets.csv`
- `11_intarna/all_lncrna_targets.csv`
- `11_intarna/intarna_summary.txt`

---

### **Step 11: Cross-Tool Consensus Analysis**

**Module:** `step_11_consensus.go`

**What it does:**
- Compares LncTar and IntaRNA predictions
- Identifies high-confidence pairs (both tools agree)
- Generates consensus summary

**Output:**
- `10_target_prediction/consensus_pairs.txt` - High-confidence pairs
- `10_target_prediction/consensus_summary.txt` - Detailed report

---

### **Step 12: Enrichment Gene List Building**

**Module:** `step_12_enrichment.go`

**Tools:** bedtools

**What it does:**
- Builds gene lists for enrichment analysis
- Extracts cis-regulatory targets (nearby genes)
- Maps trans-regulatory targets (LncTar/IntaRNA)
- Creates combined lists for getENRICH

**Output:**
- `12_enrichment/genes_near_lncRNAs_unique.txt` - Cis-regulation
- `12_enrichment/genes_from_lnctar_mapped.txt` - LncTar trans
- `12_enrichment/genes_from_intarna_mapped.txt` - IntaRNA trans
- `12_enrichment/genes_from_consensus_mapped.txt` - Consensus
- `12_enrichment/genes_associated_with_lncRNAs_combined.txt` - All combined
- `12_enrichment/enrichment_summary.txt` - Summary guide

---

### **Step 13: RNA-seq Quality Assessment**

**Module:** `step_13_rseqc.go`

**Tool:** RSeQC

**What it does:**
- Gene body coverage (5' → 3' bias)
- Read distribution (exon/intron/intergenic)
- Strand specificity inference
- GTF to BED12 conversion

**Output:**
- `13_rseqc/gene_body_coverage.txt`
- `13_rseqc/read_distribution.txt`
- `13_rseqc/infer_experiment.txt`

---

### **Step 14: IGV Genome Browser Report**

**Module:** `step_14_igv.go`

**Tool:** igv-reports (create_report)

**What it does:**
- Creates self-contained HTML IGV report
- Embeds lncRNA locations
- Interactive genome browsing

**Output:**
- `14_igv_report/lncrna_igv_report.html` - Interactive browser

---

### **Step 15: MultiQC Report**

**Module:** `step_15_multiqc.go`

**Tool:** MultiQC

**What it does:**
- Aggregates reports from ALL tools
- Single interactive HTML dashboard
- Supports: FastQC, HISAT2, Samtools, featureCounts, RSeQC

**Output:**
- `15_multiqc/lncrna_pipeline_report.html` - Interactive dashboard
- `15_multiqc/multiqc_data/` - Raw data

---

### **Step 16: Pipeline Summary Report**

**Module:** `step_16_report.go` + `step_16_html.go`

**What it does:**
- Collects metrics from all pipeline steps
- Generates JSON summary
- Creates comprehensive HTML report with:
  - Pipeline run information
  - Step-by-step metrics
  - Interactive visualizations
  - Biological interpretations
  - Key findings summary

**Output:**
- `16_pipeline_report/summary.json` - Machine-readable metrics
- `16_pipeline_report/pipeline_report.html` - Visual HTML report

---

## 🌐 **API Server Mode**

REGIS includes a REST API server for programmatic job submission:

### **Starting the Server**

```bash
regis serve --port 3000 --job-dir ./jobs
```

### **API Endpoints**

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/jobs/submit` | Submit a new job |
| GET | `/api/v1/jobs/{uuid}/status` | Get job status |
| GET | `/api/v1/jobs/{uuid}/results` | Get job results |
| GET | `/api/v1/jobs/{uuid}/results/download` | Download results ZIP |
| GET | `/api/v1/jobs/{uuid}/results/metrics` | Get job metrics |
| GET | `/swagger/*` | Swagger documentation |

### **Submit Job Example**

```bash
curl -X POST http://localhost:3000/api/v1/jobs/submit \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d @local_job.json
```

### **Response**

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "message": "Job submitted successfully"
}
```

### **Swagger Documentation**

Access interactive API docs at: `http://localhost:3000/swagger/index.html`

---

## 🖧 **Slurm Integration**

REGIS supports HPC clusters via Slurm job submission:

### **Slurm Job Submission**

1. Create a `slurm_job.json` configuration
2. Submit via API with `execution_mode: "slurm"`
3. REGIS generates Slurm script and submits with `sbatch`
4. Pipeline runs on compute node
5. Results written to output directory

### **Slurm Configuration Options**

```json
{
  "slurm": {
    "partition": "compute",
    "nodes": 1,
    "cpus": 40,
    "memory": "120G",
    "time": "24:00:00",
    "job_name": "regis_pipeline",
    "email": "user@example.com",
    "extra_args": "--exclusive",
    "extra_script": [
      "module load conda",
      "conda activate regis"
    ]
  }
}
```

### **Internal Command**

For Slurm jobs, REGIS uses an internal subcommand:

```bash
regis submit_internal --config /path/to/job_config.json
```

This runs the pipeline in headless mode with logging.

---

## 📥 **Input Files**

### **1. RNA-seq Reads (Required)**

| Format | Extensions | Compressed | Notes |
|--------|-----------|------------|-------|
| FASTQ | `.fastq`, `.fq` | No | Plain text |
| FASTQ (compressed) | `.fastq.gz`, `.fq.gz` | Yes | **Recommended** |

**Requirements:**
- Minimum 10 MB per file
- Illumina quality scores (Phred+33)
- At least 100,000 reads recommended

### **2. Reference Genome (Reference mode)**

| Format | Extensions | Notes |
|--------|-----------|-------|
| FASTA | `.fasta`, `.fa`, `.fna` | Plain text |
| FASTA (compressed) | `.fasta.gz`, `.fna.gz` | Auto-decompressed |

### **3. Annotation (Reference mode)**

| Format | Extensions | Notes |
|--------|-----------|-------|
| GTF | `.gtf` | Gene Transfer Format (preferred) |
| GFF | `.gff`, `.gff3` | General Feature Format |

---

## 📂 **Output Structure**

```
output_directory/
│
├── 01_fastqc/                        # Step 1: FastQC reports
├── 02_trimming/                      # Step 2: fastp-trimmed reads + reports
├── 03_sortmerna/                     # Step 3: rRNA filtered (optional)
├── 04_alignment/                     # Step 4: HISAT2 alignment
├── 05_assembly/                      # Step 4: StringTie/Trinity assembly
├── 06_cpc2/                          # Step 5: CPC2 predictions
├── 07_cpat/                          # Step 6: CPAT predictions
├── 08_lncrna_analysis/               # Step 7: lncRNA classification
│   ├── filtered/
│   │   ├── lncrna_filtered.fa        # ⭐ High-confidence lncRNAs
│   │   ├── lncrna.gtf
│   │   └── lncrna.bed
│   ├── novel_lncrnas/
│   │   ├── novel_intergenic.fa
│   │   ├── antisense.fa
│   │   └── class_code_counts.txt
│   ├── expression/
│   │   ├── highly_expressed.fa
│   │   └── best_candidates.txt       # ⭐ Top candidates
│   └── summary_report.txt            # ⭐ Analysis summary
├── 09_rnafold/                       # Step 8: Secondary structures
│   ├── lncrna_structures.out
│   └── png_files/*.png
├── 10_lnctar/                        # Step 9: LncTar predictions
├── 11_intarna/                       # Step 10: IntaRNA predictions
├── 10_target_prediction/             # Step 11: Consensus analysis
│   ├── consensus_pairs.txt           # ⭐ High-confidence pairs
│   └── consensus_summary.txt
├── 12_enrichment/                    # Step 12: Gene lists
│   └── genes_associated_with_lncRNAs_combined.txt
├── 13_rseqc/                         # Step 13: RSeQC metrics
├── 14_igv_report/                    # Step 14: IGV browser
│   └── lncrna_igv_report.html        # ⭐ Interactive browser
├── 15_multiqc/                       # Step 15: QC dashboard
│   └── lncrna_pipeline_report.html   # ⭐ MultiQC report
├── 16_pipeline_report/               # Step 16: Summary
│   ├── summary.json                  # Machine-readable
│   └── pipeline_report.html          # ⭐ Visual HTML report
└── pipeline.log                      # ⭐ Complete log

⭐ = Key files to check first
```

---

## 🎯 **Validation Modes**

### **1. Consensus Mode (CPC2 + CPAT)**

**When:** Model organisms (Human, Mouse, Fly, Zebrafish)

**How it works:**
1. CPC2 predicts noncoding → N transcripts
2. CPAT predicts noncoding → M transcripts
3. **Consensus:** Both agree → High-confidence lncRNAs

**Advantages:**
- ✅ High confidence (two independent tools)
- ✅ Reduces false positives
- ✅ Uses species-specific k-mer models

### **2. CPC2-Only Mode**

**When:** Non-model organisms OR `--skip-cpat` flag

**How it works:**
1. CPC2 predicts noncoding → N transcripts
2. **Result:** N lncRNAs (more permissive)

**Advantages:**
- ✅ Universal (works for any organism)
- ✅ No species-specific model needed
- ✅ Finds more candidates

### **Comparison Table**

| Mode | Tools Used | Stringency | Best For |
|------|-----------|------------|----------|
| **Consensus** | CPC2 + CPAT | High | Model organisms, publication |
| **CPC2-Only** | CPC2 | Medium | Non-model, exploration |

---

## 🎯 **Target Prediction**

### **LncTar**

- **Algorithm:** Thermodynamics + empirical rules
- **Language:** Perl (bundled script)
- **Speed:** Single-threaded, slower
- **Best for:** Comprehensive analysis

### **IntaRNA**

- **Algorithm:** Accessibility + seed regions
- **Language:** C++ (multi-threaded)
- **Speed:** 10-20x faster than LncTar
- **Best for:** Quick cross-validation

### **Consensus Analysis**

When both tools are enabled:
- REGIS identifies pairs predicted by BOTH
- Consensus pairs = highest confidence
- Best for experimental validation

---

## 📊 **Interpreting Results**

### **Key Files to Check First**

1. **Pipeline Report:** `16_pipeline_report/pipeline_report.html`
2. **Summary JSON:** `16_pipeline_report/summary.json`
3. **Best Candidates:** `08_lncrna_analysis/expression/best_candidates.txt`
4. **MultiQC Dashboard:** `15_multiqc/lncrna_pipeline_report.html`

### **Quick Checks**

```bash
# How many lncRNAs found?
grep -c ">" output/08_lncrna_analysis/filtered/lncrna_filtered.fa

# Best candidates?
cat output/08_lncrna_analysis/expression/best_candidates.txt

# Alignment rate?
grep "overall alignment rate" output/pipeline.log

# Pipeline duration?
tail -20 output/pipeline.log
```

---

## 🐛 **Troubleshooting**

### **Common Issues**

#### **1. "Missing required tools"**

```bash
# Check if tools are in PATH
which fastqc fastp hisat2 cpc2

# Activate conda environment
conda activate regis
```

#### **2. "CPAT models not found"**

```bash
# Check assets directory
ls $CONDA_PREFIX/share/regis/assets/models/

# Verify species is supported
regis --help | grep -A5 "Species"
```

#### **3. "LncTar script not found"**

```bash
# Check bundled script
ls $CONDA_PREFIX/share/regis/assets/lnctar/LncTar.pl
```

#### **4. Pipeline hangs**

```bash
# Check available memory
free -h

# Reduce threads
regis ... -c 4

# Check disk space
df -h
```

#### **5. Low alignment rate (<50%)**

- Verify reference genome matches organism
- Check read quality in FastQC report
- Consider rRNA filtering with `--sortmerna`

---

## ⚡ **Performance Tips**

1. **Use More CPU Cores**: `-c 16` for faster processing
2. **Compress Input Files**: `.gz` files are 10x smaller
3. **Enable SortMeRNA**: Removes rRNA for cleaner assembly
4. **Use Best Candidates Mode**: `--lnctar-best --intarna-best` for fastest target prediction
5. **Skip CPAT for Non-Model**: Automatic, but saves time

---

## 📚 **Citation**

### **Citing REGIS**

```bibtex
@software{regis2025,
  title = {REGIS: lncRNA Identification Pipeline v1.0.5},
  author = {Narayan, Jitendra and Pruthi, Pranjal},
  year = {2025},
  institution = {CSIR-IGIB},
  funding = {The Rockefeller Foundation},
  url = {https://github.com/BioinformaticsOnLine/regis}
}
```

### **Citing Integrated Tools**

Please also cite the tools used in your analysis:

- **FastQC** - Andrews, S. (2010)
- **fastp** - Chen, S. et al. (2018) Bioinformatics
- **HISAT2** - Kim, D. et al. (2019) Nature Biotechnology
- **StringTie** - Pertea, M. et al. (2015) Nature Biotechnology
- **CPC2** - Kang, Y.J. et al. (2017) Nucleic Acids Research
- **CPAT** - Wang, L. et al. (2013) Nucleic Acids Research
- **LncTar** - Li, J. et al. (2015) Briefings in Bioinformatics
- **IntaRNA** - Mann, M. et al. (2017) Nucleic Acids Research
- **RSeQC** - Wang, L. et al. (2012) Bioinformatics
- **MultiQC** - Ewels, P. et al. (2016) Bioinformatics

---

## 🤝 **Support & Contact**

### **Documentation**
- GitHub: https://github.com/BioinformaticsOnLine/regis
- Issues: https://github.com/BioinformaticsOnLine/regis/issues

### **Team**
- **Dr. Jitendra Narayan** - Principal Investigator (CSIR-IGIB)
- **Dr. Stefano Tiozzo** - CNRS-Sorbonne University
- **Pranjal Pruthi** - Researcher Programmer (CSIR-IGIB)

### **Funding**
This project is funded by **The Rockefeller Foundation** and **CSIR-IGIB**.

---

*For the latest updates and documentation, visit: https://github.com/BioinformaticsOnLine/regis*
