package cli

import (
	"encoding/json"
	"fmt"

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/adapters/outbound/tui"
	"github.com/openkraft/openkraft/internal/application"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/spf13/cobra"
)

func newScoreCmd() *cobra.Command {
	var (
		jsonOutput bool
		ciMode     bool
		minScore   int
		badge      bool
	)

	cmd := &cobra.Command{
		Use:   "score [path]",
		Short: "Score your codebase's AI-readiness",
		Long:  "Analyze a Go project and produce a Lighthouse-style AI-readiness score.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			svc := application.NewScoreService(
				scanner.New(),
				detector.New(),
				parser.New(),
			)

			score, err := svc.ScoreProject(path)
			if err != nil {
				return fmt.Errorf("scoring failed: %w", err)
			}

			switch {
			case jsonOutput:
				return renderJSON(cmd, score)
			case badge:
				return renderBadge(cmd, score)
			default:
				fmt.Fprint(cmd.OutOrStdout(), tui.RenderScore(score))
			}

			if ciMode && score.Overall < minScore {
				return fmt.Errorf("score %d is below minimum %d", score.Overall, minScore)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output score as JSON")
	cmd.Flags().BoolVar(&ciMode, "ci", false, "CI mode: exit 1 if below --min")
	cmd.Flags().IntVar(&minScore, "min", 0, "Minimum score for CI mode")
	cmd.Flags().BoolVar(&badge, "badge", false, "Output shields.io badge URL")

	return cmd
}

func renderJSON(cmd *cobra.Command, score *domain.Score) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(score)
}

func renderBadge(cmd *cobra.Command, score *domain.Score) error {
	color := domain.BadgeColor(score.Overall)
	url := fmt.Sprintf("https://img.shields.io/badge/openkraft-%d%%2F100-%s", score.Overall, color)
	fmt.Fprintln(cmd.OutOrStdout(), url)
	return nil
}
