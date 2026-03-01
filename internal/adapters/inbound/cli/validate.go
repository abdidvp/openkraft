package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	cacheAdapter "github.com/abdidvp/openkraft/internal/adapters/outbound/cache"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/application"
)

func newValidateCmd() *cobra.Command {
	var (
		strict  bool
		noCache bool
		deleted string
	)

	cmd := &cobra.Command{
		Use:   "validate <file1> [file2] ...",
		Short: "Incremental drift detection for changed files",
		Long:  "Check changed files against the project's established patterns. Returns drift issues and score impact.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine project path from first file's location or current dir
			projectPath := "."
			absPath, err := filepath.Abs(projectPath)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			sc := scanner.New()
			det := detector.New()
			par := parser.New()
			cfg := config.New()
			cacheSt := cacheAdapter.New()

			if noCache {
				_ = cacheSt.Invalidate(absPath)
			}

			scoreSvc := application.NewScoreService(sc, det, par, cfg)
			validateSvc := application.NewValidateService(sc, det, par, scoreSvc, cacheSt, cfg)

			var deletedFiles []string
			if deleted != "" {
				deletedFiles = strings.Split(deleted, ",")
			}

			result, err := validateSvc.Validate(absPath, args, nil, deletedFiles, strict)
			if err != nil {
				return fmt.Errorf("validate failed: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return err
			}

			// Exit code based on status
			switch result.Status {
			case "fail":
				return fmt.Errorf("validation failed: %d drift issue(s) detected", len(result.DriftIssues))
			case "warn":
				if strict {
					return fmt.Errorf("validation failed (strict): %d drift issue(s) detected", len(result.DriftIssues))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Fail on any drift warning")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Force full re-scan")
	cmd.Flags().StringVar(&deleted, "deleted", "", "Comma-separated deleted file paths")

	return cmd
}
