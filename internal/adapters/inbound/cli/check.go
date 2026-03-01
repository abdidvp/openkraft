package cli

import (
	"encoding/json"
	"fmt"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/tui"
	"github.com/abdidvp/openkraft/internal/application"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var (
		all        bool
		jsonOutput bool
		ciMode     bool
		minScore   int
		path       string
	)

	cmd := &cobra.Command{
		Use:   "check [module]",
		Short: "Check a module against the golden module's blueprint",
		Long:  "Compare a module (or all modules) against the structural blueprint extracted from the best module in the project.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("specify a module name or use --all to check all modules")
			}

			projectPath := path

			svc := application.NewCheckService(
				scanner.New(),
				detector.New(),
				parser.New(),
				config.New(),
			)

			if all {
				return runCheckAll(cmd, svc, projectPath, jsonOutput, ciMode, minScore)
			}

			moduleName := args[0]
			return runCheckSingle(cmd, svc, projectPath, moduleName, jsonOutput, ciMode, minScore)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Check all non-golden modules")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&ciMode, "ci", false, "CI mode: exit 1 if any module is below --min")
	cmd.Flags().IntVar(&minScore, "min", 0, "Minimum score for CI mode")
	cmd.Flags().StringVar(&path, "path", ".", "Project path to analyze")

	return cmd
}

func runCheckSingle(
	cmd *cobra.Command,
	svc *application.CheckService,
	projectPath, moduleName string,
	jsonOutput, ciMode bool,
	minScore int,
) error {
	report, err := svc.CheckModule(projectPath, moduleName)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	if jsonOutput {
		return renderCheckJSON(cmd, report)
	}

	fmt.Fprint(cmd.OutOrStdout(), tui.RenderCheckReport(report))

	if ciMode && report.Score < minScore {
		return fmt.Errorf("module %s score %d is below minimum %d", report.Module, report.Score, minScore)
	}

	return nil
}

func runCheckAll(
	cmd *cobra.Command,
	svc *application.CheckService,
	projectPath string,
	jsonOutput, ciMode bool,
	minScore int,
) error {
	reports, err := svc.CheckAll(projectPath)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	if jsonOutput {
		return renderCheckAllJSON(cmd, reports)
	}

	for _, report := range reports {
		fmt.Fprint(cmd.OutOrStdout(), tui.RenderCheckReport(report))
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if ciMode {
		for _, report := range reports {
			if report.Score < minScore {
				return fmt.Errorf("module %s score %d is below minimum %d", report.Module, report.Score, minScore)
			}
		}
	}

	return nil
}

func renderCheckJSON(cmd *cobra.Command, report *domain.CheckReport) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func renderCheckAllJSON(cmd *cobra.Command, reports []*domain.CheckReport) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(reports)
}
