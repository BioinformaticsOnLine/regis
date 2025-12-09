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
	truSeqPEURL = "https://raw.githubusercontent.com/timflutre/trimmomatic/master/adapters/TruSeq3-PE.fa"
	truSeqSEURL = "https://raw.githubusercontent.com/timflutre/trimmomatic/master/adapters/TruSeq3-SE.fa"
)

// Step02TrimTrimmomatic runs Trimmomatic adapter trimming
func Step02TrimTrimmomatic(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(2, "Adapter Trimming with Trimmomatic")

	// Create output directories
	trimDir := filepath.Join(cfg.OutputDir, "02_trimming")
	adapterDir := filepath.Join(trimDir, "adapters")
	if err := utils.CreateDirs(trimDir, adapterDir); err != nil {
		return fmt.Errorf("failed to create trimming directories: %w", err)
	}

	var adapterFile string

	if cfg.DataType == "paired" {
		// Paired-end mode
		adapterFile = filepath.Join(adapterDir, "TruSeq3-PE.fa")

		// Download adapter file if not present
		if !utils.FileExists(adapterFile) {
			utils.ShowProgress("Downloading TruSeq3-PE adapter file")
			if err := downloadFile(truSeqPEURL, adapterFile); err != nil {
				return fmt.Errorf("failed to download adapter file: %w", err)
			}
		}

		// Run Trimmomatic PE
		utils.ShowProgress(fmt.Sprintf("Trimming paired-end reads (using %d threads)", cfg.Threads))

		paired1 := filepath.Join(trimDir, "paired_1.fastq")
		unpaired1 := filepath.Join(trimDir, "unpaired_1.fastq")
		paired2 := filepath.Join(trimDir, "paired_2.fastq")
		unpaired2 := filepath.Join(trimDir, "unpaired_2.fastq")

		args := []string{
			"PE",
			"-threads", strconv.Itoa(cfg.Threads),
			cfg.File1,
			cfg.File2,
			paired1,
			unpaired1,
			paired2,
			unpaired2,
			fmt.Sprintf("ILLUMINACLIP:%s:2:30:10", adapterFile),
			"MINLEN:36",
		}

		if err := utils.RunCommand(ctx, "trimmomatic", args...); err != nil {
			return fmt.Errorf("Trimmomatic PE failed: %w", err)
		}

		// Update config to use trimmed files
		cfg.File1 = paired1
		cfg.File2 = paired2

	} else {
		// Single-end mode
		adapterFile = filepath.Join(adapterDir, "TruSeq3-SE.fa")

		// Download adapter file if not present
		if !utils.FileExists(adapterFile) {
			utils.ShowProgress("Downloading TruSeq3-SE adapter file")
			if err := downloadFile(truSeqSEURL, adapterFile); err != nil {
				return fmt.Errorf("failed to download adapter file: %w", err)
			}
		}

		// Run Trimmomatic SE
		utils.ShowProgress(fmt.Sprintf("Trimming single-end reads (using %d threads)", cfg.Threads))

		trimmed := filepath.Join(trimDir, "trimmed.fastq")

		args := []string{
			"SE",
			"-threads", strconv.Itoa(cfg.Threads),
			cfg.File1,
			trimmed,
			fmt.Sprintf("ILLUMINACLIP:%s:2:30:10", adapterFile),
			"MINLEN:36",
		}

		if err := utils.RunCommand(ctx, "trimmomatic", args...); err != nil {
			return fmt.Errorf("Trimmomatic SE failed: %w", err)
		}

		// Update config to use trimmed file
		cfg.File1 = trimmed
	}

	utils.StepComplete(2, "Adapter Trimming with Trimmomatic", stepStart)
	utils.Info("Trimmed reads saved to", zap.String("dir", trimDir))

	return nil
}

// downloadFile downloads a file from a URL
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
