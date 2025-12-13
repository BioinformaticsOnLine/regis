package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"github.com/shenwei356/bio/seqio/fastx"
)

// ExtractSequences filters a FASTA/FASTQ file and keeps only records with IDs in the provided list file.
// Equivalent to: seqkit grep -f ids.txt input.fa -o output.fa
func ExtractSequences(ctxID string, idsFile, inputFile, outputFile string) error {
	start := time.Now()
	
	// 1. Read IDs into a map for O(1) lookup
	ids, err := ReadLines(idsFile)
	if err != nil {
		return fmt.Errorf("failed to read IDs file: %w", err)
	}

	idMap := make(map[string]bool, len(ids))
	for _, id := range ids {
		// Trim whitespace just in case
		idMap[strings.TrimSpace(id)] = true
	}

	// 2. Open Input File using fastx (handles gzip automatically)
	reader, err := fastx.NewDefaultReader(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input sequence file: %w", err)
	}

	// 3. Open Output File
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// 4. Iterate and Filter
	var count int
	var kept int
	
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading sequence record: %w", err)
		}
		count++

		// Check if ID is in the allowlist
		// fastx Record Name is the ID (without >)
		if idMap[string(record.Name)] {
			// Write to output using proper formatting
			// record.Format(0) writes ID + Seq + desc if present, width 0 = no wrapping
			if _, err := out.Write(record.Format(60)); err != nil {
				return fmt.Errorf("error writing sequence record: %w", err)
			}
			kept++
		}
	}

	Info(fmt.Sprintf("[%s] Extracted %d/%d sequences to %s", ctxID, kept, count, outputFile))
	Info(fmt.Sprintf("Extraction took %v", time.Since(start)))

	return nil
}

// ExtractSequencesFromList filters a FASTA/FASTQ file based on a provided slice of IDs
func ExtractSequencesFromList(ctxID string, ids []string, inputFile, outputFile string) error {
	// Create a temporary ID list file primarily to reuse the file-based logic 
	// or just implement logic here. Implementing direct logic is better for performance.
	
	start := time.Now()

	idMap := make(map[string]bool, len(ids))
	for _, id := range ids {
		idMap[strings.TrimSpace(id)] = true
	}

	reader, err := fastx.NewDefaultReader(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input sequence file: %w", err)
	}

	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	var count int
	var kept int

	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading sequence record: %w", err)
		}
		count++

		if idMap[string(record.Name)] {
			if _, err := out.Write(record.Format(60)); err != nil {
				return fmt.Errorf("error writing sequence record: %w", err)
			}
			kept++
		}
	}

	Info(fmt.Sprintf("[%s] Extracted %d/%d sequences", ctxID, kept, count))
	Info(fmt.Sprintf("Extraction took %v", time.Since(start)))

	return nil
}

// GetSequenceLengths reads a FASTA file and returns a map of ID -> Length
func GetSequenceLengths(inputFile string) (map[string]int, error) {
	reader, err := fastx.NewDefaultReader(inputFile)
	if err != nil {
		return nil, err
	}

	lengths := make(map[string]int)
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		lengths[string(record.Name)] = len(record.Seq.Seq)
	}
	return lengths, nil
}
