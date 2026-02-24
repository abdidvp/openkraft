package cli

import (
	mcpadapter "github.com/openkraft/openkraft/internal/adapters/inbound/mcp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
		Long:  "Commands for running the OpenKraft MCP (Model Context Protocol) server.",
	}
	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	var projectPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start OpenKraft MCP server (stdio)",
		Long:  "Start the OpenKraft MCP server using stdio transport. This allows AI coding assistants to query project scores, blueprints, and conventions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectPath == "" {
				projectPath = "."
			}
			s := mcpadapter.NewOpenKraftMCPServer(projectPath)
			return server.ServeStdio(s)
		},
	}

	cmd.Flags().StringVar(&projectPath, "path", "", "Project path (defaults to current working directory)")

	return cmd
}
