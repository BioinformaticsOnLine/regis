package main

import "fmt"

// printBanner displays the REGIS logo and contact information
func printBanner() {
	// Cyan REGIS logo
	cyan := "\033[36m"
	gold := "\033[33m"
	gray := "\033[90m"
	reset := "\033[0m"

	fmt.Println(cyan + `
  ██████╗ ███████╗ ██████╗ ██╗███████╗
  ██╔══██╗██╔════╝██╔════╝ ██║██╔════╝
  ██████╔╝█████╗  ██║  ███╗██║███████╗
  ██╔══██╗██╔══╝  ██║   ██║██║╚════██║
  ██║  ██║███████╗╚██████╔╝██║███████║
  ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝╚══════╝` + reset)

	fmt.Println(gold + "  RNA-seq Guided Identification System")
	fmt.Println("  lncRNA Discovery Pipeline v1.0" + reset)
	fmt.Println(gray + "  Bugs: github.com/pranjalpruthi")
	fmt.Println("  Jitendralab CSIR IGIB | PI: Dr. Jitendra Narayan" + reset)
	fmt.Println()
}

// printUsage displays usage information
func printUsage() {
	// Color codes
	gray := "\033[90m"
	reset := "\033[0m"

	fmt.Println("Usage: regis -t <type> -m <method> -o <output> -f1 <file1> [OPTIONS]")
	fmt.Println("\n" + "\033[1mRequired Options:\033[0m")
	fmt.Println("  -t     Data type: 'single' or 'paired'")
	fmt.Println("  -m     Analysis method: 'denovo' or 'reference'")
	fmt.Println("  -o     Output directory")
	fmt.Println("  -f1    Input FASTQ file 1")

	fmt.Println("\n" + "\033[1mConditional:\033[0m")
	fmt.Println("  -f2    Input FASTQ file 2 (required for paired-end)")
	fmt.Println("  -r     Reference genome FASTA (required for reference mode)")
	fmt.Println("  -g     Annotation GTF/GFF file (required for reference mode)")

	fmt.Println("\n" + "\033[1mOptional:\033[0m")
	fmt.Println("  -s     Species name (Human, Mouse, Fly, Zebrafish)")
	fmt.Println("  -c     Number of CPU cores (default: auto-detect)")
	fmt.Println("  --sortmerna         Enable rRNA filtering (recommended)")
	fmt.Println("  --skip-cpat        Force CPC2-only validation")
	fmt.Println("  --cpat-hex <file>  Custom CPAT hexamer model")
	fmt.Println("  --cpat-logit <file> Custom CPAT logit model")

	fmt.Println("\n" + "\033[1mLncTar Options:\033[0m")
	fmt.Println("  --lnctar           Enable LncTar target prediction")
	fmt.Println("  --lnctar-best      Run on best candidates only (fastest)")
	fmt.Println("  --lnctar-highly    Run on highly expressed lncRNAs")
	fmt.Println("  --lnctar-all       Run on all lncRNAs (comprehensive)")

	fmt.Println("\n" + "\033[1mIntaRNA Options:\033[0m")
	fmt.Println("  --intarna          Enable IntaRNA target prediction")
	fmt.Println("  --intarna-best     Run on best candidates only (fastest)")
	fmt.Println("  --intarna-highly   Run on highly expressed lncRNAs")
	fmt.Println("  --intarna-all      Run on all lncRNAs (comprehensive)")

	fmt.Println("\n" + "\033[1mExamples:\033[0m")
	fmt.Println("  # Basic paired-end reference mode")
	fmt.Println("  regis -t paired -m reference -r genome.fa -g annot.gtf -o output -f1 R1.fq -f2 R2.fq")
	fmt.Println()
	fmt.Println("  # With rRNA filtering (recommended)")
	fmt.Println("  regis -t paired -m reference -r genome.fa -g annot.gtf -o output \\")
	fmt.Println("        -f1 R1.fq -f2 R2.fq --sortmerna")
	fmt.Println()
	fmt.Println("  # With rRNA filtering + target prediction")
	fmt.Println("  regis -t paired -m reference -r genome.fa -g annot.gtf -o output \\")
	fmt.Println("        -f1 R1.fq -f2 R2.fq --sortmerna --lnctar-best --intarna-best")
	fmt.Println()

	// Contact footer
	fmt.Println(gray + "═══════════════════════════════════════════════════════════════════" + reset)
	fmt.Println()
	fmt.Println(gray + "  Bugs: " + reset + "\033[36mgithub.com/pranjalpruthi\033[0m")
	fmt.Println()
}
