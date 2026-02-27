package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/application"
)

func newOnboardCmd() *cobra.Command {
	var (
		force  bool
		format string
	)

	cmd := &cobra.Command{
		Use:   "onboard [path]",
		Short: "Generate a consistency contract (CLAUDE.md) from codebase analysis",
		Long:  "Analyze the codebase and generate a CLAUDE.md that serves as the executable specification of project standards.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			svc := application.NewOnboardService(
				scanner.New(),
				detector.New(),
				parser.New(),
				config.New(),
			)

			report, err := svc.GenerateReport(absPath)
			if err != nil {
				return fmt.Errorf("onboard failed: %w", err)
			}

			if format == "json" {
				data, err := svc.RenderJSON(report)
				if err != nil {
					return fmt.Errorf("rendering JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// Markdown format - write CLAUDE.md
			contract := svc.RenderContract(report)

			claudePath := filepath.Join(absPath, "CLAUDE.md")
			if _, err := os.Stat(claudePath); err == nil && !force {
				return fmt.Errorf("CLAUDE.md already exists at %s (use --force to overwrite)", claudePath)
			}

			if err := os.WriteFile(claudePath, []byte(contract), 0644); err != nil {
				return fmt.Errorf("writing CLAUDE.md: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), contract)
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", claudePath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing CLAUDE.md")
	cmd.Flags().StringVar(&format, "format", "md", "Output format: md or json")

	return cmd
}
