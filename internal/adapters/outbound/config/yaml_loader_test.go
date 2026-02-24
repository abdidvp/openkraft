package config_test

import (
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".openkraft.yaml"), []byte(content), 0644))
}

func TestYAMLLoader_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, domain.DefaultConfig(), cfg)
}

func TestYAMLLoader_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
project_type: cli-tool
weights:
  tests: 0.30
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectTypeCLI, cfg.ProjectType)
	assert.InDelta(t, 0.30, cfg.Weights["tests"], 0.001)
}

func TestYAMLLoader_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{{{invalid yaml`)
	loader := appconfig.New()

	_, err := loader.Load(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing .openkraft.yaml")
}

func TestYAMLLoader_ProjectTypeMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `project_type: cli-tool`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)

	// Should get CLI defaults for weights and skips
	assert.InDelta(t, 0.25, cfg.Weights["conventions"], 0.001)
	assert.Contains(t, cfg.Skip.SubMetrics, "handler_patterns")
}

func TestYAMLLoader_ExplicitWeightsOverrideTypeDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
project_type: cli-tool
weights:
  conventions: 0.50
  architecture: 0.10
  patterns: 0.05
  tests: 0.15
  ai_context: 0.10
  completeness: 0.10
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)

	// Explicit weights should win over CLI defaults
	assert.InDelta(t, 0.50, cfg.Weights["conventions"], 0.001)
	assert.InDelta(t, 0.10, cfg.Weights["architecture"], 0.001)
}

func TestYAMLLoader_ExplicitSkipsOverrideTypeDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
project_type: cli-tool
skip:
  sub_metrics:
    - entity_patterns
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)

	// Explicit skips replace type defaults entirely
	assert.Contains(t, cfg.Skip.SubMetrics, "entity_patterns")
	assert.NotContains(t, cfg.Skip.SubMetrics, "handler_patterns")
}

func TestYAMLLoader_ExcludePaths(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
exclude_paths:
  - generated/
  - proto/
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"generated/", "proto/"}, cfg.ExcludePaths)
}

func TestYAMLLoader_MinThresholds(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
min_thresholds:
  tests: 60
  conventions: 70
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 60, cfg.MinThresholds["tests"])
	assert.Equal(t, 70, cfg.MinThresholds["conventions"])
}

func TestYAMLLoader_EmptyFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "")
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.ProjectType)
}
