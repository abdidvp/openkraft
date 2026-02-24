package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewOpenKraftMCPServer creates a new MCP server with all OpenKraft tools and
// resources registered. The projectPath is the root directory of the project
// to analyze.
func NewOpenKraftMCPServer(projectPath string) *server.MCPServer {
	s := server.NewMCPServer(
		"openkraft",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
	)

	registerTools(s, projectPath)
	registerResources(s, projectPath)

	return s
}
