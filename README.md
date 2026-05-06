# REGIS v1.1.2 - RNA-seq Guided Identification System

![Version](https://img.shields.io/badge/Version-1.1.2-brightgreen?style=for-the-badge)
![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-important?style=for-the-badge)
![License](https://img.shields.io/badge/License-GPL_v3-blue?style=for-the-badge)
![Bioinformatics](https://img.shields.io/badge/Bioinformatics-LncRNA-blueviolet?style=for-the-badge)

**REGIS (RNA-seq Guided Identification System)** is a comprehensive, modular bioinformatics pipeline designed for the high-confidence identification and functional characterization of **Long Non-Coding RNAs (lncRNAs)**. 

Re-engineered in **Go**, REGIS v1.1.2 brings a premium **Terminal User Interface (TUI)**, robust process management, REST API server, Slurm HPC support, and a seamless developer experience, while maintaining rigorous scientific accuracy.


[![Typing SVG](https://readme-typing-svg.demolab.com?font=Fira+Code&size=30&duration=3000&pause=1000&color=00ADD8&center=true&vCenter=true&width=1000&lines=REGIS+%F0%9F%A7%AC+lncRNA+Identification+Pipeline;Built+with+Go+%7C+TUI+%7C+REST+API+%7C+SLURM+Support;NGS+Analysis+%E2%9A%A1+Fast+%E2%9A%A1+Scalable)](https://git.io/typing-svg)

---

## 🚀 Key Features

*   **🖥️ Modern TUI**: Real-time progress tracking, system resource monitoring (CPU/RAM), and beautiful visualizations using `Bubble Tea`.
*   **🧬 Flexible Analysis**: Supports both **De Novo** (Trinity) and **Reference-based** (HISAT2/StringTie) assembly modes.
*   **🛡️ Robust Quality Control**: Integrated FastQC, Trimmomatic, and **SortMeRNA** (rRNA filtering) for clean data.
*   **🎯 High-Confidence Filtering**: 
    *   Multi-step coding potential assessment using **CPC2** and **CPAT**.
    *   Strict length (>200nt) and probability thresholds.
    *   Classification of **Novel Intergenic**, **Antisense**, and **Intronic** lncRNAs against reference annotations.
*   **🔗 Functional Prediction**:
    *   **RNAfold** for secondary structure prediction.
    *   **LncTar** and **IntaRNA** for lncRNA-mRNA interaction discovery.
    *   **Consensus Analysis** to identify high-confidence targets confirmed by multiple tools.
*   **🧪 Enrichment Ready**: Automatically generates gene lists (Background, Associated, Targets) formatted for enrichment analysis (e.g., getENRICH).
*   **📊 Comprehensive Reporting**: Generates interactive **MultiQC** reports, **IGV** HTML genome browser reports, and a final pipeline summary in JSON/Markdown/HTML.

---

## 🛠️ Pipeline Workflow

```mermaid
graph TD
    %% Nodes
    Input([Input FASTQ]) --> QC[01. FastQC]
    QC --> Trim[02. Trimmomatic]
    Trim --> Sort{SortMeRNA?}
    
    Sort -- Yes --> Clean([Cleaned Reads])
    Sort -- No --> Clean
    
    Clean --> Mode{Analysis Mode}
    
    %% Reference Based Branch
    Mode -- Reference --> Align[04. HISAT2 Alignment]
    Align --> Assemble1[StringTie Assembly]
    
    %% De Novo Branch
    Mode -- De Novo --> Assemble2[04. Trinity Assembly]
    
    %% Convergence
    Assemble1 --> Coding[05. CPC2 Coding Potential]
    Assemble2 --> Coding
    
    Coding --> CPAT[06. CPAT Validation]
    CPAT --> Filter[07. LncRNA Filtering]
    
    %% Functional Analysis
    Filter --> Struct[08. RNAfold Structure]
    Filter --> Targets{Target Predict?}
    
    Targets -- Yes --> LncTar[09. LncTar]
    Targets -- Yes --> IntRNA[10. IntaRNA]
    
    LncTar --> Consensus[11. Consensus Targets]
    IntRNA --> Consensus
    
    %% Downstream
    Consensus --> Enrich[12. Enrichment Lists]
    Filter --> RSeQC[13. RSeQC]
    
    %% Reporting
    RSeQC --> IGV[14. IGV Report]
    IGV --> MQC[15. MultiQC]
    MQC --> Final[16. Final Summary]

    %% Styling
    style Input fill:#f9f,stroke:#333,stroke-width:2px
    style Final fill:#9f9,stroke:#333,stroke-width:2px
    style Mode fill:#ff9,stroke:#333,stroke-width:2px
```

---

## 📦 Installation

The easiest way to install REGIS and all its dependencies is via Conda/Mamba.

### Option 1: Quick Install via Conda (Recommended)

REGIS is available on Anaconda Cloud:
🔗 [Anaconda Package Overview](https://anaconda.org/channels/jitendralab/packages/regis/overview)

```bash
conda create -n regis_env -c bioconda -c conda-forge -c jitendralab python=3.10
conda activate regis_env
conda install jitendralab::regis
```

#### Need Conda/Mamba?
If you don't have Conda installed on your system, we highly recommend installing **Miniforge** (which includes `mamba` for faster resolving):

```bash
# Download the installer (Linux x86_64)
wget https://github.com/conda-forge/miniforge/releases/latest/download/Miniforge3-Linux-x86_64.sh

# Run the installer
bash Miniforge3-Linux-x86_64.sh

# Restart your terminal or source your .bashrc
source ~/.bashrc
```

### Option 2: Build from Source (For Developers)

If you prefer to build from source, you will need to manually install dependencies.

1.  **Install Go (v1.21+)**: [Download Go](https://go.dev/dl/)
2.  **External Tools**:
    ```bash
    conda create -n regis -c bioconda -c conda-forge \
        fastqc trimmomatic sortmerna hisat2 trinity stringtie \
        samtools gffcompare seqkit bedtools subread \
        python=3.10 rna-seqc multiqc igv-reports sqlite
    conda activate regis
    ```
    *Note: CPC2, CPAT, LncTar, and IntaRNA may require manual installation or specific bioconda recipes.*

> [!WARNING]
> **Apple Silicon (M1/M2/M3) Users**: The `sortmerna` binary from Bioconda uses AVX2 instructions incompatible with Rosetta 2, causing a crash. Please **omit** the `--sortmerna` flag when running REGIS natively on macOS ARM systems, or run the pipeline inside a Linux Docker container.

### Build from Source
```bash
# Clone the repository
git clone https://github.com/BioinformaticsOnLine/regis.git
cd regis/regis-go

# Build the binary
go build -o regis .

# Verify installation
./regis --help
```

---

## 💻 Usage

REGIS offers two modes of operation: **Interactive TUI** and **CLI Arguments**.

### 1. Interactive Mode 🌟
Simply run `regis` without any arguments to launch the TUI wizard. It will guide you through setting up your analysis.
```bash
./regis
```

### 2. CLI Mode ⚡
For automated workflows or power users, use command-line flags.

#### Basic Reference-Based Analysis
```bash
./regis -t paired \
        -m reference \
        -f1 R1.fastq.gz -f2 R2.fastq.gz \
        -r genome.fa \
        -g annotation.gtf \
        -o results_dir
```

#### Detailed CLI Flags
| Flag | Description | Required? |
|------|-------------|-----------|
| **-t** | Data type: `single` or `paired` | ✅ |
| **-m** | Method: `denovo` or `reference` | ✅ |
| **-f1** | Input file 1 (Forward/Single) | ✅ |
| **-f2** | Input file 2 (Reverse, for paired) | Conditional |
| **-r** | Reference Genome FASTA | Conditional |
| **-g** | Reference Annotation GTF/GFF | Conditional |
| **-o** | Output Directory | ✅ |
| **-c** | CPU Cores (default: auto-detect) | ❌ |
| **--sortmerna** | Enable rRNA filtering (Highly Recommended) | ❌ |
| **--lnctar** | Enable LncTar predictions | ❌ |
| **--intarna** | Enable IntaRNA predictions | ❌ |
| **--skip-cpat** | Skip CPAT validation (use CPC2 only) | ❌ |

---

## 📂 Output Structure

REGIS organizes results into a structured directory tree:

```text
results_dir/
├── 01_fastqc/             # FastQC reports
├── 02_trimmed/            # Cleaned FASTQ files
├── 03_sortmerna/          # rRNA filtered reads (if enabled)
├── 04_alignment/          # HISAT2 BAM files / Trinity Assembly
├── 05_assembly/           # StringTie GTF assemblies
├── 06_cpc2/               # Coding potential results
├── 07_validation/         # CPAT results & Consensus list
├── 08_lncrna_analysis/    # Final LncRNA Characterization
│   ├── filtered/          # Final lncRNA sequences (FASTA/GTF)
│   ├── novel_lncrnas/     # Specifically novel, antisense, intronic transcripts
│   └── expression/        # FeatureCounts expression matrices
├── 11_target_prediction/  # LncTar & IntaRNA results
├── 12_enrichment/         # Gene lists for enrichment (Background vs Targets)
├── 15_multiqc/            # Aggregate QC report
└── 16_pipeline_report/    # FINAL SUMMARY (HTML, JSON, Markdown)
```

---

## 👥 Authors & Acknowledgements

**REGIS Team**:
*   **Dr. Jitendra Narayan** (Principal Investigator) - University of Namur / CSIR-IGIB
*   **Dr. Stefano Tiozzo** (Principal Investigator) - CNRS-Sorbonne University
*   **Pranjal Pruthi** (Lead Developer & Researcher) - CSIR-IGIB

**Funding**:
*   Supported by **The Rockefeller Foundation** and **CSIR-IGIB**.

---

## 📜 Citation & License

### Citing REGIS
If you use REGIS in your research, please cite:
> *REGIS: A Comprehensive RNA-seq Guided Identification System for lncRNA Discovery. [Paper Link]*

## 🗺️ Roadmap

- [x] Core pipeline implementation
- [x] TUI interface
- [x] REST API
- [x] SLURM support
- [ ] Kubernetes support
- [ ] Web UI dashboard
- [ ] Nextflow DSL2 compatibility
- [ ] Cloud execution (AWS Batch, GCP)


### Third-Party Citations
REGIS wraps several academic tools. Please also cite:
*   **LncTar**: *Li, J., et al. (2015). LncTar: a tool for predicting the RNA targets of long noncoding RNAs. Briefings in Bioinformatics.*
    *   *Note: LncTar is included/used under a license restricted to non-commercial genomic research.*


### Quality Control & Preprocessing
- **FastQC**: Andrews, S. (2010). FastQC: A Quality Control Tool for High Throughput Sequence Data. Available online at: http://www.bioinformatics.babraham.ac.uk/projects/fastqc

### Alignment & Assembly
- **HISAT2**: Kim, D., Paggi, J.M., Park, C., Bennett, C., & Salzberg, S.L. (2019). Graph-based genome alignment and genotyping with HISAT2 and HISAT-genotype. *Nature Biotechnology*, 37, 907-915. https://doi.org/10.1038/s41587-019-0201-4

- **StringTie**: Pertea, M., Pertea, G.M., Antonescu, C.M., Chang, T.C., Mendell, J.T., & Salzberg, S.L. (2015). StringTie enables improved reconstruction of a transcriptome from RNA-seq reads. *Nature Biotechnology*, 33, 290-295. https://doi.org/10.1038/nbt.3122

- **Trinity**: Grabherr, M.G., et al. (2011). Full-length transcriptome assembly from RNA-Seq data without a reference genome. *Nature Biotechnology*, 29, 644-652. https://doi.org/10.1038/nbt.1883

### lncRNA Prediction
- **CPC2**: Kang, Y.J., et al. (2017). CPC2: a fast and accurate coding potential assessment tool. *Nucleic Acids Research*, 45(W1), W12-W16. https://doi.org/10.1093/nar/gkx428

- **CPAT**: Wang, L., et al. (2013). CPAT: Coding-Potential Assessment Tool using an alignment-free logistic regression model. *Nucleic Acids Research*, 41(6), e74. https://doi.org/10.1093/nar/gkt006

### Target Prediction
- **IntaRNA**: Mann, M., et al. (2017). IntaRNA 2.0: enhanced and customizable prediction of RNA-RNA interactions. *Nucleic Acids Research*, 45(W1), W435-W439. https://doi.org/10.1093/nar/gkx279

### License
REGIS is licensed under the **GNU General Public License v3.0**. See `LICENSE` for details.
