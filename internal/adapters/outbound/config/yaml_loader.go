package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openkraft/openkraft/internal/domain"
	"gopkg.in/yaml.v3"
)

const fileName = ".openkraft.yaml"

// YAMLLoader implements domain.ConfigLoader by reading .openkraft.yaml.
type YAMLLoader struct{}

// New creates a YAMLLoader.
func New() *YAMLLoader { return &YAMLLoader{} }

// Load reads .openkraft.yaml from projectPath.
// Returns DefaultConfig if the file does not exist (backward compatible).
func (l *YAMLLoader) Load(projectPath string) (domain.ProjectConfig, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, fileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.DefaultConfig(), nil
		}
		return domain.ProjectConfig{}, err
	}

	var cfg domain.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return domain.ProjectConfig{}, fmt.Errorf("parsing %s: %w", fileName, err)
	}

	// Validate before merging — catches typos in user's raw input.
	if err := cfg.Validate(); err != nil {
		return domain.ProjectConfig{}, fmt.Errorf("invalid %s: %w", fileName, err)
	}

	// If project_type is set, merge type defaults under explicit values.
	if cfg.ProjectType != "" {
		defaults := domain.DefaultConfigForType(cfg.ProjectType)
		cfg = mergeConfig(defaults, cfg)
	}

	return cfg, nil
}

// mergeConfig overlays explicit overrides on top of type defaults.
// Explicit (non-zero) values always win.
func mergeConfig(base, override domain.ProjectConfig) domain.ProjectConfig {
	result := base

	// Explicit weights replace the entire map.
	if len(override.Weights) > 0 {
		result.Weights = override.Weights
	}

	// Explicit skips replace type defaults entirely.
	if len(override.Skip.Categories) > 0 {
		result.Skip.Categories = override.Skip.Categories
	}
	if len(override.Skip.SubMetrics) > 0 {
		result.Skip.SubMetrics = override.Skip.SubMetrics
	}

	if len(override.ExcludePaths) > 0 {
		result.ExcludePaths = override.ExcludePaths
	}
	if len(override.MinThresholds) > 0 {
		result.MinThresholds = override.MinThresholds
	}

	// Profile overrides are always preserved from user config.
	result.Profile = override.Profile

	return result
}
