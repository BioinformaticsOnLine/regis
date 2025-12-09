package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	"github.com/BioinformaticsOnLine/regis/utils"
	"go.uber.org/zap"
)

// Step03AlignAssembly handles both de novo assembly (Trinity) and reference-based alignment (HISAT2/StringTie)
func Step04AlignAssembly(ctx context.Context, cfg *config.Config) error {
	stepStart := time.Now()

	if cfg.Method == "denovo" {
		return runDeNovoAssembly(ctx, cfg, stepStart)
	} else if cfg.Method == "reference" {
		return runReferenceAlignment(ctx, cfg, stepStart)
	}

	return fmt.Errorf("invalid method: %s", cfg.Method)
}

// runDeNovoAssembly runs Trinity de novo assembly
func runDeNovoAssembly(ctx context.Context, cfg *config.Config, stepStart time.Time) error {
	utils.StepHeader(4, "De Novo Assembly with Trinity")

	// Create output directories
	assemblyDir := filepath.Join(cfg.OutputDir, "05_assembly")
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	if err := utils.CreateDirs(assemblyDir, cpc2Dir); err != nil {
		return fmt.Errorf("failed to create assembly directories: %w", err)
	}

	// Build Trinity command
	utils.ShowProgress(fmt.Sprintf("Running Trinity (using %d CPUs)", cfg.Threads))

	var args []string
	args = append(args, "--seqType", "fq")

	if cfg.DataType == "paired" {
		args = append(args, "--left", cfg.File1, "--right", cfg.File2)
	} else {
		args = append(args, "--single", cfg.File1)
	}

	args = append(args,
		"--max_memory", "50G",
		"--CPU", strconv.Itoa(cfg.Threads),
		"--output", assemblyDir,
	)

	// Run Trinity
	if err := utils.RunCommand(ctx, "Trinity", args...); err != nil {
		return fmt.Errorf("Trinity failed: %w", err)
	}

	// Verify Trinity output
	trinityFasta := filepath.Join(assemblyDir, "Trinity.fasta")
	if !utils.FileExists(trinityFasta) {
		return fmt.Errorf("Trinity output missing: %s", trinityFasta)
	}

	// Copy Trinity output to CPC2 directory
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")
	if err := utils.CopyFile(trinityFasta, transcriptsFa); err != nil {
		return fmt.Errorf("failed to copy Trinity output: %w", err)
	}

	utils.StepComplete(3, "De Novo Assembly with Trinity", stepStart)
	utils.Info("Trinity assembly complete", zap.String("output", trinityFasta))

	return nil
}

// runReferenceAlignment runs HISAT2 alignment and StringTie assembly
func runReferenceAlignment(ctx context.Context, cfg *config.Config, stepStart time.Time) error {
	utils.StepHeader(4, "Reference-based Alignment with HISAT2")

	// Create output directories
	alignmentDir := filepath.Join(cfg.OutputDir, "04_alignment")
	assemblyDir := filepath.Join(cfg.OutputDir, "05_assembly")
	cpc2Dir := filepath.Join(cfg.OutputDir, "06_cpc2")
	if err := utils.CreateDirs(alignmentDir, assemblyDir, cpc2Dir); err != nil {
		return fmt.Errorf("failed to create alignment directories: %w", err)
	}

	// Step 3.1: Build HISAT2 index
	hisat2Index := filepath.Join(alignmentDir, "hisat2_index")
	utils.ShowProgress(fmt.Sprintf("Building HISAT2 index (using %d threads)", cfg.Threads))

	if err := utils.RunCommand(ctx, "hisat2-build",
		"-p", strconv.Itoa(cfg.Threads),
		cfg.Reference,
		hisat2Index,
	); err != nil {
		return fmt.Errorf("HISAT2 index building failed: %w", err)
	}

	// Step 3.2: Align reads with HISAT2
	alignedSam := filepath.Join(alignmentDir, "aligned.sam")
	utils.ShowProgress(fmt.Sprintf("Aligning reads with HISAT2 (using %d threads)", cfg.Threads))

	var hisat2Args []string
	hisat2Args = append(hisat2Args, "-p", strconv.Itoa(cfg.Threads), "-x", hisat2Index)

	if cfg.DataType == "paired" {
		hisat2Args = append(hisat2Args, "-1", cfg.File1, "-2", cfg.File2)
	} else {
		hisat2Args = append(hisat2Args, "-U", cfg.File1)
	}

	hisat2Args = append(hisat2Args, "-S", alignedSam)

	if err := utils.RunCommand(ctx, "hisat2", hisat2Args...); err != nil {
		return fmt.Errorf("HISAT2 alignment failed: %w", err)
	}

	// Step 3.3: Convert SAM to BAM and sort
	alignedBam := filepath.Join(alignmentDir, "aligned_sorted.bam")
	utils.ShowProgress(fmt.Sprintf("Sorting BAM file (using %d threads)", cfg.Threads))

	// samtools view -@ threads -bS aligned.sam | samtools sort -@ threads -o aligned_sorted.bam
	if err := convertAndSortBAM(ctx, alignedSam, alignedBam, cfg.Threads); err != nil {
		return fmt.Errorf("BAM conversion/sorting failed: %w", err)
	}

	// Step 3.4: Index BAM file
	utils.ShowProgress(fmt.Sprintf("Indexing BAM file (using %d threads)", cfg.Threads))

	if err := utils.RunCommand(ctx, "samtools", "index",
		"-@", strconv.Itoa(cfg.Threads),
		alignedBam,
	); err != nil {
		return fmt.Errorf("BAM indexing failed: %w", err)
	}

	// Step 3.5: Run StringTie
	transcriptsGtf := filepath.Join(assemblyDir, "transcripts.gtf")
	utils.ShowProgress(fmt.Sprintf("Running StringTie (using %d threads)", cfg.Threads))

	if err := utils.RunCommand(ctx, "stringtie",
		alignedBam,
		"-p", strconv.Itoa(cfg.Threads),
		"-G", cfg.GTF,
		"-o", transcriptsGtf,
	); err != nil {
		return fmt.Errorf("StringTie failed: %w", err)
	}

	// Step 3.6: Extract transcript sequences with gffread
	transcriptsFa := filepath.Join(cpc2Dir, "transcripts.fa")
	utils.ShowProgress("Extracting transcript sequences")

	if err := utils.RunCommand(ctx, "gffread",
		"-w", transcriptsFa,
		"-g", cfg.Reference,
		transcriptsGtf,
	); err != nil {
		return fmt.Errorf("gffread failed: %w", err)
	}

	utils.StepComplete(3, "Reference-based Alignment with HISAT2", stepStart)
	utils.Info("Alignment and assembly complete",
		zap.String("bam", alignedBam),
		zap.String("gtf", transcriptsGtf),
		zap.String("fasta", transcriptsFa))

	return nil
}

// convertAndSortBAM converts SAM to BAM and sorts it
func convertAndSortBAM(ctx context.Context, samFile, bamFile string, threads int) error {
	// Implements: samtools view -bS | samtools sort
	// Done in two steps for clarity and error handling

	// Step 1: Convert SAM to BAM
	unsortedBam := samFile + ".unsorted.bam"
	if err := utils.RunCommand(ctx, "samtools", "view",
		"-@", strconv.Itoa(threads),
		"-bS", samFile,
		"-o", unsortedBam,
	); err != nil {
		return err
	}

	// Step 2: Sort BAM
	if err := utils.RunCommand(ctx, "samtools", "sort",
		"-@", strconv.Itoa(threads),
		"-o", bamFile,
		unsortedBam,
	); err != nil {
		return err
	}

	// Clean up intermediate file
	utils.RunCommand(ctx, "rm", "-f", unsortedBam)

	return nil
}
