package modules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
)

// stepSentinel returns the key output file that signals a step is complete.
// An empty string means "no sentinel defined — never skip".
func stepSentinel(cfg *config.Config, step int) string {
	out := cfg.OutputDir
	switch step {
	case 1: // FastQC
		return filepath.Join(out, "01_fastqc")
	case 2: // fastp
		if cfg.DataType == "paired" {
			return filepath.Join(out, "02_trimming", "paired_1.fastq")
		}
		return filepath.Join(out, "02_trimming", "trimmed.fastq")
	case 3: // SortMeRNA (optional)
		return filepath.Join(out, "03_sortmerna")
	case 4: // Assembly / Alignment — all paths copy to 06_cpc2/transcripts.fa
		return filepath.Join(out, "06_cpc2", "transcripts.fa")
	case 5: // CPC2
		return filepath.Join(out, "06_cpc2", "cpc2_output.txt")
	case 6: // CPAT / consensus
		return filepath.Join(out, "07_validation", "consensus_noncoding.txt")
	case 7: // lncRNA filter
		return filepath.Join(out, "08_lncrna_analysis", "filtered", "lncrna_filtered.fa")
	case 8: // RNAfold
		return filepath.Join(out, "09_rnafold", "lncrna_structures.out")
	case 9: // LncTar
		return filepath.Join(out, "11_target_prediction", "lnctar")
	case 10: // IntaRNA
		return filepath.Join(out, "11_target_prediction", "intarna")
	case 11: // Consensus targets
		return filepath.Join(out, "11_target_prediction", "consensus_pairs.txt")
	case 12: // Enrichment
		return filepath.Join(out, "12_enrichment")
	case 13: // RSeQC
		return filepath.Join(out, "13_rseqc")
	case 14: // IGV
		return filepath.Join(out, "14_igv_report")
	case 15: // MultiQC
		return filepath.Join(out, "15_multiqc", "lncrna_pipeline_report.html")
	case 16: // Summary report
		return filepath.Join(out, "16_pipeline_report", "pipeline_summary.html")
	}
	return ""
}

// sentinelExists returns true if the sentinel path exists and, for regular files,
// is non-empty (size > 0). Directories only need to exist.
func sentinelExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return true
	}
	return info.Size() > 0
}

// ShouldSkipStep returns true when the step should be skipped because:
//  a) --from-step N is set and step < N, OR
//  b) --resume is set and the step's sentinel output already exists.
//
// Step 0 (dependency check) is never skipped.
func ShouldSkipStep(cfg *config.Config, step int) bool {
	if step == 0 {
		return false
	}

	// Hard skip: --from-step N skips everything before N
	if cfg.FromStep > 0 && step < cfg.FromStep {
		utils.Info(fmt.Sprintf("⏭  Step %d skipped (--from-step %d)", step, cfg.FromStep))
		return true
	}

	// Soft skip: --resume + sentinel exists
	if cfg.Resume {
		sentinel := stepSentinel(cfg, step)
		if sentinelExists(sentinel) {
			utils.Info(fmt.Sprintf("⏭  Step %d skipped (output exists, --resume): %s", step, sentinel))
			return true
		}
	}

	return false
}
