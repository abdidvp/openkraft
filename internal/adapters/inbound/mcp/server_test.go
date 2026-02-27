package mcp_test

import (
	"testing"

	mcpadapter "github.com/openkraft/openkraft/internal/adapters/inbound/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenKraftMCPServer(t *testing.T) {
	s := mcpadapter.NewOpenKraftMCPServer(".")
	require.NotNil(t, s)
}

func TestMCPServerHasTools(t *testing.T) {
	s := mcpadapter.NewOpenKraftMCPServer(".")
	require.NotNil(t, s)

	tools := s.ListTools()
	require.NotNil(t, tools)

	expectedTools := []string{
		"openkraft_score",
		"openkraft_check_module",
		"openkraft_get_blueprint",
		"openkraft_get_golden_example",
		"openkraft_get_conventions",
		"openkraft_check_file",
		"openkraft_onboard",
		"openkraft_fix",
		"openkraft_validate",
	}

	for _, name := range expectedTools {
		_, exists := tools[name]
		assert.True(t, exists, "tool %q should be registered", name)
	}

	assert.Len(t, tools, len(expectedTools), "should have exactly %d tools", len(expectedTools))
}
