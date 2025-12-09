package modules

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

const (
	silvaURL     = "https://github.com/biocore/sortmerna/releases/download/v4.3.4/database.tar.gz"
	silvaDBName  = "smr_v4.3_default_db.fasta"
	silvaArchive = "database.tar.gz"
)

// Step03SortMeRNA filters ribosomal RNA using SortMeRNA
func Step03SortMeRNA(ctx context.Context, cfg *config.Config) error {
	// Skip if not enabled
	if !cfg.EnableSortMeRNA {
		utils.Info("💡 Tip: Use --sortmerna to remove rRNA contamination (recommended for lncRNA discovery)")
		return nil
	}

	stepStart := time.Now()
	utils.StepHeader(3, "rRNA Filtering with SortMeRNA")

	// Create output directory
	sortmernaDir := filepath.Join(cfg.OutputDir, "03_sortmerna")
	workDir := filepath.Join(sortmernaDir, "work")
	if err := utils.CreateDirs(sortmernaDir, workDir); err != nil {
		return fmt.Errorf("failed to create SortMeRNA directory: %w", err)
	}

	// Set up database directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dbDir := filepath.Join(homeDir, ".sortmerna_db")
	silvaDB := filepath.Join(dbDir, silvaDBName)

	// Download SILVA database if needed (one-time)
	if err := downloadSILVADatabase(dbDir, silvaDB); err != nil {
		return fmt.Errorf("failed to download SILVA database: %w", err)
	}

	// Count original reads for statistics
	utils.ShowProgress("Counting input reads")
	originalReads, err := countFastqReads(cfg.File1)
	if err != nil {
		utils.Warn("Failed to count original reads", zap.Error(err))
		originalReads = 0
	}

	// Run SortMeRNA
	utils.ShowProgress(fmt.Sprintf("Filtering rRNA with SortMeRNA (using %d threads)", cfg.Threads))

	var filteredFile1, filteredFile2 string
	var rrnaFile1, rrnaFile2 string

	if cfg.DataType == "paired" {
		// Paired-end mode
		filteredFile1 = filepath.Join(sortmernaDir, "filtered_fwd.fq")
		filteredFile2 = filepath.Join(sortmernaDir, "filtered_rev.fq")
		rrnaFile1 = filepath.Join(sortmernaDir, "rrna_fwd.fq")
		rrnaFile2 = filepath.Join(sortmernaDir, "rrna_rev.fq")

		if err := runSortMeRNAPaired(ctx, cfg, silvaDB, workDir, filteredFile1, filteredFile2, rrnaFile1, rrnaFile2); err != nil {
			return fmt.Errorf("SortMeRNA failed: %w", err)
		}

		// Update config to use filtered files
		cfg.File1 = filteredFile1
		cfg.File2 = filteredFile2

	} else {
		// Single-end mode
		filteredFile1 = filepath.Join(sortmernaDir, "filtered.fq")
		rrnaFile1 = filepath.Join(sortmernaDir, "rrna.fq")

		if err := runSortMeRNASingle(ctx, cfg, silvaDB, workDir, filteredFile1, rrnaFile1); err != nil {
			return fmt.Errorf("SortMeRNA failed: %w", err)
		}

		// Update config to use filtered file
		cfg.File1 = filteredFile1
	}

	// Calculate and log statistics
	if originalReads > 0 {
		filteredReads, err := countFastqReads(filteredFile1)
		if err == nil {
			rrnaRemoved := originalReads - filteredReads
			rrnaPercent := float64(rrnaRemoved) / float64(originalReads) * 100

			utils.Info(fmt.Sprintf("✓ Original reads: %d pairs", originalReads))
			utils.Info(fmt.Sprintf("✓ rRNA removed: %d reads (%.2f%%)", rrnaRemoved, rrnaPercent))
			utils.Info(fmt.Sprintf("✓ Clean reads: %d pairs (%.2f%%)", filteredReads, 100-rrnaPercent))
		}
	}

	utils.StepComplete(3, "rRNA Filtering with SortMeRNA", stepStart)
	utils.Info("SortMeRNA filtering complete", zap.String("output", sortmernaDir))

	return nil
}

// downloadSILVADatabase downloads the SILVA database if not present
func downloadSILVADatabase(dbDir, silvaDB string) error {
	// Check if database already exists
	if utils.FileExists(silvaDB) {
		utils.Info("✓ SILVA database found (cached)", zap.String("path", dbDir))
		return nil
	}

	utils.ShowProgress("Downloading SILVA 138 + Rfam database (~150 MB, one-time only)")

	// Create database directory
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Download to temporary file
	archivePath := filepath.Join(dbDir, silvaArchive)
	tempArchive := archivePath + ".tmp"

	// Download
	resp, err := http.Get(silvaURL)
	if err != nil {
		return fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading database: %s", resp.Status)
	}

	// Write to temp file
	out, err := os.Create(tempArchive)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tempArchive)
		return fmt.Errorf("failed to write database: %w", err)
	}

	// Rename to final location
	if err := os.Rename(tempArchive, archivePath); err != nil {
		os.Remove(tempArchive)
		return fmt.Errorf("failed to finalize download: %w", err)
	}

	utils.ShowProgress("Extracting SILVA database")

	// Extract using tar
	if err := utils.RunCommand(context.Background(), "tar", "-xzf", archivePath, "-C", dbDir); err != nil {
		return fmt.Errorf("failed to extract database: %w", err)
	}

	// Clean up archive
	os.Remove(archivePath)

	// Verify database file exists
	if !utils.FileExists(silvaDB) {
		return fmt.Errorf("database extraction failed: %s not found", silvaDB)
	}

	utils.Info("✓ SILVA database downloaded and extracted", zap.String("path", dbDir))

	return nil
}

// runSortMeRNAPaired runs SortMeRNA on paired-end data
func runSortMeRNAPaired(ctx context.Context, cfg *config.Config, silvaDB, workDir, filteredFwd, filteredRev, rrnaFwd, rrnaRev string) error {
	// SortMeRNA command for paired-end
	// sortmerna --ref db.fasta --reads R1.fq --reads R2.fq --aligned rrna --other filtered --paired_out --out2 --fastx --threads N --workdir work/
	args := []string{
		"--ref", silvaDB,
		"--reads", cfg.File1,
		"--reads", cfg.File2,
		"--aligned", filepath.Join(filepath.Dir(rrnaFwd), "rrna"),
		"--other", filepath.Join(filepath.Dir(filteredFwd), "filtered"),
		"--paired_out",
		"--out2",
		"--fastx",
		"--threads", strconv.Itoa(cfg.Threads),
		"--workdir", workDir,
		"-v",
	}

	if err := utils.RunCommand(ctx, "sortmerna", args...); err != nil {
		return err
	}

	// SortMeRNA outputs files with specific suffixes, rename them
	// filtered_fwd.fq and filtered_rev.fq
	// rrna_fwd.fq and rrna_rev.fq
	// The actual output names are: filtered_fwd.fq, filtered_rev.fq, rrna_fwd.fq, rrna_rev.fq

	return nil
}

// runSortMeRNASingle runs SortMeRNA on single-end data
func runSortMeRNASingle(ctx context.Context, cfg *config.Config, silvaDB, workDir, filtered, rrna string) error {
	// SortMeRNA command for single-end
	args := []string{
		"--ref", silvaDB,
		"--reads", cfg.File1,
		"--aligned", filepath.Join(filepath.Dir(rrna), "rrna"),
		"--other", filepath.Join(filepath.Dir(filtered), "filtered"),
		"--fastx",
		"--threads", strconv.Itoa(cfg.Threads),
		"--workdir", workDir,
		"-v",
	}

	if err := utils.RunCommand(ctx, "sortmerna", args...); err != nil {
		return err
	}

	return nil
}

// countFastqReads counts the number of reads in a FASTQ file
func countFastqReads(fastqFile string) (int, error) {
	file, err := os.Open(fastqFile)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Count lines and divide by 4 (FASTQ format)
	lineCount := 0
	buf := make([]byte, 32*1024)
	for {
		c, err := file.Read(buf)
		lineCount += countNewlines(buf[:c])

		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}

	return lineCount / 4, nil
}

// countNewlines counts newlines in a byte slice
func countNewlines(data []byte) int {
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count
}
