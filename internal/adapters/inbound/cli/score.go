package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/gitinfo"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/history"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/tui"
	"github.com/abdidvp/openkraft/internal/application"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/spf13/cobra"
)

func newScoreCmd() *cobra.Command {
	var (
		jsonOutput  bool
		ciMode      bool
		minScore    int
		badge       bool
		showHistory bool
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

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			svc := application.NewScoreService(
				scanner.New(),
				detector.New(),
				parser.New(),
				config.New(),
			)

			score, err := svc.ScoreProject(absPath)
			if err != nil {
				return fmt.Errorf("scoring failed: %w", err)
			}

			// Attach git commit hash if available
			gi := gitinfo.New()
			if hash, err := gi.CommitHash(absPath); err == nil {
				score.CommitHash = hash
			}

			// Save to history
			hist := history.New()
			entry := domain.ScoreEntry{
				Timestamp:  time.Now().Format(time.RFC3339),
				CommitHash: score.CommitHash,
				Overall:    score.Overall,
				Grade:      score.Grade(),
			}
			_ = hist.Save(absPath, entry) // best-effort

			// Show history if requested
			if showHistory {
				entries, err := hist.Load(absPath)
				if err != nil {
					return fmt.Errorf("loading history: %w", err)
				}
				fmt.Fprint(cmd.OutOrStdout(), tui.RenderHistory(entries))
				return nil
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
	cmd.Flags().BoolVar(&showHistory, "history", false, "Show score history")

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
