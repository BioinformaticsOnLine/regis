package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step04CPC2 runs CPC2 coding potential analysis
func Step05CPC2(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()
	utils.StepHeader(5, "Coding Potential with CPC2")

	// CPC2 directory already exists from Step 3
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")

	// Verify input file exists
	if !utils.FileExists(transcriptsFa) {
		return fmt.Errorf("transcripts.fa not found: %s", transcriptsFa)
	}

	// Run CPC2
	cpc2Output := filepath.Join(cpc2Dir, "cpc2_output")
	utils.ShowProgress("Running CPC2")

	if err := utils.RunCommand(ctx, "cpc2",
		"-i", transcriptsFa,
		"-o", cpc2Output,
	); err != nil {
		return fmt.Errorf("CPC2 failed: %w", err)
	}

	// Verify CPC2 output
	cpc2ResultFile := cpc2Output + ".txt"
	if !utils.FileExists(cpc2ResultFile) {
		return fmt.Errorf("CPC2 output file not found: %s", cpc2ResultFile)
	}

	utils.StepComplete(4, "Coding Potential with CPC2", stepStart)
	utils.Info("CPC2 analysis complete", zap.String("output", cpc2ResultFile))

	return nil
}
