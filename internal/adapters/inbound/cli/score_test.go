package cli_test

import (
	"bytes"
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/inbound/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../../testdata/go-hexagonal/perfect"

func TestScoreCommand_JSON(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", fixtureDir, "--json"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `"overall"`)
	assert.Contains(t, buf.String(), `"categories"`)
}

func TestScoreCommand_CIFails(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"score", fixtureDir, "--ci", "--min", "100"})
	assert.Error(t, cmd.Execute())
}

func TestScoreCommand_CIPasses(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"score", fixtureDir, "--ci", "--min", "1"})
	assert.NoError(t, cmd.Execute())
}

func TestScoreCommand_Badge(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", fixtureDir, "--badge"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "img.shields.io")
}

func TestScoreCommand_DefaultTUI(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"score", fixtureDir})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "OpenKraft")
	assert.Contains(t, buf.String(), "100")
}
