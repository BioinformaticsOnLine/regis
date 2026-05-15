# REGIS Module Reference (v1.1.4)

This document provides an in-depth reference for each pipeline module, including the exact commands executed, input/output files, and implementation details.

---

## Table of Contents

1. [Pipeline Architecture](#pipeline-architecture)
2. [Step 0: Dependency Check](#step-0-dependency-check)
3. [Step 1: FastQC](#step-1-fastqc)
4. [Step 2: fastp](#step-2-fastp)
5. [Step 3: SortMeRNA](#step-3-sortmerna)
6. [Step 4: Alignment/Assembly](#step-4-alignmentassembly)
7. [Step 5: CPC2](#step-5-cpc2)
8. [Step 6: CPAT](#step-6-cpat)
9. [Step 7: lncRNA Filtering](#step-7-lncrna-filtering)
10. [Step 8: RNAfold](#step-8-rnafold)
11. [Step 9: LncTar](#step-9-lnctar)
12. [Step 10: IntaRNA](#step-10-intarna)
13. [Step 11: Consensus](#step-11-consensus)
14. [Step 12: Enrichment](#step-12-enrichment)
15. [Step 13: RSeQC](#step-13-rseqc)
16. [Step 14: IGV Report](#step-14-igv-report)
17. [Step 15: MultiQC](#step-15-multiqc)
18. [Step 16: Report Generation](#step-16-report-generation)

---

## Pipeline Architecture

### Execution Flow

```
main.go
    ↓
runPipeline(ctx, cfg, program)
    ↓
┌─────────────────────────────────────────────────────────┐
│ Step 0:  modules.Step00CheckDependencies()              │
│ Step 1:  modules.Step01QCFastQC()                       │
│ Step 2:  modules.Step02TrimFastp()                      │
│ Step 3:  modules.Step03SortMeRNA()        [if enabled]  │
│ Step 4:  modules.Step04AlignAssembly()                  │
│ Step 5:  modules.Step05CPC2()                           │
│ Step 6:  modules.Step06CPAT()                           │
│ Step 7:  modules.Step07FilterLncRNA()                   │
│ Step 8:  modules.Step08RNAfold()                        │
│ Step 9:  modules.Step09LncTar()           [if enabled]  │
│ Step 10: modules.Step10IntaRNA()          [if enabled]  │
│ Step 11: modules.Step11Consensus()                      │
│ Step 12: modules.Step12Enrichment()                     │
│ Step 13: modules.Step13RSeQC()                          │
│ Step 14: modules.Step14IGV()                            │
│ Step 15: modules.Step15MultiQC()                        │
│ Step 16: modules.Step16GenerateReport()                 │
└─────────────────────────────────────────────────────────┘
```

### Module Files Location

All modules are in: `modules/`

| File | Description |
|------|-------------|
| `pipeline.go` | `PipelineRunner` for headless/API execution |
| `slurm.go` | Slurm sbatch script generation |
| `step_00_dependencies.go` | Tool availability checks |
| `step_01_qc_fastqc.go` | FastQC quality control |
| `step_02_trim_fastp.go` | Quality trimming (fastp) |
| `step_03_sortmerna.go` | rRNA filtering |
| `step_04_align_assembly.go` | HISAT2/Trinity |
| `step_05_cpc2.go` | CPC2 coding potential |
| `step_06_cpat.go` | CPAT cross-validation |
| `step_07_filter_lncrna.go` | Classification & expression |
| `step_08_rnafold.go` | Secondary structure |
| `step_09_lnctar.go` | LncTar prediction |
| `step_10_intarna.go` | IntaRNA prediction |
| `step_11_consensus.go` | Cross-tool consensus |
| `step_12_enrichment.go` | Gene list building |
| `step_13_rseqc.go` | RNA-seq QC |
| `step_14_igv.go` | IGV browser report |
| `step_15_multiqc.go` | Aggregate QC |
| `step_16_report.go` | JSON/HTML report |
| `step_16_html.go` | HTML template rendering |

---

## Step 0: Dependency Check

**File:** `step_00_dependencies.go`

### Purpose
Verifies all required external tools are installed and accessible in `$PATH`.

### Tools Checked

#### Core Tools (Always Required)
```go
coreTools := []string{
    "fastqc",
    "fastp",
    "gffcompare",
    "RNAfold",
    "bedtools",
}
```

#### Conditional Tools

| Tool | Condition |
|------|-----------|
| `sortmerna` | `cfg.EnableSortMeRNA == true` |
| `hisat2`, `samtools`, `stringtie`, `gffread` | `cfg.Method == "reference"` |
| `Trinity` or `trinity` | `cfg.Method == "denovo"` |
| `cpat.py` or `cpat` | `cfg.SkipCPAT == false` |
| `cpc2` | Always required |
| `perl` | `cfg.EnableLncTar == true` |
| `IntaRNA` | `cfg.EnableIntaRNA == true` |

### Asset Validation

```go
// Check CPAT model files exist
cfg.CPATHexamerFile = filepath.Join(cfg.CPATModelsDir, cfg.CPATSpecies+"_Hexamer.tsv")
cfg.CPATLogitFile = filepath.Join(cfg.CPATModelsDir, cfg.CPATSpecies+"_logitModel.RData")

// Check LncTar script exists
if _, err := os.Stat(cfg.LncTarScript); os.IsNotExist(err) {
    missingTools = append(missingTools, "LncTar script")
}
```

### Output
- Logs each tool version found
- Returns error if any required tool is missing

---

## Step 1: FastQC

**File:** `step_01_qc_fastqc.go`

### Purpose
Generates quality control reports for raw FASTQ files.

### Commands Executed

```bash
# Paired-end
fastqc -o 01_fastqc -t {threads} {file1} {file2}

# Single-end
fastqc -o 01_fastqc -t {threads} {file1}
```

### Output Directory: `01_fastqc/`

| File | Description |
|------|-------------|
| `*_fastqc.html` | Interactive HTML report |
| `*_fastqc.zip` | Raw data for MultiQC |

---

## Step 2: fastp

**File:** `step_02_trim_fastp.go`

### Purpose
Quality-trims reads, auto-detects/removes adapters, and emits HTML/JSON QC reports.

### Commands Executed

```bash
# Paired-end
fastp -i {file1} -I {file2} \
    -o 02_trimming/paired_1.fastq -O 02_trimming/paired_2.fastq \
    --unpaired1 02_trimming/unpaired_1.fastq --unpaired2 02_trimming/unpaired_2.fastq \
    -w {threads} -l 36 \
    -j 02_trimming/fastp_report.json -h 02_trimming/fastp_report.html

# Single-end
fastp -i {file1} -o 02_trimming/trimmed.fastq \
    -w {threads} -l 36 \
    -j 02_trimming/fastp_report.json -h 02_trimming/fastp_report.html
```

### fastp Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| `-w` | `{threads}` | Worker threads |
| `-l` | `36` | Minimum read length after trimming |
| `-j` / `-h` | reports | JSON and HTML QC reports |

### Output Directory: `02_trimming/`

| File | Description |
|------|-------------|
| `paired_1.fastq` | Trimmed forward reads (paired-end) |
| `paired_2.fastq` | Trimmed reverse reads (paired-end) |
| `unpaired_1.fastq` | Unpaired forward reads |
| `unpaired_2.fastq` | Unpaired reverse reads |
| `trimmed.fastq` | Trimmed reads (single-end) |
| `fastp_report.json` | Trimming statistics (parsed by step 16 report) |
| `fastp_report.html` | Interactive trimming report |

---

## Step 3: SortMeRNA

**File:** `step_03_sortmerna.go`

### Purpose
Filters ribosomal RNA (rRNA) sequences using SILVA database.

### Condition
Only runs if `cfg.EnableSortMeRNA == true`

### SILVA Database Download

If databases don't exist, downloads automatically:
```bash
wget https://github.com/biocore/sortmerna/releases/download/v4.3.4/database.tar.gz
tar -xzf database.tar.gz -C {silva_dir}
```

### Commands Executed

```bash
# Paired-end
sortmerna \
    --ref {silva_db}/smr_v4.3_default_db.fasta \
    --reads {trimmed_R1} --reads {trimmed_R2} \
    --paired_out \
    --aligned 03_sortmerna/rRNA \
    --other 03_sortmerna/non_rRNA \
    --fastx \
    --threads {threads} \
    --workdir 03_sortmerna/workdir

# Post-process: Split interleaved output
seqkit split2 -1 non_rRNA_R1.fastq.gz -2 non_rRNA_R2.fastq.gz
```

### Output Directory: `03_sortmerna/`

| File | Description |
|------|-------------|
| `non_rRNA_R1.fastq.gz` | Non-rRNA forward reads |
| `non_rRNA_R2.fastq.gz` | Non-rRNA reverse reads |
| `rRNA_reads/` | Filtered rRNA reads |
| `sortmerna_log.txt` | Statistics |

---

## Step 4: Alignment/Assembly

**File:** `step_04_align_assembly.go`

### Reference Mode (HISAT2 + StringTie)

#### HISAT2 Index Building
```bash
hisat2-build -p {threads} {reference.fa} 04_alignment/hisat2_index/genome
```

#### HISAT2 Alignment
```bash
# Paired-end
hisat2 -p {threads} \
    -x 04_alignment/hisat2_index/genome \
    -1 {trimmed_R1} -2 {trimmed_R2} \
    -S 04_alignment/aligned.sam \
    --dta

# Single-end
hisat2 -p {threads} \
    -x 04_alignment/hisat2_index/genome \
    -U {trimmed} \
    -S 04_alignment/aligned.sam \
    --dta
```

#### SAM to BAM Conversion
```bash
samtools view -@ {threads} -bS 04_alignment/aligned.sam > 04_alignment/aligned.bam
samtools sort -@ {threads} -o 04_alignment/aligned_sorted.bam 04_alignment/aligned.bam
samtools index 04_alignment/aligned_sorted.bam
rm 04_alignment/aligned.sam 04_alignment/aligned.bam
```

#### StringTie Assembly
```bash
stringtie 04_alignment/aligned_sorted.bam \
    -G {annotation.gtf} \
    -o 05_assembly/stringtie_transcripts.gtf \
    -p {threads}
```

#### Extract Sequences
```bash
gffread 05_assembly/stringtie_transcripts.gtf \
    -g {reference.fa} \
    -w 05_assembly/transcripts.fa
```

### De Novo Mode (Trinity)

```bash
Trinity \
    --seqType fq \
    --left {trimmed_R1} --right {trimmed_R2} \
    --max_memory 50G \
    --CPU {threads} \
    --output 05_assembly/trinity_out

cp 05_assembly/trinity_out/Trinity.fasta 05_assembly/transcripts.fa
```

### Output Directories

| Directory | Contents |
|-----------|----------|
| `04_alignment/` | HISAT2 index, BAM files |
| `05_assembly/` | GTF transcripts, FASTA sequences |

---

## Step 5: CPC2

**File:** `step_05_cpc2.go`

### Purpose
Predicts coding potential using Support Vector Machine (SVM).

### Command Executed

```bash
cpc2 -i 05_assembly/transcripts.fa -o 06_cpc2/cpc2_output
```

### Output Directory: `06_cpc2/`

| File | Description |
|------|-------------|
| `cpc2_output.txt` | Tab-delimited predictions |

### CPC2 Output Format

```
#ID         transcript_length   peptide_length   Fickett_score   pI   ORF_integrity   coding_probability   label
MSTRG.1.1   1245                0                0.0             0    0               0.0245               noncoding
MSTRG.1.2   2340                412              0.95            6.5  1               0.9876               coding
```

---

## Step 6: CPAT

**File:** `step_06_cpat.go`

### Purpose
Cross-validates CPC2 predictions using species-specific logistic regression model.

### Condition
- Skipped if `cfg.SkipCPAT == true`
- Skipped if species is not supported (Human, Mouse, Fly, Zebrafish)

### Command Executed

```bash
cpat.py \
    -g 05_assembly/transcripts.fa \
    -x {models_dir}/{Species}_Hexamer.tsv \
    -d {models_dir}/{Species}_logitModel.RData \
    -o 07_cpat/cpat_output
```

### Bundled Model Files

```
assets/models/
├── Human_Hexamer.tsv
├── Human_logitModel.RData
├── Mouse_Hexamer.tsv
├── Mouse_logitModel.RData
├── Fly_Hexamer.tsv
├── Fly_logitModel.RData
├── Zebrafish_Hexamer.tsv
└── Zebrafish_logitModel.RData
```

### Consensus Calculation

```go
// Read CPC2 noncoding IDs
cpc2NoncodingIDs := readLines("06_cpc2/cpc2_noncoding.txt")

// Read CPAT noncoding IDs (coding_prob < 0.364)
cpatNoncodingIDs := filterCPAT("07_cpat/cpat_output.ORF_prob.best.tsv", 0.364)

// Consensus = intersection
consensusIDs := intersection(cpc2NoncodingIDs, cpatNoncodingIDs)
writeLines("07_cpat/consensus_noncoding.txt", consensusIDs)
```

### Output Directory: `07_cpat/`

| File | Description |
|------|-------------|
| `cpat_output.ORF_prob.best.tsv` | CPAT predictions |
| `cpat_noncoding.txt` | IDs with coding_prob < 0.364 |
| `consensus_noncoding.txt` | CPC2 ∩ CPAT consensus |
| `validation_summary.txt` | Human-readable summary |

---

## Step 7: lncRNA Filtering

**File:** `step_07_filter_lncrna.go`

### Purpose
Filters, classifies, and quantifies lncRNA candidates.

### Sub-steps

#### 7.1 Filter by Consensus
```go
// Use consensus_noncoding.txt (or cpc2_noncoding.txt if CPAT skipped)
// Filter transcripts with:
// - Length > 200 nt
// - Coding probability < 0.5
```

#### 7.2 Extract lncRNA GTF
```bash
# Filter GTF entries matching consensus IDs
grep -F -f consensus_ids.txt transcripts.gtf > lncrna.gtf
```

#### 7.3 Classification with gffcompare
```bash
gffcompare \
    -r {annotation.gtf} \
    08_lncrna_analysis/filtered/lncrna.gtf \
    -o 08_lncrna_analysis/comparison/gffcompare
```

### gffcompare Class Codes

| Code | Description | Type |
|------|-------------|------|
| `=` | Complete match | Known |
| `c` | Contained in reference | Known |
| `u` | Intergenic (unknown/novel) | Novel Intergenic |
| `x` | Antisense | Antisense |
| `i` | Intronic | Intronic |
| `j` | Multi-exon isoform | Other |
| `o` | Overlap | Other |
| `p` | Possible polymerase run-on | Other |

#### 7.4 Expression Quantification
```bash
featureCounts \
    -a 08_lncrna_analysis/filtered/lncrna.gtf \
    -o 08_lncrna_analysis/expression/lncrna_counts.txt \
    -T {threads} \
    04_alignment/aligned_sorted.bam
```

#### 7.5 Best Candidates Selection
```go
// Highly expressed: counts >= 10
// Best candidates: highly_expressed ∩ novel_intergenic
```

### Output Directory: `08_lncrna_analysis/`

```
08_lncrna_analysis/
├── filtered/
│   ├── lncrna_filtered.fa      # Final lncRNA sequences
│   ├── lncrna_filtered.txt     # Statistics table
│   ├── lncrna.gtf              # lncRNA annotations
│   └── lncrna.bed              # BED format
├── novel_lncrnas/
│   ├── novel_intergenic.fa     # Class code 'u'
│   ├── antisense.fa            # Class code 'x'
│   ├── intronic.fa             # Class code 'i'
│   ├── known.fa                # Class code '=' or 'c'
│   └── class_code_counts.txt   # All class code counts
├── expression/
│   ├── lncrna_counts.txt       # featureCounts output
│   ├── lncrna_counts_simple.txt # ID + count only
│   ├── highly_expressed.fa     # Counts >= 10
│   ├── highly_expressed_ids.txt
│   └── best_candidates.txt     # Highly expressed + novel
├── comparison/
│   ├── gffcompare.stats
│   ├── gffcompare.annotated.gtf
│   └── gffcompare.tracking
└── summary_report.txt
```

---

## Step 8: RNAfold

**File:** `step_08_rnafold.go`

### Purpose
Predicts minimum free energy (MFE) secondary structures.

### Command Executed

```bash
cd 09_rnafold
RNAfold < ../08_lncrna_analysis/filtered/lncrna_filtered.fa > lncrna_structures.out
```

### PostScript to PNG Conversion

```bash
# For each .ps file
magick {file}.ps png_files/{file}.png
```

### Output Directory: `09_rnafold/`

| File | Description |
|------|-------------|
| `lncrna_structures.out` | Dot-bracket notation |
| `ps_files/*.ps` | PostScript diagrams |
| `png_files/*.png` | PNG images |

---

## Step 9: LncTar

**File:** `step_09_lnctar.go`

### Purpose
Predicts lncRNA-mRNA interactions based on thermodynamics.

### Condition
Only runs if any of these flags are set:
- `cfg.EnableLncTar`
- `cfg.LncTarBestOnly`
- `cfg.LncTarHighly`
- `cfg.LncTarComprehensive`

### Input Selection

| Flag | Input File |
|------|------------|
| `lnctar_best_only` | `best_candidates.fa` |
| `lnctar_highly` | `highly_expressed.fa` |
| `lnctar_comprehensive` | `lncrna_filtered.fa` |

### Command Executed

```bash
perl {assets}/lnctar/LncTar.pl \
    -l {lncrna_input.fa} \
    -m {mRNA_targets.fa} \
    -d -0.1 \
    -s F \
    -o 10_lnctar/targets.txt
```

### LncTar Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| `-l` | lncRNA file | Query sequences |
| `-m` | mRNA file | Target sequences |
| `-d` | `-0.1` | Delta G threshold |
| `-s` | `F` | Single strand mode |

### Output Directory: `10_lnctar/`

| File | Description |
|------|-------------|
| `best_candidates_targets.txt` | Best only predictions |
| `highly_expressed_targets.txt` | Highly expressed predictions |
| `all_lncrna_targets.txt` | Comprehensive predictions |
| `lnctar_summary.txt` | Statistics |

---

## Step 10: IntaRNA

**File:** `step_10_intarna.go`

### Purpose
Predicts RNA-RNA interactions using accessibility-based algorithm.

### Condition
Only runs if any of these flags are set:
- `cfg.EnableIntaRNA`
- `cfg.IntaRNABestOnly`
- `cfg.IntaRNAHighly`
- `cfg.IntaRNAComprehensive`

### Command Executed

```bash
IntaRNA \
    -q {lncrna_input.fa} \
    -t {mRNA_targets.fa} \
    --outMode=C \
    --outCsvCols=id1,start1,end1,id2,start2,end2,E \
    --threads={threads} \
    > 11_intarna/targets.csv
```

### Output Directory: `11_intarna/`

| File | Description |
|------|-------------|
| `best_candidates_targets.csv` | Best only predictions |
| `highly_expressed_targets.csv` | Highly expressed predictions |
| `all_lncrna_targets.csv` | Comprehensive predictions |
| `intarna_summary.txt` | Statistics |

---

## Step 11: Consensus

**File:** `step_11_consensus.go`

### Purpose
Identifies high-confidence target pairs predicted by BOTH LncTar and IntaRNA.

### Algorithm

```go
// Parse LncTar pairs: (lncRNA_ID, target_ID)
lnctarPairs := parseLncTarOutput("10_lnctar/targets.txt")

// Parse IntaRNA pairs: (lncRNA_ID, target_ID)
intarnaPairs := parseIntaRNAOutput("11_intarna/targets.csv")

// Consensus = intersection
consensusPairs := intersection(lnctarPairs, intarnaPairs)
```

### Output Directory: `10_target_prediction/`

| File | Description |
|------|-------------|
| `consensus_pairs.txt` | High-confidence pairs |
| `consensus_summary.txt` | Statistics and overlap |

---

## Step 12: Enrichment

**File:** `step_12_enrichment.go`

### Purpose
Builds gene lists for pathway enrichment analysis (e.g., getENRICH).

### Gene List Types

1. **Cis-regulatory genes**: Genes within 10kb of lncRNAs
2. **Trans-regulatory genes**: LncTar/IntaRNA target genes
3. **Background genes**: All genes in annotation

### Commands Executed

```bash
# Get genes near lncRNAs (cis-regulation)
bedtools window -a lncrna.bed -b genes.bed -w 10000 \
    | cut -f10 | sort -u > genes_near_lncRNAs.txt

# Map transcript IDs to gene names
# Parse target prediction files and extract gene symbols
```

### Output Directory: `12_enrichment/`

| File | Description |
|------|-------------|
| `background_genes.txt` | All genes in annotation |
| `genes_near_lncRNAs_unique.txt` | Cis-regulatory targets |
| `genes_from_lnctar_mapped.txt` | LncTar trans-targets |
| `genes_from_intarna_mapped.txt` | IntaRNA trans-targets |
| `genes_from_consensus_mapped.txt` | Consensus trans-targets |
| `genes_associated_with_lncRNAs_combined.txt` | All associated genes |
| `enrichment_summary.txt` | Usage guide |

---

## Step 13: RSeQC

**File:** `step_13_rseqc.go`

### Purpose
Generates RNA-seq quality metrics.

### GTF to BED12 Conversion

```bash
gtfToGenePred annotation.gtf annotation.genePred
genePredToBed annotation.genePred annotation.bed12
```

### Commands Executed

```bash
# Read distribution
read_distribution.py \
    -i aligned_sorted.bam \
    -r annotation.bed12 \
    > 13_rseqc/read_distribution.txt

# Gene body coverage
geneBody_coverage.py \
    -i aligned_sorted.bam \
    -r annotation.bed12 \
    -o 13_rseqc/gene_body

# Infer experiment (strand specificity)
infer_experiment.py \
    -i aligned_sorted.bam \
    -r annotation.bed12 \
    > 13_rseqc/infer_experiment.txt
```

### Output Directory: `13_rseqc/`

| File | Description |
|------|-------------|
| `read_distribution.txt` | Exon/intron/intergenic distribution |
| `gene_body_coverage.txt` | 5'→3' coverage profile |
| `infer_experiment.txt` | Strand specificity |

---

## Step 14: IGV Report

**File:** `step_14_igv.go`

### Purpose
Creates interactive HTML genome browser report.

### Command Executed

```bash
create_report lncrna.bed {reference.fa} \
    --output 14_igv_report/lncrna_igv_report.html \
    --tracks aligned_sorted.bam lncrna.gtf \
    --sequence 1
```

### Output Directory: `14_igv_report/`

| File | Description |
|------|-------------|
| `lncrna_igv_report.html` | Interactive IGV browser |

---

## Step 15: MultiQC

**File:** `step_15_multiqc.go`

### Purpose
Aggregates all QC reports into single dashboard.

### Command Executed

```bash
multiqc . \
    -o 15_multiqc \
    -n lncrna_pipeline_report \
    --force
```

### Supported Input Sources

- `01_fastqc/` - FastQC reports
- `02_trimming/` - fastp-trimmed reads and reports
- `04_alignment/` - HISAT2 logs
- `08_lncrna_analysis/expression/` - featureCounts
- `13_rseqc/` - RSeQC reports

### Output Directory: `15_multiqc/`

| File | Description |
|------|-------------|
| `lncrna_pipeline_report.html` | Interactive dashboard |
| `multiqc_data/` | Raw JSON data |

---

## Step 16: Report Generation

**File:** `step_16_report.go` + `step_16_html.go`

### Purpose
Generates comprehensive JSON and HTML pipeline summary.

### Metrics Collected

```go
type PipelineSummary struct {
    RunID            string
    SampleName       string
    StartTime        time.Time
    EndTime          time.Time
    Duration         string
    ValidationMode   string
    Species          string
    
    // Step Metrics
    FastQCMetrics           FastQCMetrics
    TrimmomaticMetrics      TrimmomaticMetrics  // JSON key `trimmomatic`; populated from fastp_report.json
    SortMeRNAMetrics        SortMeRNAMetrics
    HISAT2Metrics           HISAT2Metrics
    StringTieMetrics        StringTieMetrics
    CPC2Metrics             CPC2Metrics
    CPATMetrics             CPATMetrics
    LncRNAFilteringMetrics  LncRNAFilteringMetrics
    RNAfoldMetrics          RNAfoldMetrics
    LncTarMetrics           LncTarMetrics
    IntaRNAMetrics          IntaRNAMetrics
    ConsensusMetrics        ConsensusMetrics
    RSeQCMetrics            RSeQCMetrics
    MultiQCMetrics          MultiQCMetrics
    IGVReportMetrics        IGVReportMetrics
    
    // Counts
    TotalTranscripts  int64
    LncRNACandidates  int64
    NovelLncRNAs      int64
    HighlyExpressed   int64
    BestCandidates    int64
    AssociatedGenes   int64
    
    Errors []string
}
```

### Output Directory: `16_pipeline_report/`

| File | Description |
|------|-------------|
| `summary.json` | Machine-readable metrics |
| `pipeline_report.html` | Visual HTML report |

### HTML Report Sections

1. **Header** - Pipeline name, version, run ID
2. **Run Information** - Start/end time, duration, parameters
3. **Pipeline Steps** - Each step with key metrics
4. **Key Findings** - Summary statistics
5. **Downloads** - Links to key output files

---

*Last Updated: December 2025 | REGIS v1.0.5*
