package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// PipelineRunner handles the execution of the pipeline
type PipelineRunner struct {
	Config *config.Config
}

// NewPipelineRunner creates a new pipeline runner
func NewPipelineRunner(cfg *config.Config) *PipelineRunner {
	return &PipelineRunner{
		Config: cfg,
	}
}

// RunHeadless executes the pipeline without TUI updates
// It uses the utils.Logger for output (file + stdout)
func (p *PipelineRunner) RunHeadless(ctx context.Context) error {
	cfg := p.Config
	utils.Info("Starting Headless Pipeline Execution")

	// Step 0: Dependency Check
	if err := runStep(ctx, 0, "Checking Dependencies", func() error {
		return Step00CheckDependencies(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 1: FastQC
	if err := runStep(ctx, 1, "Quality Control with FastQC", func() error {
		return Step01QCFastQC(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 2: Trimmomatic
	if err := runStep(ctx, 2, "Adapter Trimming with Trimmomatic", func() error {
		return Step02TrimTrimmomatic(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 3: SortMeRNA (optional)
	if cfg.EnableSortMeRNA {
		if err := runStep(ctx, 3, "rRNA Filtering with SortMeRNA", func() error {
			return Step03SortMeRNA(ctx, cfg)
		}); err != nil {
			return err
		}
	}

	// Step 4: Alignment/Assembly
	stepName := "De Novo Assembly with Trinity"
	if cfg.Method == "reference" {
		stepName = "Reference-based Alignment with HISAT2"
	} else if cfg.Assembler == "rnabloom" {
		stepName = "De Novo Assembly with RNA-Bloom"
	}
	if err := runStep(ctx, 4, stepName, func() error {
		return Step04AlignAssembly(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 5: CPC2
	if err := runStep(ctx, 5, "Coding Potential with CPC2", func() error {
		return Step05CPC2(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 6: CPAT + Consensus
	if err := runStep(ctx, 6, "Cross-Validation with CPAT", func() error {
		return Step06CPAT(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 7: lncRNA Filtering
	if err := runStep(ctx, 7, "Processing GTF and Filtering lncRNAs", func() error {
		return Step07FilterLncRNA(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 8: RNAfold
	if err := runStep(ctx, 8, "lncRNA Secondary Structure Prediction", func() error {
		return Step08RNAfold(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 9: LncTar (optional)
	if cfg.EnableLncTar {
		if err := runStep(ctx, 9, "Predicting lncRNA-mRNA Interactions with LncTar", func() error {
			return Step09LncTar(ctx, cfg)
		}); err != nil {
			return err
		}
	}

	// Step 10: IntaRNA (optional)
	if cfg.EnableIntaRNA {
		if err := runStep(ctx, 10, "Cross-Validating Targets with IntaRNA", func() error {
			return Step10IntaRNA(ctx, cfg)
		}); err != nil {
			return err
		}
	}

	// Step 11: Consensus
	if err := runStep(ctx, 11, "Cross-Tool Consensus Analysis", func() error {
		return Step11Consensus(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 12: Enrichment
	if err := runStep(ctx, 12, "Building Enrichment Gene Lists", func() error {
		return Step12Enrichment(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 13: RSeQC
	if err := runStep(ctx, 13, "RNA-seq Quality Assessment with RSeQC", func() error {
		return Step13RSeQC(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 14: IGV Report
	if err := runStep(ctx, 14, "Creating IGV Genome Browser Report", func() error {
		return Step14IGV(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 15: MultiQC
	if err := runStep(ctx, 15, "Generating MultiQC Report", func() error {
		return Step15MultiQC(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 16: Generate Summary Report
	if err := runStep(ctx, 16, "Generating Pipeline Summary Report", func() error {
		return Step16GenerateReport(ctx, cfg)
	}); err != nil {
		return err
	}

	utils.Info("Pipeline Completed Successfully")
	return nil
}

// runStep is a helper to log step start/completion
func runStep(ctx context.Context, stepNum int, name string, fn func() error) error {
	start := time.Now()
	// Logic matches utils.StepHeader but ensures we log even if utils doesn't
	// utils.Info(fmt.Sprintf("Starting Step %d: %s", stepNum, name))

	if err := fn(); err != nil {
		utils.Error(fmt.Sprintf("Step %d failed: %v", stepNum, err))
		return fmt.Errorf("step %d (%s) failed: %w", stepNum, name, err)
	}

	duration := time.Since(start)
	utils.Info(fmt.Sprintf("Step %d completed", stepNum), zap.Duration("duration", duration))
	return nil
}
