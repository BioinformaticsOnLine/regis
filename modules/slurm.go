package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BioinformaticsOnLine/regis/config"
)

// GenerateSbatchScript creates a Slurm submission script for the Regis pipeline
func GenerateSbatchScript(cfg *config.Config) (string, error) {
	scriptPath := filepath.Join(cfg.OutputDir, "submit_regis.sh")

	// Default values
	if cfg.Slurm.JobName == "" {
		cfg.Slurm.JobName = fmt.Sprintf("regis_%s", cfg.RunID[:8])
	}
	if cfg.Slurm.Nodes == 0 {
		cfg.Slurm.Nodes = 1
	}
	if cfg.Slurm.CPUs == 0 {
		// Use general threads config or default to 8
		if cfg.Threads > 0 {
			cfg.Slurm.CPUs = cfg.Threads
		} else {
			cfg.Slurm.CPUs = 8
		}
	}
	if cfg.Slurm.Memory == "" {
		cfg.Slurm.Memory = "16G"
	}
	if cfg.Slurm.Partition == "" {
		cfg.Slurm.Partition = "compute" // Standard default
	}

	// Determine absolute path to the current executable (regis binary)
	// This ensures the worker runs the exact same binary
	exePath, err := os.Executable()
	if err != nil {
		exePath = "regis" // Fallback to PATH
	}

	// Construct the command that the worker will run
	// It basically replicates what "modules.RunHeadless" does, but via CLI
	// regis -t paired -m denovo -o ... etc

	// IMPORTANT: We need to reconstruct the CLI args from the Config object
	// This maps API request -> CLI flags
	cmd := fmt.Sprintf("%s submit_internal --config %s", exePath, filepath.Join(cfg.OutputDir, "job_config.json"))

	// Create script content
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString(fmt.Sprintf("#SBATCH --job-name=%s\n", cfg.Slurm.JobName))
	sb.WriteString(fmt.Sprintf("#SBATCH --output=%s\n", filepath.Join(cfg.OutputDir, "slurm-%j.out")))
	sb.WriteString(fmt.Sprintf("#SBATCH --error=%s\n", filepath.Join(cfg.OutputDir, "slurm-%j.err")))
	sb.WriteString(fmt.Sprintf("#SBATCH --partition=%s\n", cfg.Slurm.Partition))
	sb.WriteString(fmt.Sprintf("#SBATCH --nodes=%d\n", cfg.Slurm.Nodes))
	sb.WriteString(fmt.Sprintf("#SBATCH --ntasks-per-node=1\n")) // Regis is single-process multi-threaded
	sb.WriteString(fmt.Sprintf("#SBATCH --cpus-per-task=%d\n", cfg.Slurm.CPUs))
	sb.WriteString(fmt.Sprintf("#SBATCH --mem=%s\n", cfg.Slurm.Memory))

	if cfg.Slurm.Time != "" {
		sb.WriteString(fmt.Sprintf("#SBATCH --time=%s\n", cfg.Slurm.Time))
	}
	if cfg.Slurm.Email != "" {
		sb.WriteString(fmt.Sprintf("#SBATCH --mail-user=%s\n", cfg.Slurm.Email))
		sb.WriteString("#SBATCH --mail-type=ALL\n")
	}

	// Auto-generate Job Name: regis-<email_username>
	// Sanitize email to be safe for Slurm (remove @ and everything after, only keeping username)
	username := cfg.Email
	if idx := strings.Index(cfg.Email, "@"); idx != -1 {
		username = cfg.Email[:idx]
	}
	// Sanitize username (alphanumeric only)
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	username = reg.ReplaceAllString(username, "")
	jobName := fmt.Sprintf("regis-%s", username)

	sb.WriteString(fmt.Sprintf("#SBATCH --job-name=%s\n", jobName))

	sb.WriteString("\n# Environment Info\n")
	sb.WriteString("echo \"Starting Regis Job on $(hostname)\"\n")
	sb.WriteString("echo \"Job ID: $SLURM_JOB_ID\"\n")
	sb.WriteString("date\n\n")

	// Custom Preamble from User (for module loads, exports, etc.)
	// Custom Preamble from User (for module loads, exports, etc.)
	if len(cfg.Slurm.ExtraScript) > 0 {
		sb.WriteString("\n# Custom User Script\n")
		for _, line := range cfg.Slurm.ExtraScript {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// Conda initialization (Critical for bioinformatics tools)
	// We only add default conda logic if user hasn't fully taken over
	// But usually it's safer to just append user script *after* or *before*?
	// The user script might set ENV vars needed for the run.
	// We'll keep default conda init for safety, unless user script handles it?
	// Let's stick to standard behavior: Init conda, then run user script, then run pipeline.

	// Default Conda Init (can be overridden by ExtraScript if they export PATHs)
	sb.WriteString("# Initialize Conda (Default)\n")
	sb.WriteString("eval \"$(conda shell.bash hook)\"\n")
	sb.WriteString("if [ -z \"$CONDA_DEFAULT_ENV\" ]; then\n")
	sb.WriteString("    conda activate regis || echo \"Warning: Could not activate regis env\"\n")
	sb.WriteString("fi\n\n")

	// Run command
	sb.WriteString("# Run Pipeline\n")
	sb.WriteString(cmd + "\n")

	// Write file
	if err := os.WriteFile(scriptPath, []byte(sb.String()), 0755); err != nil {
		return "", err
	}

	return scriptPath, nil
}

// SubmitSbatch submits the script and returns the Slurm Job ID
func SubmitSbatch(scriptPath string) (string, error) {
	cmd := exec.Command("sbatch", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Mock submission for testing if sbatch not found?
		// No, for now let it fail if sbatch missing
		return "", fmt.Errorf("sbatch failed: %v, output: %s", err, string(output))
	}

	// Parse Output: "Submitted batch job 123456"
	outStr := string(output)
	re := regexp.MustCompile(`Submitted batch job (\d+)`)
	matches := re.FindStringSubmatch(outStr)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("could not parse job ID from sbatch output: %s", outStr)
}
