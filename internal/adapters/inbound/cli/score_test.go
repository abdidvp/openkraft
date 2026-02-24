package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/inbound/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../../testdata/go-hexagonal/perfect"

func cleanupHistory(t *testing.T, path string) {
	t.Helper()
	absPath, _ := filepath.Abs(path)
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(absPath, ".openkraft"))
	})
}

func TestScoreCommand_JSON(t *testing.T) {
	cleanupHistory(t, fixtureDir)
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", fixtureDir, "--json"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `"overall"`)
	assert.Contains(t, buf.String(), `"categories"`)
}

func TestScoreCommand_CIFails(t *testing.T) {
	cleanupHistory(t, fixtureDir)
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"score", fixtureDir, "--ci", "--min", "100"})
	assert.Error(t, cmd.Execute())
}

func TestScoreCommand_CIPasses(t *testing.T) {
	cleanupHistory(t, fixtureDir)
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"score", fixtureDir, "--ci", "--min", "1"})
	assert.NoError(t, cmd.Execute())
}

func TestScoreCommand_Badge(t *testing.T) {
	cleanupHistory(t, fixtureDir)
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", fixtureDir, "--badge"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "img.shields.io")
}

func TestScoreCommand_DefaultTUI(t *testing.T) {
	cleanupHistory(t, fixtureDir)
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", fixtureDir})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "openkraft")
	assert.Contains(t, buf.String(), "100")
}

func TestScoreCommand_History(t *testing.T) {
	cleanupHistory(t, fixtureDir)
	// Run score twice to build history
	cmd1 := cli.NewRootCmdForTest()
	cmd1.SetArgs([]string{"score", fixtureDir})
	cmd1.SetOut(new(bytes.Buffer))
	require.NoError(t, cmd1.Execute())

	cmd2 := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd2.SetOut(buf)
	cmd2.SetArgs([]string{"score", fixtureDir, "--history"})
	require.NoError(t, cmd2.Execute())
	assert.Contains(t, buf.String(), "Score History")
	assert.Contains(t, buf.String(), "/100")
}
