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
  verifiability: 0.30
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectTypeCLI, cfg.ProjectType)
	assert.InDelta(t, 0.30, cfg.Weights["verifiability"], 0.001)
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
	assert.InDelta(t, 0.20, cfg.Weights["discoverability"], 0.001)
	assert.Contains(t, cfg.Skip.SubMetrics, "interface_contracts")
}

func TestYAMLLoader_ExplicitWeightsOverrideTypeDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
project_type: cli-tool
weights:
  discoverability: 0.50
  code_health: 0.10
  structure: 0.05
  verifiability: 0.15
  context_quality: 0.10
  predictability: 0.10
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)

	// Explicit weights should win over CLI defaults
	assert.InDelta(t, 0.50, cfg.Weights["discoverability"], 0.001)
	assert.InDelta(t, 0.10, cfg.Weights["code_health"], 0.001)
}

func TestYAMLLoader_ExplicitSkipsOverrideTypeDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
project_type: cli-tool
skip:
  sub_metrics:
    - expected_layers
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)

	// Explicit skips replace type defaults entirely
	assert.Contains(t, cfg.Skip.SubMetrics, "expected_layers")
	assert.NotContains(t, cfg.Skip.SubMetrics, "interface_contracts")
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
  verifiability: 60
  discoverability: 70
`)
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 60, cfg.MinThresholds["verifiability"])
	assert.Equal(t, 70, cfg.MinThresholds["discoverability"])
}

func TestYAMLLoader_EmptyFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "")
	loader := appconfig.New()

	cfg, err := loader.Load(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.ProjectType)
}
