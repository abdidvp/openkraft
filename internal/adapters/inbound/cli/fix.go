package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/application"
	"github.com/abdidvp/openkraft/internal/domain"
)

func newFixCmd() *cobra.Command {
	var (
		dryRun   bool
		autoOnly bool
		category string
	)

	cmd := &cobra.Command{
		Use:   "fix [path]",
		Short: "Detect drift from project patterns and apply fixes",
		Long:  "Score the project, detect drift from established patterns, apply safe fixes, and return structured instructions for complex corrections.",
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

			sc := scanner.New()
			det := detector.New()
			par := parser.New()
			cfg := config.New()

			scoreSvc := application.NewScoreService(sc, det, par, cfg)
			onboardSvc := application.NewOnboardService(sc, det, par, cfg)
			fixSvc := application.NewFixService(scoreSvc, onboardSvc)

			opts := domain.FixOptions{
				DryRun:   dryRun,
				AutoOnly: autoOnly,
				Category: category,
			}

			plan, err := fixSvc.PlanFixes(absPath, opts)
			if err != nil {
				return fmt.Errorf("fix failed: %w", err)
			}

			// Output as JSON
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(plan)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show plan without applying fixes")
	cmd.Flags().BoolVar(&autoOnly, "auto-only", false, "Only apply safe auto-fixes")
	cmd.Flags().StringVar(&category, "category", "", "Fix only a specific category")

	return cmd
}
