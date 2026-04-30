package tui

import (
	"fmt"
	"strings"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	formLogoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	commandPreviewStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#6272A4")).
				Padding(1, 2).
				Foreground(lipgloss.Color("#50FA7B"))
)

func showLogo() {
	logo := `
  ██████╗ ███████╗ ██████╗ ██╗███████╗
  ██╔══██╗██╔════╝██╔════╝ ██║██╔════╝
  ██████╔╝█████╗  ██║  ███╗██║███████╗
  ██╔══██╗██╔══╝  ██║   ██║██║╚════██║
  ██║  ██║███████╗╚██████╔╝██║███████║
  ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝╚══════╝
`
	logoStr := formLogoStyle.Render(logo)
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")). // Gold
		Italic(true).
		Render("  RNA-seq Guided Identification System\n  lncRNA Discovery Pipeline v1.0")
	contact := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render("\n  Bugs: github.com/pranjalpruthi")
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render(strings.Repeat("═", 80))

	team := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Render("\n  REGIS Team:\n  Dr. Jitendra Narayan (Principal Investigator)\n  Dr. Stefano Tiozzo (CNRS-Sorbonne University)\n  Pranjal Pruthi (Researcher Programmer, CSIR IGIB)")

	banner := lipgloss.JoinVertical(lipgloss.Left,
		"",
		logoStr,
		"", // Empty line after logo
		subtitle,
		contact,
		team,
		"", // Empty line before separator
		separator,
		"", // Empty line after separator
	)
	fmt.Println(banner)
}

func buildCommandPreview(cfg *config.Config) string {
	parts := []string{"regis"}

	if cfg.DataType != "" {
		parts = append(parts, "-t", cfg.DataType)
	}
	if cfg.Method != "" {
		parts = append(parts, "-m", cfg.Method)
	}
	if cfg.File1 != "" {
		parts = append(parts, "-f1", cfg.File1)
	}
	if cfg.File2 != "" {
		parts = append(parts, "-f2", cfg.File2)
	}
	if cfg.Reference != "" {
		parts = append(parts, "-r", cfg.Reference)
	}
	if cfg.GTF != "" {
		parts = append(parts, "-g", cfg.GTF)
	}
	if cfg.OutputDir != "" {
		parts = append(parts, "-o", cfg.OutputDir)
	}
	if cfg.Threads > 0 {
		parts = append(parts, fmt.Sprintf("-p %d", cfg.Threads))
	}

	cmd := strings.Join(parts, " \\\n  ")
	return commandPreviewStyle.Render("Command Preview:\n\n" + cmd)
}

// CollectParameters shows interactive forms to collect pipeline parameters
func CollectParameters() (*config.Config, error) {
	cfg := &config.Config{}

	// Show logo once at the start
	showLogo()

	// Data type selection
	var dataType, email string
	form1 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Email (optional)").
				Value(&email).
				Validate(func(s string) error {
					if s != "" && !strings.Contains(s, "@") {
						return fmt.Errorf("invalid email format")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Data Type").
				Options(
					huh.NewOption("Paired-end", "paired"),
					huh.NewOption("Single-end", "single"),
				).
				Value(&dataType),
		),
	)

	if err := form1.Run(); err != nil {
		return nil, err
	}
	cfg.DataType = dataType
	cfg.Email = email

	// Method selection
	var method string
	form2 := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Analysis Method").
				Options(
					huh.NewOption("Reference-based", "reference"),
					huh.NewOption("De novo assembly", "denovo"),
				).
				Value(&method),
		),
	)

	if err := form2.Run(); err != nil {
		return nil, err
	}
	cfg.Method = method

	// Assembler selection (only for de novo)
	if method == "denovo" {
		var assembler string
		assemblerForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("De Novo Assembler").
					Options(
						huh.NewOption("Trinity (established, memory-heavy)", "trinity"),
						huh.NewOption("RNA-Bloom (fast, memory-efficient)", "rnabloom"),
					).
					Value(&assembler),
			),
		)

		if err := assemblerForm.Run(); err != nil {
			return nil, err
		}
		cfg.Assembler = assembler
	}

	// Strandedness selection
	var stranded string
	strandedOptions := []huh.Option[string]{
		huh.NewOption("Unstranded (Standard)", "unstranded"),
	}
	
	if dataType == "paired" {
		strandedOptions = append(strandedOptions, 
			huh.NewOption("RF (Reverse-Forward, typical dUTP)", "rf"),
			huh.NewOption("FR (Forward-Reverse)", "fr"),
		)
	} else {
		strandedOptions = append(strandedOptions, 
			huh.NewOption("F (Forward)", "f"),
			huh.NewOption("R (Reverse)", "r"),
		)
	}

	strandedForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Library Strandedness").
				Options(strandedOptions...).
				Value(&stranded),
		),
	)

	if err := strandedForm.Run(); err != nil {
		return nil, err
	}
	cfg.Stranded = stranded

	// File inputs
	var file1, file2, reference, gtf, outputDir string

	fileForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Input File 1 (or single-end file)").
				Value(&file1).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("file 1 is required")
					}
					return nil
				}),
		),
	)

	if err := fileForm.Run(); err != nil {
		return nil, err
	}
	cfg.File1 = file1

	if dataType == "paired" {
		file2Form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Input File 2 (paired-end)").
					Value(&file2).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("file 2 is required for paired-end")
						}
						return nil
					}),
			),
		)

		if err := file2Form.Run(); err != nil {
			return nil, err
		}
		cfg.File2 = file2
	}

	if method == "reference" {
		refForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Reference Genome FASTA").
					Value(&reference).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("reference genome is required")
						}
						return nil
					}),
				huh.NewInput().
					Title("Annotation GTF/GFF").
					Value(&gtf).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("annotation file is required")
						}
						return nil
					}),
			),
		)

		if err := refForm.Run(); err != nil {
			return nil, err
		}
		cfg.Reference = reference
		cfg.GTF = gtf
	}

	// Output directory
	outForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Output Directory").
				Value(&outputDir).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("output directory is required")
					}
					return nil
				}),
		),
	)

	if err := outForm.Run(); err != nil {
		return nil, err
	}
	cfg.OutputDir = outputDir

	// Optional parameters
	var coresStr string
	var species string
	var enableLncTar, enableIntaRNA bool

	optForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("CPU Cores (0 for all available)").
				Value(&coresStr),
			huh.NewInput().
				Title("Species (optional: Human, Mouse, Fly, Zebrafish)").
				Value(&species),
			huh.NewConfirm().
				Title("Enable LncTar target prediction?").
				Value(&enableLncTar),
			huh.NewConfirm().
				Title("Enable IntaRNA target prediction?").
				Value(&enableIntaRNA),
		),
	)

	if err := optForm.Run(); err != nil {
		return nil, err
	}

	// Convert cores string to int
	cores := 0
	if coresStr != "" {
		fmt.Sscanf(coresStr, "%d", &cores)
	}

	cfg.Threads = cores
	cfg.Species = species
	cfg.EnableLncTar = enableLncTar
	cfg.EnableIntaRNA = enableIntaRNA

	// CPAT advanced options (optional)
	var skipCPAT bool
	var cpatHex, cpatLogit string

	cpatForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Skip CPAT validation? (CPC2-only mode)").
				Value(&skipCPAT),
			huh.NewInput().
				Title("Custom CPAT Hexamer file (optional)").
				Value(&cpatHex),
			huh.NewInput().
				Title("Custom CPAT Logit model file (optional)").
				Value(&cpatLogit),
		),
	)

	if err := cpatForm.Run(); err != nil {
		return nil, err
	}

	cfg.SkipCPAT = skipCPAT
	cfg.CPATHex = cpatHex
	cfg.CPATLogit = cpatLogit

	// If target prediction enabled, ask for mode
	if enableLncTar {
		var lnctarMode string
		lnctarForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("LncTar Mode").
					Options(
						huh.NewOption("Best candidates only", "best"),
						huh.NewOption("Highly expressed", "highly"),
						huh.NewOption("All lncRNAs (comprehensive)", "all"),
					).
					Value(&lnctarMode),
			),
		)

		if err := lnctarForm.Run(); err != nil {
			return nil, err
		}

		cfg.LncTarBestOnly = (lnctarMode == "best")
		cfg.LncTarComprehensive = (lnctarMode == "all")
	}

	if enableIntaRNA {
		var intarnaMode string
		intarnaForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("IntaRNA Mode").
					Options(
						huh.NewOption("Best candidates only", "best"),
						huh.NewOption("Highly expressed", "highly"),
						huh.NewOption("All lncRNAs (comprehensive)", "all"),
					).
					Value(&intarnaMode),
			),
		)

		if err := intarnaForm.Run(); err != nil {
			return nil, err
		}

		cfg.IntaRNABestOnly = (intarnaMode == "best")
		cfg.IntaRNAComprehensive = (intarnaMode == "all")
	}

	// Show final command preview
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Println(buildCommandPreview(cfg))
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render("✓ Configuration complete! Starting pipeline..."))
	fmt.Println()

	return cfg, nil
}
