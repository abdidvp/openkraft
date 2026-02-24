package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/spf13/cobra"
)

const configFileName = ".openkraft.yaml"

func newInitCmd() *cobra.Command {
	var (
		projectType string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a .openkraft.yaml configuration file",
		Long:  "Create a .openkraft.yaml with sensible defaults for your project type.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			dest := filepath.Join(absPath, configFileName)

			if !force {
				if _, err := os.Stat(dest); err == nil {
					return fmt.Errorf("%s already exists (use --force to overwrite)", configFileName)
				}
			}

			pt := domain.ProjectType(projectType)

			// Validate project type
			if projectType != "" {
				valid := false
				for _, vt := range domain.ValidProjectTypes {
					if pt == vt {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("unknown project type %q (valid: api, cli-tool, library, microservice)", projectType)
				}
			}

			content := generateConfig(pt)

			if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", configFileName)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectType, "type", "api", "Project type (api, cli-tool, library, microservice)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing .openkraft.yaml")

	return cmd
}

func generateConfig(pt domain.ProjectType) string {
	if pt == "" {
		pt = domain.ProjectTypeAPI
	}

	cfg := domain.DefaultConfigForType(pt)

	var skipSection string
	if len(cfg.Skip.SubMetrics) > 0 {
		skipSection = "skip:\n  sub_metrics:\n"
		for _, sm := range cfg.Skip.SubMetrics {
			skipSection += fmt.Sprintf("    - %s\n", sm)
		}
	}

	weightsSection := "weights:\n"
	// Ordered output for readability
	for _, name := range domain.ValidCategories {
		if w, ok := cfg.Weights[name]; ok {
			weightsSection += fmt.Sprintf("  %s: %.2f\n", name, w)
		}
	}

	result := fmt.Sprintf("# OpenKraft configuration\n# See: https://github.com/openkraft/openkraft\n\nproject_type: %s\n\n%s\n", pt, weightsSection)

	if skipSection != "" {
		result += skipSection + "\n"
	}

	result += `# exclude_paths:
#   - generated
#   - third_party

# min_thresholds:
#   tests: 60
#   architecture: 50
`

	return result
}
