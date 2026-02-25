package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/inbound/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_CreatesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir})
	require.NoError(t, root.Execute())

	data, err := os.ReadFile(filepath.Join(tmpDir, ".openkraft.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "project_type: api")
	assert.Contains(t, string(data), "weights:")
}

func TestInitCmd_CLIToolType(t *testing.T) {
	tmpDir := t.TempDir()

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir, "--type", "cli-tool"})
	require.NoError(t, root.Execute())

	data, err := os.ReadFile(filepath.Join(tmpDir, ".openkraft.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "project_type: cli-tool")
	assert.Contains(t, string(data), "interface_contracts")
	assert.Contains(t, string(data), "module_completeness")
}

func TestInitCmd_FailsIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".openkraft.yaml"), []byte("existing"), 0644))

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir})
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInitCmd_ForceOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".openkraft.yaml"), []byte("old"), 0644))

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir, "--force"})
	require.NoError(t, root.Execute())

	data, err := os.ReadFile(filepath.Join(tmpDir, ".openkraft.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "project_type:")
	assert.NotEqual(t, "old", string(data))
}

func TestInitCmd_InvalidType(t *testing.T) {
	tmpDir := t.TempDir()

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir, "--type", "webapp"})
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown project type")
}

func TestInitCmd_ConfigContainsProfileSection(t *testing.T) {
	tmpDir := t.TempDir()

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir})
	require.NoError(t, root.Execute())

	data, err := os.ReadFile(filepath.Join(tmpDir, ".openkraft.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "# profile:")
	assert.Contains(t, content, "#   max_function_lines: 50")
	assert.Contains(t, content, "#   min_test_ratio: 0.5")
	assert.Contains(t, content, "#   naming_convention: auto")
}

func TestInitCmd_LibraryProfileValues(t *testing.T) {
	tmpDir := t.TempDir()

	root := cli.NewRootCmdForTest()
	root.SetArgs([]string{"init", tmpDir, "--type", "library"})
	require.NoError(t, root.Execute())

	data, err := os.ReadFile(filepath.Join(tmpDir, ".openkraft.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "project_type: library")
	// Library has stricter defaults
	assert.Contains(t, content, "#   max_function_lines: 40")
	assert.Contains(t, content, "#   min_test_ratio: 0.8")
	assert.Contains(t, content, "#   max_parameters: 3")
}
