package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/inbound/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCommand_SingleModule(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"check", "payments", "--path", fixtureDir})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "payments")
}

func TestCheckCommand_JSON(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"check", "payments", "--path", fixtureDir, "--json"})
	require.NoError(t, cmd.Execute())

	var result map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err, "output should be valid JSON")
	assert.Contains(t, result, "module")
	assert.Contains(t, result, "missing_files")
}

func TestCheckCommand_All(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"check", "--all", "--path", fixtureDir})
	require.NoError(t, cmd.Execute())
	output := buf.String()
	// Should contain at least one module name
	assert.True(t, len(output) > 0, "should produce output")
}

func TestCheckCommand_AllJSON(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"check", "--all", "--path", fixtureDir, "--json"})
	require.NoError(t, cmd.Execute())

	var result []map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err, "output should be valid JSON array")
	assert.True(t, len(result) >= 1, "should have at least 1 report")
}

func TestCheckCommand_CIFails(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"check", "payments", "--path", fixtureDir, "--ci", "--min", "100"})
	err := cmd.Execute()
	assert.Error(t, err, "CI mode should fail when score is below minimum")
}

func TestCheckCommand_CIPasses(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"check", "payments", "--path", fixtureDir, "--ci", "--min", "0"})
	err := cmd.Execute()
	assert.NoError(t, err, "CI mode should pass when score is above minimum")
}

func TestCheckCommand_NoArgsNoAll(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"check", "--path", fixtureDir})
	err := cmd.Execute()
	assert.Error(t, err, "should require either a module name or --all flag")
}
