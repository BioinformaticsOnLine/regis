package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BioinformaticsOnLine/regis/api"
	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/modules"
	"github.com/BioinformaticsOnLine/regis/tui"
	"golang.org/x/term"
	"github.com/BioinformaticsOnLine/regis/utils"
	"github.com/BioinformaticsOnLine/regis/version"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func main() {
	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			serveCmd := pflag.NewFlagSet("serve", pflag.ExitOnError)
			port := serveCmd.StringP("port", "p", "3000", "Port to run the server on")
			jobDir := serveCmd.String("job-dir", "./jobs", "Directory to store job outputs")
			serveCmd.Parse(os.Args[2:])
			api.StartServer(*port, *jobDir)
			return

		case "submit_internal":
			// Internal command used by Slurm jobs to execute the pipeline
			submitCmd := pflag.NewFlagSet("submit_internal", pflag.ExitOnError)
			configPath := submitCmd.String("config", "", "Path to job configuration JSON")
			submitCmd.Parse(os.Args[2:])

			if *configPath == "" {
				fmt.Println("Error: --config is required for submit_internal")
				os.Exit(1)
			}

			runInternalJob(*configPath)
			return
		}
	}

	// Define Root CLI flags
	// We use pflag here which allows binding to koanf in config.Load
	pflag.StringP("data_type", "t", "", "Data type: 'single' or 'paired'")
	pflag.StringP("method", "m", "", "Analysis method: 'denovo' or 'reference'")
	pflag.String("f1", "", "Input file 1 (or single-end file)")
	pflag.String("f2", "", "Input file 2 (for paired-end)")
	pflag.StringP("reference", "r", "", "Reference genome FASTA file")
	pflag.StringP("gtf", "g", "", "Annotation GTF/GFF file")
	pflag.StringP("output_dir", "o", "", "Output directory")

	// Optional strings
	pflag.StringP("species", "s", "", "Species name for CPAT (Human, Mouse, Fly, Zebrafish)")
	pflag.StringP("email", "e", "", "User email (optional)")
	pflag.IntP("threads", "c", 0, "Number of CPU cores (default: all available)")
	pflag.IntP("min_length", "l", 200, "Minimum sequence length for lncRNA filtering")
	pflag.Float64("length_penalty", 0.5, "Penalty factor (0-1) applied to transcripts shorter than --min_length")
	pflag.Float64("score_threshold", 0.5, "Minimum confidence score to retain a transcript (after length penalty)")

	// CPAT flags
	pflag.Bool("skip_cpat", false, "Force CPC2-only mode (skip CPAT)")
	pflag.String("cpat_hex", "", "Custom CPAT hexamer model file")
	pflag.String("cpat_logit", "", "Custom CPAT logit model file")

	// LncTar flags
	pflag.Bool("lnctar", false, "Enable LncTar target prediction")
	pflag.Bool("lnctar_best", false, "Run LncTar on best candidates only")
	pflag.Bool("lnctar_highly", false, "Run LncTar on highly expressed lncRNAs")
	pflag.Bool("lnctar_all", false, "Run LncTar on all lncRNAs (comprehensive)")
	pflag.Bool("lnctar_comprehensive", false, "Alias for --lnctar-all")

	// IntaRNA flags
	pflag.Bool("intarna", false, "Enable IntaRNA target prediction")
	pflag.Bool("intarna_best", false, "Run IntaRNA on best candidates only")
	pflag.Bool("intarna_highly", false, "Run IntaRNA on highly expressed lncRNAs")
	pflag.Bool("intarna_all", false, "Run IntaRNA on all lncRNAs (comprehensive)")
	pflag.Bool("intarna_comprehensive", false, "Alias for --intarna-all")

	// rRNA filtering
	pflag.Bool("sortmerna", false, "Enable rRNA filtering (recommended)")

	// RNAfold options
	pflag.Int("rnafold_limit", 0, "Max sequences for RNAfold/SVG (default 100; 0 = use default)")
	pflag.Bool("rnafold_full", false, "Run RNAfold on all filtered lncRNAs (no limit)")

	// Resume / checkpoint options
	pflag.Bool("resume", false, "Skip steps whose output already exists (safe re-run after interruption)")
	pflag.Int("from_step", 0, "Hard-skip all steps before N and start from step N (e.g. --from-step 5)")

	// De novo assembler
	pflag.String("assembler", "rnabloom", "De novo assembler: 'trinity' or 'rnabloom'")

	// Strandedness
	pflag.String("stranded", "unstranded", "Library strandedness: 'unstranded', 'rf', 'fr', 'f', 'r'")

	// Config file
	configFile := pflag.String("config", "", "Path to configuration file")

	// Report only
	reportOnly := pflag.String("report", "", "Generate summary report from existing output directory")

	// Help & Version
	help := pflag.BoolP("help", "h", false, "Show help message")
	versionFlag := pflag.BoolP("version", "v", false, "Show version information")

	// Normalize flags: allow both underscores and hyphens (e.g. --lnctar-best works for --lnctar_best)
	pflag.CommandLine.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		from := []string{"-", "_"}
		to := "_"
		for _, sep := range from {
			name = strings.Replace(name, sep, to, -1)
		}
		return pflag.NormalizedName(name)
	})

	// Parse flags
	pflag.Parse()

	// Handle immediate actions
	if *help {
		tui.ShowBanner()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("regis version %s\n", version.Version)
		os.Exit(0)
	}

	if *reportOnly != "" {
		if err := generateReportOnly(*reportOnly); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Report generated successfully in %s/16_pipeline_report/\n", *reportOnly)
		os.Exit(0)
	}

	// LOAD CONFIGURATION (Koanf)
	// This merges Defaults + Config File + Env + Flags
	cfg, err := config.Load(pflag.CommandLine, *configFile)
	if err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		os.Exit(1)
	}

	// DEBUG: Check if flags are loaded
	// fmt.Printf("DEBUG CONFIG: Method='%s' DataType='%s' File1='%s' File2='%s' OutputDir='%s'\n",
	//	cfg.Method, cfg.DataType, cfg.File1, cfg.File2, cfg.OutputDir)

	// Interactive Mode Trigger
	// If essential fields are missing, and no specific action requested, launch TUI wizard
	if cfg.DataType == "" && cfg.Method == "" && cfg.File1 == "" && cfg.OutputDir == "" {
		fmt.Println("Launching interactive mode...")
		// Note: TUI CollectParameters returns a *Config.
		// We might want to merge it? Or just use it.
		// For now, TUI generates a fresh config.
		interactiveCfg, err := tui.CollectParameters()
		if err != nil {
			fmt.Printf("Error collecting parameters: %v\n", err)
			os.Exit(1)
		}
		cfg = interactiveCfg
	} else {
		// CLI Mode Validation
		if cfg.DataType == "" || cfg.Method == "" || cfg.OutputDir == "" || cfg.File1 == "" {
			// Missing required flags
			tui.ShowBanner()
			fmt.Println("\n❌ Missing required arguments via CLI/Config/Env.")
			if cfg.DataType == "" {
				fmt.Println("   • Missing: -t / --data_type")
			}
			if cfg.Method == "" {
				fmt.Println("   • Missing: -m / --method")
			}
			if cfg.File1 == "" {
				fmt.Println("   • Missing: --f1")
			}
			if cfg.OutputDir == "" {
				fmt.Println("   • Missing: -o / --output_dir")
			}
			os.Exit(1)
		}

		// Map 'Logic' fields to 'Enable' fields (compatibility with flags)
		if cfg.LncTarBestOnly || cfg.LncTarComprehensive || cfg.LncTarHighly {
			cfg.EnableLncTar = true
		}
		if cfg.IntaRNABestOnly || cfg.IntaRNAComprehensive || cfg.IntaRNAHighly {
			cfg.EnableIntaRNA = true
		}

		// --rnafold-full overrides --rnafold-limit: -1 signals "no cap" to Step08
		if full, _ := pflag.CommandLine.GetBool("rnafold_full"); full {
			cfg.RNAfoldLimit = -1
		}
	}

	// Enable TUI mode to suppress stdout printing
	utils.SetTUIMode(true)

	// Initialize logger
	logFile := filepath.Join(cfg.OutputDir, "pipeline.log")
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	if err := utils.InitLogger(logFile); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.Sync()

	// Validate configuration (Business Logic)
	if err := utils.ValidateConfig(cfg); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// ── TTY Detection ────────────────────────────────────────────────────────────
	// If stdout is not a terminal (Slurm, nohup, CI, piped output) skip the
	// Bubble Tea dashboard and run headlessly, logging to pipeline.log.
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println("[regis] No TTY detected — running in headless mode (see pipeline.log)")
		utils.SetTUIMode(false)

		ctx, cancel := utils.SetupSignalHandler()
		defer cancel()

		runner := modules.NewPipelineRunner(cfg)
		if err := runner.RunHeadless(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Pipeline failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[regis] Pipeline completed successfully.")
		os.Exit(0)
	}

	// ── Interactive TUI Dashboard ─────────────────────────────────────────────
	// Launch TUI
	msgChan := make(chan tea.Msg, 100)
	model := tui.NewModel(msgChan)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	utils.SetTUIProgram(program)

	// Run pipeline in goroutine
	go func() {
		ctx, cancel := utils.SetupSignalHandler()
		defer cancel()

		pipelineStart := time.Now()

		program.Send(tui.PipelineMetadataMsg{
			StartTime:        time.Now(),
			DataType:         cfg.DataType,
			Method:           cfg.Method,
			Cores:            cfg.Threads,
			Species:          cfg.Species,
			ValidationMode:   cfg.ValidationMode,
			LncTarMode:       getLncTarMode(cfg),
			IntaRNAMode:      getIntaRNAMode(cfg),
			SortMeRNAEnabled: cfg.EnableSortMeRNA,
			OriginalCommand:  strings.Join(os.Args, " "),
		})

		err := runPipeline(ctx, cfg, program)

		if ctx.Err() == context.Canceled {
			program.Send(tui.PipelineCompleteMsg{
				Success:  false,
				Duration: time.Since(pipelineStart),
			})
			program.Send(tui.LogEntryMsg{
				Timestamp: time.Now(),
				Level:     "warn",
				Message:   "⚠ Pipeline interrupted by user",
			})
			return
		}

		duration := time.Since(pipelineStart)
		failureReason := ""
		if err != nil {
			failureReason = err.Error()
		}

		program.Send(tui.PipelineCompleteMsg{
			Success:       err == nil,
			Duration:      duration,
			FailureReason: failureReason,
		})

		if err != nil {
			program.Send(tui.LogEntryMsg{
				Timestamp: time.Now(),
				Level:     "error",
				Message:   fmt.Sprintf("Pipeline failed: %v", err),
			})
		} else {
			tui.SendLog(program, "info", "Pipeline completed successfully", "main")
		}
	}()

	finalModel, err := program.Run()

	if err != nil && !strings.Contains(err.Error(), "interrupted") {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(tui.Model); ok {
		if m.PipelineSuccess {
			fmt.Println("\n\033[32m✓ Pipeline execution completed successfully.\033[0m")
			os.Exit(0)
		}

		cancelled := tui.ShowGracefulShutdown()

		if !m.PipelineSuccess && m.FailureReason != "" {
			fmt.Println("\n\033[1;31m❌ PIPELINE FAILURE\033[0m")
			parts := strings.SplitN(m.FailureReason, ": ", 2)
			if len(parts) > 1 {
				fmt.Printf("\033[31m   ├─ Context: %s\n", parts[0])
				fmt.Printf("   ╰─ Reason:  %s\033[0m\n", parts[1])
			} else {
				fmt.Printf("\033[31m   ╰─ Reason: %s\033[0m\n", m.FailureReason)
			}
		}

		if cancelled {
			fmt.Println("\033[32m✓ Shutdown cancelled! Pipeline will continue running in background.\033[0m")
			fmt.Println("\033[90mPress Ctrl+C when you're ready to exit.\033[0m")
			fmt.Println()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			<-sigChan

			fmt.Println()
			fmt.Println("\033[33mTerminating pipeline processes...\033[0m")
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(os.Interrupt)
			time.Sleep(1 * time.Second)

			fmt.Println("\033[36m(｡･ω･)ﾉﾞ Goodbye! Process terminated.\033[0m")
		} else {
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(os.Interrupt)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func getLncTarMode(cfg *config.Config) string {
	if !cfg.EnableLncTar {
		return "off"
	}
	if cfg.LncTarBestOnly {
		return "best candidates"
	}
	if cfg.LncTarComprehensive {
		return "all lncRNAs"
	}
	return "highly expressed"
}

func getIntaRNAMode(cfg *config.Config) string {
	if !cfg.EnableIntaRNA {
		return "off"
	}
	if cfg.IntaRNABestOnly {
		return "best candidates"
	}
	if cfg.IntaRNAComprehensive {
		return "all lncRNAs"
	}
	return "highly expressed"
}

// tuiStep wraps a single pipeline step for the TUI path.
// It checks checkpoints (--resume / --from-step) before running, and marks
// the step complete in the TUI regardless of whether it was skipped or ran.
func tuiStep(ctx context.Context, cfg *config.Config, program *tea.Program, stepNum int, name, tool string, fn func() error) error {
	if modules.ShouldSkipStep(cfg, stepNum) {
		tui.SendStepComplete(program, stepNum, true, 0)
		return nil
	}
	tui.SendStepStart(program, stepNum, name, tool)
	if err := fn(); err != nil {
		tui.SendStepComplete(program, stepNum, false, 0)
		return fmt.Errorf("Step %d failed: %w", stepNum, err)
	}
	tui.SendStepComplete(program, stepNum, true, 0)
	return nil
}

func runPipeline(ctx context.Context, cfg *config.Config, program *tea.Program) error {
	// Step 0: Dependency Check (never skipped)
	tui.SendStepStart(program, 0, "Checking Dependencies", "dependency_check")
	if err := modules.Step00CheckDependencies(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 0, false, 0)
		return fmt.Errorf("dependency check failed: %w", err)
	}
	tui.SendStepComplete(program, 0, true, 0)

	// Step 1: FastQC
	if err := tuiStep(ctx, cfg, program, 1, "Quality Control with FastQC", "fastqc", func() error {
		return modules.Step01QCFastQC(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 2: fastp
	if err := tuiStep(ctx, cfg, program, 2, "Quality Trimming with fastp", "fastp", func() error {
		return modules.Step02TrimFastp(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 3: SortMeRNA (optional)
	if cfg.EnableSortMeRNA {
		if err := tuiStep(ctx, cfg, program, 3, "rRNA Filtering with SortMeRNA", "sortmerna", func() error {
			return modules.Step03SortMeRNA(ctx, cfg)
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
	if err := tuiStep(ctx, cfg, program, 4, stepName, "hisat2/trinity/rnabloom", func() error {
		return modules.Step04AlignAssembly(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 5: CPC2
	if err := tuiStep(ctx, cfg, program, 5, "Coding Potential with CPC2", "cpc2", func() error {
		return modules.Step05CPC2(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 6: CPAT + Consensus
	if err := tuiStep(ctx, cfg, program, 6, "Cross-Validation with CPAT", "cpat", func() error {
		return modules.Step06CPAT(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 7: lncRNA Filtering
	if err := tuiStep(ctx, cfg, program, 7, "Processing GTF and Filtering lncRNAs", "seqkit/gffcompare", func() error {
		return modules.Step07FilterLncRNA(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 8: RNAfold
	if err := tuiStep(ctx, cfg, program, 8, "lncRNA Secondary Structure Prediction", "RNAfold", func() error {
		return modules.Step08RNAfold(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 9: LncTar (optional)
	if cfg.EnableLncTar {
		if err := tuiStep(ctx, cfg, program, 9, "Predicting lncRNA-mRNA Interactions with LncTar", "LncTar", func() error {
			return modules.Step09LncTar(ctx, cfg)
		}); err != nil {
			return err
		}
	}

	// Step 10: IntaRNA (optional)
	if cfg.EnableIntaRNA {
		if err := tuiStep(ctx, cfg, program, 10, "Cross-Validating Targets with IntaRNA", "IntaRNA", func() error {
			return modules.Step10IntaRNA(ctx, cfg)
		}); err != nil {
			return err
		}
	}

	// Step 11: Consensus
	if err := tuiStep(ctx, cfg, program, 11, "Cross-Tool Consensus Analysis", "consensus", func() error {
		return modules.Step11Consensus(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 12: Enrichment
	if err := tuiStep(ctx, cfg, program, 12, "Building Enrichment Gene Lists", "bedtools", func() error {
		return modules.Step12Enrichment(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 13: RSeQC
	if err := tuiStep(ctx, cfg, program, 13, "RNA-seq Quality Assessment with RSeQC", "RSeQC", func() error {
		return modules.Step13RSeQC(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 14: IGV Report
	if err := tuiStep(ctx, cfg, program, 14, "Creating IGV Genome Browser Report", "create_report", func() error {
		return modules.Step14IGV(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 15: MultiQC
	if err := tuiStep(ctx, cfg, program, 15, "Generating MultiQC Report", "multiqc", func() error {
		return modules.Step15MultiQC(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 16: Generate Summary Report
	if err := tuiStep(ctx, cfg, program, 16, "Generating Pipeline Summary Report", "report", func() error {
		return modules.Step16GenerateReport(ctx, cfg)
	}); err != nil {
		return err
	}

	return nil
}

// generateReportOnly generates a summary report from an existing output directory
func generateReportOnly(outputDir string) error {
	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return fmt.Errorf("output directory does not exist: %s", outputDir)
	}

	// Check if it looks like a valid pipeline output
	requiredDirs := []string{
		"08_lncrna_analysis",
		"12_enrichment",
	}

	for _, dir := range requiredDirs {
		dirPath := filepath.Join(outputDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			return fmt.Errorf("directory %s not found - this doesn't appear to be a valid pipeline output directory", dir)
		}
	}

	fmt.Printf("Generating report for: %s\n", outputDir)

	// Create minimal config with output directory
	cfg := config.NewConfig()
	cfg.OutputDir = outputDir

	// Try to infer some settings from the output
	// Check if sortmerna was used
	if _, err := os.Stat(filepath.Join(outputDir, "03_sortmerna")); err == nil {
		cfg.EnableSortMeRNA = true
	}

	// Generate report
	ctx := context.Background()
	if err := modules.Step16GenerateReport(ctx, cfg); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	return nil
}

func runInternalJob(configPath string) {
	// Load configuration
	cfg, err := utils.LoadConfigJSON(configPath)
	if err != nil {
		fmt.Printf("Critical Error: Failed to load job config: %v\n", err)
		os.Exit(1)
	}

	// Initialize Logger
	logFile := filepath.Join(cfg.OutputDir, "pipeline.log")
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	if err := utils.InitLogger(logFile); err != nil {
		fmt.Printf("Critical Error: Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.Sync()

	utils.Info("Starting Internal Slurm Job",
		zap.String("run_id", cfg.RunID),
		zap.String("output_dir", cfg.OutputDir))

	// Run Pipeline
	runner := modules.NewPipelineRunner(cfg)
	ctx := context.Background()

	if err := runner.RunHeadless(ctx); err != nil {
		utils.Error("Pipeline failed", zap.Error(err))
		os.Exit(1) // Exit with error so Slurm knows it failed
	}

	utils.Info("Pipeline completed successfully")
}
