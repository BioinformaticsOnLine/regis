package tui

import (
	"fmt"
	"runtime"

	"github.com/BioinformaticsOnLine/regis/version"
)

// const version = "1.0.0" // Removed

func ShowBanner() {
	// ASCII Logo
	logo := `
  ██████╗ ███████╗ ██████╗ ██╗███████╗
  ██╔══██╗██╔════╝██╔════╝ ██║██╔════╝
  ██████╔╝█████╗  ██║  ███╗██║███████╗
  ██╔══██╗██╔══╝  ██║   ██║██║╚════██║
  ██║  ██║███████╗╚██████╔╝██║███████║
  ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝╚══════╝
`

	fmt.Println(logo)
	fmt.Println("  RNA-seq Guided Identification System")
	fmt.Println("  lncRNA Discovery Pipeline v" + version.Version)
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Usage
	fmt.Println("USAGE:")
	fmt.Println("  regis [OPTIONS]")
	fmt.Println()

	// Required flags
	fmt.Println("REQUIRED FLAGS:")
	fmt.Println("  -t string     Data type: 'single' or 'paired'")
	fmt.Println("  -m string     Analysis method: 'denovo' or 'reference'")
	fmt.Println("  -f1 string    Input file 1 (or single-end file)")
	fmt.Println("  -o string     Output directory")
	fmt.Println()

	// Conditional flags
	fmt.Println("CONDITIONAL FLAGS:")
	fmt.Println("  -r string     Reference genome FASTA (required for reference mode)")
	fmt.Println("  -g string     Annotation GTF/GFF (required for reference mode)")
	fmt.Println("  -f2 string    Input file 2 (required for paired-end)")
	fmt.Println()

	// Optional flags
	fmt.Println("OPTIONAL FLAGS:")
	fmt.Println("  -e string     User email (optional for job tracking)")
	fmt.Println("  -p int        Number of threads (default: all available)")
	fmt.Println("  -s string     Species for CPAT (Human, Mouse, Fly, Zebrafish)")
	fmt.Println("  --skip-cpat   Force CPC2-only validation mode")
	fmt.Println("  --sortmerna   Enable rRNA filtering using SortMeRNA")
	fmt.Println()

	// Target prediction
	fmt.Println("TARGET PREDICTION:")
	fmt.Println("  LncTar Options:")
	fmt.Println("    --lnctar-best       Analyze best candidates only")
	fmt.Println("    --lnctar-highly     Analyze highly expressed lncRNAs")
	fmt.Println("    --lnctar-all        Analyze all lncRNAs (comprehensive)")
	fmt.Println()
	fmt.Println("  IntaRNA Options:")
	fmt.Println("    --intarna-best      Analyze best candidates only")
	fmt.Println("    --intarna-highly    Analyze highly expressed lncRNAs")
	fmt.Println("    --intarna-all       Analyze all lncRNAs (comprehensive)")
	fmt.Println()

	// Examples
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Paired-end, reference-based with target prediction")
	fmt.Println("  regis -t paired -m reference \\")
	fmt.Println("        -r genome.fna -g annotation.gff \\")
	fmt.Println("        -f1 reads_1.fq -f2 reads_2.fq \\")
	fmt.Println("        -o output/ \\")
	fmt.Println("        --lnctar-best --intarna-best")
	fmt.Println()
	fmt.Println("  # Single-end, de novo assembly")
	fmt.Println("  regis -t single -m denovo \\")
	fmt.Println("        -f1 reads.fq -o output/ \\")
	fmt.Println("        -s Human")
	fmt.Println()

	// System info
	fmt.Println("SYSTEM INFO:")
	fmt.Printf("  Go Version: %s\n", runtime.Version())
	fmt.Printf("  Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  CPUs:       %d\n", runtime.NumCPU())
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("REGIS Team:")
	fmt.Println("  Dr. Jitendra Narayan (Principal Investigator)")
	fmt.Println("  Dr. Stefano Tiozzo (CNRS-Sorbonne University)")
	fmt.Println("  Pranjal Pruthi (Researcher Programmer, CSIR IGIB)")
	fmt.Println()
	fmt.Println("Funded by The Rockefeller Foundation and CSIR-IGIB.")
	fmt.Println()
	fmt.Println("Bugs: github.com/pranjalpruthi")
	fmt.Println()
}
