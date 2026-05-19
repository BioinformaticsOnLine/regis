package modules

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step08RNAfold predicts lncRNA secondary structures using RNAfold
func Step08RNAfold(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(8, "lncRNA Secondary Structure Prediction")

	// Create directories
	rnafoldDir := filepath.Join(cfg.OutputDir, "09_rnafold")
	svgDir := filepath.Join(rnafoldDir, "svg_files")

	if err := utils.CreateDirs(rnafoldDir, svgDir); err != nil {
		return fmt.Errorf("failed to create RNAfold directories: %w", err)
	}

	lncrnaDir := filepath.Join(cfg.OutputDir, "08_lncrna_analysis")
	lncrnaFa := filepath.Join(lncrnaDir, "filtered", "lncrna_filtered.fa")

	if !utils.FileExists(lncrnaFa) {
		return fmt.Errorf("lncrna_filtered.fa not found: %s", lncrnaFa)
	}

	// Select top-N sequences by CPC2 score (or all when --rnafold-full)
	limit := cfg.RNAfoldLimit // 100 by default, -1 = unlimited
	inputFa, selected, total, err := selectForRNAfold(lncrnaFa, lncrnaDir, rnafoldDir, limit)
	if err != nil {
		utils.Warn("Could not apply RNAfold limit, using all sequences", zap.Error(err))
		inputFa = lncrnaFa
		selected = total
	}

	if limit > 0 {
		utils.ShowProgress(fmt.Sprintf("Predicting secondary structures (top %d / %d by CPC2 score)", selected, total))
	} else {
		utils.ShowProgress(fmt.Sprintf("Predicting secondary structures (%d sequences, no limit)", total))
	}

	// RNAfold must run from its output directory so intermediate files land there
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(rnafoldDir); err != nil {
		return fmt.Errorf("failed to change to RNAfold directory: %w", err)
	}

	outputFile := filepath.Join(rnafoldDir, "lncrna_structures.out")
	if err := runRNAfold(ctx, inputFa, outputFile); err != nil {
		return fmt.Errorf("RNAfold failed: %w", err)
	}

	utils.ShowProgress("Generating SVG visualizations with RNAplot")
	if err := generateSVGs(ctx, outputFile, svgDir); err != nil {
		utils.Warn("SVG generation failed", zap.Error(err))
	}

	utils.StepComplete(8, "lncRNA Secondary Structure Prediction", stepStart)
	utils.Info("Structure prediction complete",
		zap.String("svg_files", svgDir),
		zap.String("structures", outputFile))

	return nil
}

// selectForRNAfold picks the top-N sequences by final_score from lncrna_scores.tsv.
// Returns the FASTA path to use, how many were selected, and the total available.
// When limit < 0 (--rnafold-full) the original file is returned unchanged.
func selectForRNAfold(lncrnaFa, lncrnaDir, rnafoldDir string, limit int) (faPath string, selected, total int, err error) {
	seqs, err := readFastaIDs(lncrnaFa)
	if err != nil {
		return lncrnaFa, 0, 0, err
	}
	total = len(seqs)

	// No cap requested, or fewer sequences than the limit
	if limit < 0 || total <= limit {
		return lncrnaFa, total, total, nil
	}

	// Rank by final_score from lncrna_scores.tsv
	scoresFile := filepath.Join(lncrnaDir, "intermediate", "lncrna_scores.tsv")
	ranked, rankErr := rankByScore(scoresFile, seqs)
	if rankErr != nil {
		utils.Warn("lncrna_scores.tsv unavailable, taking first N sequences", zap.Int("n", limit))
		ranked = seqs
	}

	topIDs := ranked
	if len(topIDs) > limit {
		topIDs = topIDs[:limit]
	}
	selected = len(topIDs)

	idsFile := filepath.Join(rnafoldDir, "rnafold_selected_ids.txt")
	if err := utils.WriteLines(idsFile, topIDs); err != nil {
		return lncrnaFa, 0, total, fmt.Errorf("failed to write selected IDs: %w", err)
	}

	subsetFa := filepath.Join(rnafoldDir, "rnafold_input.fa")
	if err := utils.ExtractSequences("Step08-Select", idsFile, lncrnaFa, subsetFa); err != nil {
		return lncrnaFa, 0, total, fmt.Errorf("failed to extract top sequences: %w", err)
	}

	return subsetFa, selected, total, nil
}

// readFastaIDs returns all sequence IDs (first token of each header) in order.
func readFastaIDs(faFile string) ([]string, error) {
	f, err := os.Open(faFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, _ := io.ReadAll(f)
	var ids []string
	for _, line := range strings.Split(string(buf), "\n") {
		if strings.HasPrefix(line, ">") {
			hdr := strings.TrimSpace(strings.TrimPrefix(line, ">"))
			if i := strings.IndexAny(hdr, " \t"); i >= 0 {
				hdr = hdr[:i]
			}
			if hdr != "" {
				ids = append(ids, hdr)
			}
		}
	}
	return ids, nil
}

type scoredID struct {
	id    string
	score float64
}

// rankByScore reads lncrna_scores.tsv and returns IDs sorted by final_score descending,
// restricted to those present in the allowList.
func rankByScore(scoresFile string, allowList []string) ([]string, error) {
	allow := make(map[string]bool, len(allowList))
	for _, id := range allowList {
		allow[id] = true
	}

	lines, err := utils.ReadLines(scoresFile)
	if err != nil {
		return nil, err
	}

	var scored []scoredID
	for i, line := range lines {
		if i == 0 {
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		id := fields[0]
		if !allow[id] {
			continue
		}
		score, err := strconv.ParseFloat(fields[5], 64)
		if err != nil {
			continue
		}
		scored = append(scored, scoredID{id, score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	ids := make([]string, len(scored))
	for i, s := range scored {
		ids[i] = s.id
	}
	return ids, nil
}

// runRNAfold runs RNAfold with stdin/stdout redirection
func runRNAfold(ctx context.Context, inputFile, outputFile string) error {
	input, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()

	cmd := exec.CommandContext(ctx, "RNAfold", "--noPS")
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = output

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	utils.Info("Running RNAfold (MFE calculation)")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("RNAfold failed: %w\nStderr: %s", err, stderr.String())
	}

	return nil
}

// generateSVGs uses RNAplot to convert structure output to SVG
func generateSVGs(ctx context.Context, structureFile, svgDir string) error {
	input, err := os.ReadFile(structureFile)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "RNAplot", "--output-format=svg")
	cmd.Stdin = bytes.NewReader(input)

	utils.Info("Running RNAplot (SVG generation)")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("RNAplot failed: %w", err)
	}

	files, err := filepath.Glob("*_ss.svg")
	if err != nil {
		return err
	}

	for _, file := range files {
		destPath := filepath.Join(svgDir, file)
		if err := os.Rename(file, destPath); err != nil {
			utils.Warn("Failed to move SVG file", zap.String("file", file), zap.Error(err))
		}
	}

	utils.Info(fmt.Sprintf("Generated %d SVG structure plots", len(files)))
	return nil
}
