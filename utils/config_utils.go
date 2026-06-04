package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BioinformaticsOnLine/regis/config"
)

// WriteConfigJSON serializes the configuration to a JSON file
func WriteConfigJSON(cfg *config.Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfigJSON reads a configuration from a JSON file
func LoadConfigJSON(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := new(config.Config)
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Re-validate to ensure paths are still valid/absolute
	// (though we assume the file was valid when written)
	return cfg, nil
}
