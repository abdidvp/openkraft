package cli_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/adapters/inbound/cli"
	"github.com/stretchr/testify/assert"
)

func TestMCPCommandExists(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"mcp", "--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestMCPServeCommandExists(t *testing.T) {
	cmd := cli.NewRootCmdForTest()
	cmd.SetArgs([]string{"mcp", "serve", "--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
}
