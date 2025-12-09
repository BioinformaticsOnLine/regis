package main

import (
	"context"
	"flag"
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
	"github.com/BioinformaticsOnLine/regis/utils"
	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

func main() {
	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
			port := serveCmd.String("port", "3000", "Port to run the server on")
			jobDir := serveCmd.String("job-dir", "./jobs", "Directory to store job outputs")
			serveCmd.Parse(os.Args[2:])
			api.StartServer(*port, *jobDir)
			return

		case "submit_internal":
			// Internal command used by Slurm jobs to execute the pipeline
			// Usage: regis submit_internal --config /path/to/job_config.json
			submitCmd := flag.NewFlagSet("submit_internal", flag.ExitOnError)
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
	// Define CLI flags matching bash script interface
	dataType := flag.String("t", "", "Data type: 'single' or 'paired'")
	method := flag.String("m", "", "Analysis method: 'denovo' or 'reference'")
	file1 := flag.String("f1", "", "Input file 1 (or single-end file)")
	file2 := flag.String("f2", "", "Input file 2 (for paired-end)")
	reference := flag.String("r", "", "Reference genome FASTA file")
	gtf := flag.String("g", "", "Annotation GTF/GFF file")
	outputDir := flag.String("o", "", "Output directory")

	// Optional flags
	species := flag.String("s", "", "Species name for CPAT (Human, Mouse, Fly, Zebrafish)")
	email := flag.String("e", "", "User email (optional)")
	emailLong := flag.String("email", "", "User email (optional)")
	cores := flag.Int("c", 0, "Number of CPU cores (default: all available)")
	threads := flag.Int("p", 0, "Number of threads (alias for -c)")

	// CPAT flags
	skipCPAT := flag.Bool("skip-cpat", false, "Force CPC2-only mode (skip CPAT)")
	cpatHex := flag.String("cpat-hex", "", "Custom CPAT hexamer model file")
	cpatLogit := flag.String("cpat-logit", "", "Custom CPAT logit model file")

	// LncTar flags
	lnctar := flag.Bool("lnctar", false, "Enable LncTar target prediction")
	lnctarBest := flag.Bool("lnctar-best", false, "Run LncTar on best candidates only")
	lnctarHighly := flag.Bool("lnctar-highly", false, "Run LncTar on highly expressed lncRNAs")
	lnctarAll := flag.Bool("lnctar-all", false, "Run LncTar on all lncRNAs (comprehensive)")
	lnctarComprehensive := flag.Bool("lnctar-comprehensive", false, "Alias for --lnctar-all")

	// IntaRNA flags
	intarna := flag.Bool("intarna", false, "Enable IntaRNA target prediction")
	intarnaBest := flag.Bool("intarna-best", false, "Run IntaRNA on best candidates only")
	intarnaHighly := flag.Bool("intarna-highly", false, "Run IntaRNA on highly expressed lncRNAs")
	intarnaAll := flag.Bool("intarna-all", false, "Run IntaRNA on all lncRNAs (comprehensive)")
	intarnaComprehensive := flag.Bool("intarna-comprehensive", false, "Alias for --intarna-all")

	// rRNA filtering
	sortmerna := flag.Bool("sortmerna", false, "Enable rRNA filtering with SortMeRNA (recommended)")

	// Report generation flag
	reportOnly := flag.String("report", "", "Generate summary report from existing output directory (path to output folder)")

	// Help flag
	help := flag.Bool("help", false, "Show help message")
	helpShort := flag.Bool("h", false, "Show help message")

	flag.Parse()

	// Check if report-only mode
	if *reportOnly != "" {
		if err := generateReportOnly(*reportOnly); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Report generated successfully in %s/16_pipeline_report/\n", *reportOnly)
		os.Exit(0)
	}

	// Show banner only if help flag is set
	if *help || *helpShort {
		tui.ShowBanner()
		os.Exit(0)
	}

	var cfg *config.Config
	var err error

	// Check if any pipeline flags were provided
	// If no flags, launch interactive mode with forms
	if *dataType == "" && *method == "" && *file1 == "" && *outputDir == "" {
		// Interactive mode - show forms
		fmt.Println("Launching interactive mode...")
		cfg, err = tui.CollectParameters()
		if err != nil {
			fmt.Printf("Error collecting parameters: %v\n", err)
			os.Exit(1)
		}
	} else {
		// CLI mode - validate required flags
		// Show help with logo if no arguments or essential flags are missing
		if len(os.Args) == 1 || *dataType == "" || *method == "" || *outputDir == "" || *file1 == "" {
			printBanner()
			printUsage()
			os.Exit(1)
		}

		// Validate required flags
		if *dataType != "single" && *dataType != "paired" {
			fmt.Fprintf(os.Stderr, "Error: -t must be 'single' or 'paired'\n")
			os.Exit(1)
		}

		if *method != "denovo" && *method != "reference" {
			fmt.Fprintf(os.Stderr, "Error: -m must be 'denovo' or 'reference'\n")
			os.Exit(1)
		}

		if *dataType == "paired" && *file2 == "" {
			fmt.Fprintf(os.Stderr, "Error: -f2 required for paired-end data\n")
			os.Exit(1)
		}

		if *method == "reference" && (*reference == "" || *gtf == "") {
			fmt.Fprintf(os.Stderr, "Error: -r and -g required for reference-based analysis\n")
			os.Exit(1)
		}

		// Determine thread count (prefer -c over -p)
		threadCount := *cores
		if threadCount == 0 {
			threadCount = *threads
		}

		// Handle email flag priority
		if *emailLong != "" {
			email = emailLong
		}

		cfg = &config.Config{
			DataType:  *dataType,
			Method:    *method,
			File1:     *file1,
			File2:     *file2,
			Reference: *reference,
			GTF:       *gtf,
			OutputDir: *outputDir,
			Threads:   threadCount,
			Email:     *email,

			// Species and CPAT options
			Species:   *species,
			SkipCPAT:  *skipCPAT,
			CPATHex:   *cpatHex,
			CPATLogit: *cpatLogit,

			// LncTar settings
			EnableLncTar:        *lnctar || *lnctarBest || *lnctarHighly || *lnctarAll || *lnctarComprehensive,
			LncTarBestOnly:      *lnctarBest,
			LncTarComprehensive: *lnctarAll || *lnctarComprehensive,

			// IntaRNA settings
			EnableIntaRNA:        *intarna || *intarnaBest || *intarnaHighly || *intarnaAll || *intarnaComprehensive,
			IntaRNABestOnly:      *intarnaBest,
			IntaRNAComprehensive: *intarnaAll || *intarnaComprehensive,

			// rRNA filtering
			EnableSortMeRNA: *sortmerna,
		}
	}

	// Enable TUI mode to suppress stdout printing (must be before InitLogger)
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

	// Validate configuration
	if err := utils.ValidateConfig(cfg); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Launch TUI
	msgChan := make(chan tea.Msg, 100)
	model := tui.NewModel(msgChan)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Set TUI program reference for logger
	utils.SetTUIProgram(program)

	// Run pipeline in goroutine
	go func() {
		ctx, cancel := utils.SetupSignalHandler()
		defer cancel()

		pipelineStart := time.Now()

		// Send pipeline metadata to TUI
		program.Send(tui.PipelineMetadataMsg{
			StartTime:        time.Now(),
			DataType:         cfg.DataType,
			Method:           cfg.Method,
			Cores:            cfg.Threads, // Assuming cfg.Threads is the correct field for cores
			Species:          cfg.Species,
			ValidationMode:   cfg.ValidationMode, // Assuming cfg.ValidationMode is the correct field
			LncTarMode:       getLncTarMode(cfg),
			IntaRNAMode:      getIntaRNAMode(cfg),
			SortMeRNAEnabled: cfg.EnableSortMeRNA,
			OriginalCommand:  strings.Join(os.Args, " "), // Full command line
		})

		// Run pipeline
		err := runPipeline(ctx, cfg, program)

		// Check if context was cancelled (user interrupted)
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

		// Send completion message
		duration := time.Since(pipelineStart)
		program.Send(tui.PipelineCompleteMsg{
			Success:  err == nil,
			Duration: duration,
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

	// Start TUI
	finalModel, err := program.Run()

	// Ignore "interrupted" error - this is expected when user quits gracefully
	if err != nil && !strings.Contains(err.Error(), "interrupted") {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Check if user requested shutdown (via 'q', 'ctrl+c', or 't')
	// Show graceful shutdown animation after TUI exits
	if m, ok := finalModel.(tui.Model); ok {
		// If pipeline completed successfully, just exit
		if m.PipelineSuccess {
			fmt.Println("\n\033[32m✓ Pipeline execution completed successfully.\033[0m")
			os.Exit(0)
		}

		// Only show shutdown if pipeline was running or user explicitly quit
		cancelled := tui.ShowGracefulShutdown()

		// If user cancelled the shutdown, keep the program running
		if cancelled {
			fmt.Println("\033[32m✓ Shutdown cancelled! Pipeline will continue running in background.\033[0m")
			fmt.Println("\033[90mPress Ctrl+C when you're ready to exit.\033[0m")
			fmt.Println()

			// Wait indefinitely until user presses Ctrl+C again
			// This allows the pipeline to continue running in the background
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			<-sigChan

			// Now kill the pipeline
			fmt.Println()
			fmt.Println("\033[33mTerminating pipeline processes...\033[0m")
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(os.Interrupt)
			time.Sleep(1 * time.Second)

			fmt.Println("\033[36m(｡･ω･)ﾉﾞ Goodbye! Process terminated.\033[0m")
		} else {
			// Countdown completed without cancellation - kill pipeline now
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

func runPipeline(ctx context.Context, cfg *config.Config, program *tea.Program) error {
	// Step 1: FastQC
	tui.SendStepStart(program, 1, "Quality Control with FastQC", "fastqc")
	if err := modules.Step01QCFastQC(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 1, false, 0)
		return fmt.Errorf("Step 1 failed: %w", err)
	}
	tui.SendStepComplete(program, 1, true, 0)

	// Step 2: Trimmomatic
	tui.SendStepStart(program, 2, "Adapter Trimming with Trimmomatic", "trimmomatic")
	if err := modules.Step02TrimTrimmomatic(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 2, false, 0)
		return fmt.Errorf("Step 2 failed: %w", err)
	}
	tui.SendStepComplete(program, 2, true, 0)

	// Step 3: SortMeRNA (optional)
	if cfg.EnableSortMeRNA {
		tui.SendStepStart(program, 3, "rRNA Filtering with SortMeRNA", "sortmerna")
		if err := modules.Step03SortMeRNA(ctx, cfg); err != nil {
			tui.SendStepComplete(program, 3, false, 0)
			return fmt.Errorf("Step 3 failed: %w", err)
		}
		tui.SendStepComplete(program, 3, true, 0)
	}

	// Step 4: Alignment/Assembly
	stepName := "De Novo Assembly with Trinity"
	if cfg.Method == "reference" {
		stepName = "Reference-based Alignment with HISAT2"
	}
	tui.SendStepStart(program, 4, stepName, "hisat2/trinity")
	if err := modules.Step04AlignAssembly(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 4, false, 0)
		return fmt.Errorf("Step 4 failed: %w", err)
	}
	tui.SendStepComplete(program, 4, true, 0)

	// Step 5: CPC2
	tui.SendStepStart(program, 5, "Coding Potential with CPC2", "cpc2")
	if err := modules.Step05CPC2(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 5, false, 0)
		return fmt.Errorf("Step 5 failed: %w", err)
	}
	tui.SendStepComplete(program, 5, true, 0)

	// Step 6: CPAT + Consensus
	tui.SendStepStart(program, 6, "Cross-Validation with CPAT", "cpat")
	if err := modules.Step06CPAT(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 6, false, 0)
		return fmt.Errorf("Step 6 failed: %w", err)
	}
	tui.SendStepComplete(program, 6, true, 0)

	// Step 7: lncRNA Filtering
	tui.SendStepStart(program, 7, "Processing GTF and Filtering lncRNAs", "seqkit/gffcompare")
	if err := modules.Step07FilterLncRNA(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 7, false, 0)
		return fmt.Errorf("Step 7 failed: %w", err)
	}
	tui.SendStepComplete(program, 7, true, 0)

	// Step 8: RNAfold
	tui.SendStepStart(program, 8, "lncRNA Secondary Structure Prediction", "RNAfold")
	if err := modules.Step08RNAfold(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 8, false, 0)
		return fmt.Errorf("Step 8 failed: %w", err)
	}
	tui.SendStepComplete(program, 8, true, 0)

	// Step 9: LncTar (optional)
	if cfg.EnableLncTar {
		tui.SendStepStart(program, 9, "Predicting lncRNA-mRNA Interactions with LncTar", "LncTar")
		if err := modules.Step09LncTar(ctx, cfg); err != nil {
			tui.SendStepComplete(program, 9, false, 0)
			return fmt.Errorf("Step 9 failed: %w", err)
		}
		tui.SendStepComplete(program, 9, true, 0)
	}

	// Step 10: IntaRNA (optional)
	if cfg.EnableIntaRNA {
		tui.SendStepStart(program, 10, "Cross-Validating Targets with IntaRNA", "IntaRNA")
		if err := modules.Step10IntaRNA(ctx, cfg); err != nil {
			tui.SendStepComplete(program, 10, false, 0)
			return fmt.Errorf("Step 10 failed: %w", err)
		}
		tui.SendStepComplete(program, 10, true, 0)
	}

	// Step 11: Consensus
	tui.SendStepStart(program, 11, "Cross-Tool Consensus Analysis", "consensus")
	if err := modules.Step11Consensus(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 11, false, 0)
		return fmt.Errorf("Step 11 failed: %w", err)
	}
	tui.SendStepComplete(program, 11, true, 0)

	// Step 12: Enrichment
	tui.SendStepStart(program, 12, "Building Enrichment Gene Lists", "bedtools")
	if err := modules.Step12Enrichment(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 12, false, 0)
		return fmt.Errorf("Step 12 failed: %w", err)
	}
	tui.SendStepComplete(program, 12, true, 0)

	// Step 13: RSeQC
	tui.SendStepStart(program, 13, "RNA-seq Quality Assessment with RSeQC", "RSeQC")
	if err := modules.Step13RSeQC(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 13, false, 0)
		return fmt.Errorf("Step 13 failed: %w", err)
	}
	tui.SendStepComplete(program, 13, true, 0)

	// Step 14: IGV Report
	tui.SendStepStart(program, 14, "Creating IGV Genome Browser Report", "create_report")
	if err := modules.Step14IGV(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 14, false, 0)
		return fmt.Errorf("Step 14 failed: %w", err)
	}
	tui.SendStepComplete(program, 14, true, 0)

	// Step 15: MultiQC
	tui.SendStepStart(program, 15, "Generating MultiQC Report", "multiqc")
	if err := modules.Step15MultiQC(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 15, false, 0)
		return fmt.Errorf("Step 15 failed: %w", err)
	}
	tui.SendStepComplete(program, 15, true, 0)

	// Step 16: Generate Summary Report
	tui.SendStepStart(program, 16, "Generating Pipeline Summary Report", "report")
	if err := modules.Step16GenerateReport(ctx, cfg); err != nil {
		tui.SendStepComplete(program, 16, false, 0)
		return fmt.Errorf("Step 16 failed: %w", err)
	}
	tui.SendStepComplete(program, 16, true, 0)

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
