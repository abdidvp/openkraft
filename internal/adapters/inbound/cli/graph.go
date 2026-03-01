package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/adapters/outbound/tui"
	"github.com/openkraft/openkraft/internal/application"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/spf13/cobra"
)

func newGraphCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "graph [path]",
		Short: "Visualize the import dependency graph",
		Long:  "Analyze a Go project's internal import structure and display package metrics, cycles, and coupling outliers.",
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

			data, err := svc.AnalyzeProject(absPath)
			if err != nil {
				return fmt.Errorf("analysis failed: %w", err)
			}

			graph := scoring.BuildImportGraph(data.Scan.ModulePath, data.Analyzed)

			if jsonOutput {
				return renderGraphJSON(cmd, graph, data)
			}

			fmt.Fprint(cmd.OutOrStdout(), tui.RenderGraph(graph, data.Scan.ModulePath, &data.Profile))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output graph metrics as JSON")
	return cmd
}

type graphJSONOutput struct {
	ModulePath string        `json:"module_path"`
	Packages   int           `json:"packages"`
	Edges      int           `json:"edges"`
	Violations int           `json:"violations"`
	Cycles     [][]string    `json:"cycles"`
	Outliers   []outlierJSON `json:"coupling_outliers"`
	Metrics    []packageJSON `json:"package_metrics"`
}

type outlierJSON struct {
	Package  string  `json:"package"`
	Ce       int     `json:"ce"`
	MedianCe float64 `json:"median_ce"`
}

type packageJSON struct {
	Package    string   `json:"package"`
	Ca         int      `json:"ca"`
	Ce         int      `json:"ce"`
	Role       string   `json:"role"`
	Violations []string `json:"violations"`
}

func renderGraphJSON(cmd *cobra.Command, graph *scoring.ImportGraph, data *application.ProjectData) error {
	out := graphJSONOutput{
		ModulePath: data.Scan.ModulePath,
	}

	if graph == nil {
		out.Cycles = [][]string{}
		out.Outliers = []outlierJSON{}
		out.Metrics = []packageJSON{}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	annotated := graph.ClassifyPackages(data.Scan.ModulePath, &data.Profile)

	out.Packages = len(graph.Packages)
	out.Edges = graph.EdgeCount()
	out.Violations = scoring.TotalViolations(annotated)

	cycles := graph.DetectCycles()
	if cycles != nil {
		out.Cycles = cycles
	} else {
		out.Cycles = [][]string{}
	}

	multiplier := 2.0
	if data.Profile.CouplingOutlierMultiplier > 0 {
		multiplier = data.Profile.CouplingOutlierMultiplier
	}
	outliers := graph.CouplingOutliers(multiplier)
	out.Outliers = make([]outlierJSON, len(outliers))
	for i, o := range outliers {
		out.Outliers[i] = outlierJSON{Package: o.Package, Ce: o.Ce, MedianCe: o.MedianCe}
	}

	// Sort by package path for deterministic output.
	pkgs := make([]string, 0, len(annotated))
	for pkg := range annotated {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	out.Metrics = make([]packageJSON, 0, len(pkgs))
	for _, pkg := range pkgs {
		ap := annotated[pkg]
		var viols []string
		for _, v := range ap.Violations {
			viols = append(viols, v.Message)
		}
		if viols == nil {
			viols = []string{}
		}
		out.Metrics = append(out.Metrics, packageJSON{
			Package:    pkg,
			Ca:         len(ap.Node.ImportedBy),
			Ce:         len(ap.Node.ImportsInternal),
			Role:       string(ap.Role),
			Violations: viols,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
