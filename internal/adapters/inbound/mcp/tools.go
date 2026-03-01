package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	cacheAdapter "github.com/abdidvp/openkraft/internal/adapters/outbound/cache"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/config"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/detector"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/parser"
	"github.com/abdidvp/openkraft/internal/adapters/outbound/scanner"
	"github.com/abdidvp/openkraft/internal/application"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/golden"
	"github.com/abdidvp/openkraft/internal/domain/scoring"
)

// registerTools registers all OpenKraft MCP tools on the given server.
func registerTools(s *server.MCPServer, projectPath string) {
	// 1. openkraft_score
	s.AddTool(
		mcplib.NewTool("openkraft_score",
			mcplib.WithDescription("Returns the full AI-readiness score for the project as JSON"),
		),
		handleScore(projectPath),
	)

	// 2. openkraft_check_module
	s.AddTool(
		mcplib.NewTool("openkraft_check_module",
			mcplib.WithDescription("Check a single module against the golden module's blueprint"),
			mcplib.WithString("module",
				mcplib.Required(),
				mcplib.Description("Name of the module to check"),
			),
		),
		handleCheckModule(projectPath),
	)

	// 3. openkraft_get_blueprint
	s.AddTool(
		mcplib.NewTool("openkraft_get_blueprint",
			mcplib.WithDescription("Returns the structural blueprint extracted from the golden module"),
		),
		handleGetBlueprint(projectPath),
	)

	// 4. openkraft_get_golden_example
	s.AddTool(
		mcplib.NewTool("openkraft_get_golden_example",
			mcplib.WithDescription("Returns the source code of a file from the golden module by file type"),
			mcplib.WithString("file_type",
				mcplib.Required(),
				mcplib.Description("Type of file to retrieve (e.g. domain_entity, service, handler, repository, ports, domain_errors, domain_test)"),
			),
		),
		handleGetGoldenExample(projectPath),
	)

	// 5. openkraft_get_conventions
	s.AddTool(
		mcplib.NewTool("openkraft_get_conventions",
			mcplib.WithDescription("Returns the detected coding conventions for the project"),
		),
		handleGetConventions(projectPath),
	)

	// 6. openkraft_check_file
	s.AddTool(
		mcplib.NewTool("openkraft_check_file",
			mcplib.WithDescription("Returns issues found for a single file in the project"),
			mcplib.WithString("file",
				mcplib.Required(),
				mcplib.Description("Relative path to the file to check"),
			),
		),
		handleCheckFile(projectPath),
	)

	// 7. openkraft_onboard
	s.AddTool(
		mcplib.NewTool("openkraft_onboard",
			mcplib.WithDescription("Generate a consistency contract (CLAUDE.md) from codebase analysis. Returns project conventions, golden module, dependency rules, and norms."),
			mcplib.WithString("format", mcplib.Description("Output format: md or json (default: json)")),
		),
		handleOnboard(projectPath),
	)

	// 8. openkraft_fix
	s.AddTool(
		mcplib.NewTool("openkraft_fix",
			mcplib.WithDescription("Detect drift from project patterns and return fix plan with safe auto-fixes and structured instructions"),
			mcplib.WithBoolean("dry_run", mcplib.Description("Show plan without applying fixes")),
			mcplib.WithString("category", mcplib.Description("Fix only a specific category")),
		),
		handleFix(projectPath),
	)

	// 9. openkraft_validate
	s.AddTool(
		mcplib.NewTool("openkraft_validate",
			mcplib.WithDescription("Incremental drift detection for changed files. Returns drift issues and score impact."),
			mcplib.WithString("changed", mcplib.Required(), mcplib.Description("Comma-separated changed file paths relative to project root")),
			mcplib.WithString("added", mcplib.Description("Comma-separated added file paths")),
			mcplib.WithString("deleted", mcplib.Description("Comma-separated deleted file paths")),
			mcplib.WithBoolean("strict", mcplib.Description("Fail on warnings")),
		),
		handleValidate(projectPath),
	)
}

// newServices creates the standard set of outbound adapters and services.
func newServices() (*application.ScoreService, *application.CheckService) {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	cfg := config.New()
	return application.NewScoreService(sc, det, par, cfg),
		application.NewCheckService(sc, det, par, cfg)
}

func handleScore(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		scoreSvc, _ := newServices()
		score, err := scoreSvc.ScoreProject(projectPath)
		if err != nil {
			return errorResult(fmt.Sprintf("scoring failed: %v", err)), nil
		}
		return jsonResult(score)
	}
}

func handleCheckModule(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		moduleName, err := request.RequireString("module")
		if err != nil {
			return errorResult(err.Error()), nil
		}

		_, checkSvc := newServices()
		report, err := checkSvc.CheckModule(projectPath, moduleName)
		if err != nil {
			return errorResult(fmt.Sprintf("check failed: %v", err)), nil
		}
		return jsonResult(report)
	}
}

func handleGetBlueprint(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		sc := scanner.New()
		det := detector.New()
		par := parser.New()

		scan, err := sc.Scan(projectPath)
		if err != nil {
			return errorResult(fmt.Sprintf("scan failed: %v", err)), nil
		}

		modules, err := det.Detect(scan)
		if err != nil {
			return errorResult(fmt.Sprintf("detect failed: %v", err)), nil
		}

		analyzed := analyzeFiles(scan, par)

		goldenMod, err := golden.SelectGolden(modules, analyzed)
		if err != nil {
			return errorResult(fmt.Sprintf("golden selection failed: %v", err)), nil
		}

		blueprint, err := golden.ExtractBlueprint(goldenMod.Module, analyzed)
		if err != nil {
			return errorResult(fmt.Sprintf("blueprint extraction failed: %v", err)), nil
		}

		return jsonResult(blueprint)
	}
}

func handleGetGoldenExample(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		fileType, err := request.RequireString("file_type")
		if err != nil {
			return errorResult(err.Error()), nil
		}

		sc := scanner.New()
		det := detector.New()
		par := parser.New()

		scan, err := sc.Scan(projectPath)
		if err != nil {
			return errorResult(fmt.Sprintf("scan failed: %v", err)), nil
		}

		modules, err := det.Detect(scan)
		if err != nil {
			return errorResult(fmt.Sprintf("detect failed: %v", err)), nil
		}

		analyzed := analyzeFiles(scan, par)

		goldenMod, err := golden.SelectGolden(modules, analyzed)
		if err != nil {
			return errorResult(fmt.Sprintf("golden selection failed: %v", err)), nil
		}

		blueprint, err := golden.ExtractBlueprint(goldenMod.Module, analyzed)
		if err != nil {
			return errorResult(fmt.Sprintf("blueprint extraction failed: %v", err)), nil
		}

		// Find the file of the requested type in the golden module
		for i, bpFile := range blueprint.Files {
			if bpFile.Type == fileType {
				// The golden module files correspond 1:1 with blueprint files
				if i < len(goldenMod.Module.Files) {
					filePath := goldenMod.Module.Files[i]
					absPath := filepath.Join(scan.RootPath, filePath)
					content, readErr := os.ReadFile(absPath)
					if readErr != nil {
						return errorResult(fmt.Sprintf("reading file failed: %v", readErr)), nil
					}
					return textResult(string(content)), nil
				}
			}
		}

		return errorResult(fmt.Sprintf("no file of type %q found in golden module", fileType)), nil
	}
}

func handleGetConventions(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		sc := scanner.New()
		par := parser.New()

		scan, err := sc.Scan(projectPath)
		if err != nil {
			return errorResult(fmt.Sprintf("scan failed: %v", err)), nil
		}

		analyzed := analyzeFiles(scan, par)
		p := domain.DefaultProfile()
		conventions := scoring.ScoreDiscoverability(&p, nil, scan, analyzed)
		return jsonResult(conventions)
	}
}

func handleCheckFile(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		file, err := request.RequireString("file")
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Score the project and find issues for this file
		scoreSvc, _ := newServices()
		score, err := scoreSvc.ScoreProject(projectPath)
		if err != nil {
			return errorResult(fmt.Sprintf("scoring failed: %v", err)), nil
		}

		// Filter issues for the requested file
		type fileIssues struct {
			File   string         `json:"file"`
			Issues []domain.Issue `json:"issues"`
		}

		result := fileIssues{File: file}
		for _, cat := range score.Categories {
			for _, issue := range cat.Issues {
				if issue.File == file || strings.HasSuffix(issue.File, "/"+file) {
					result.Issues = append(result.Issues, issue)
				}
			}
		}

		return jsonResult(result)
	}
}

func handleOnboard(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		sc := scanner.New()
		det := detector.New()
		par := parser.New()
		cfg := config.New()
		svc := application.NewOnboardService(sc, det, par, cfg)

		report, err := svc.GenerateReport(projectPath)
		if err != nil {
			return errorResult(fmt.Sprintf("onboard failed: %v", err)), nil
		}

		format, _ := request.GetArguments()["format"].(string)
		if format == "md" {
			return textResult(svc.RenderContract(report)), nil
		}
		return jsonResult(report)
	}
}

func handleFix(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		sc := scanner.New()
		det := detector.New()
		par := parser.New()
		cfg := config.New()

		scoreSvc := application.NewScoreService(sc, det, par, cfg)
		onboardSvc := application.NewOnboardService(sc, det, par, cfg)
		fixSvc := application.NewFixService(scoreSvc, onboardSvc)

		dryRun, _ := request.GetArguments()["dry_run"].(bool)
		category, _ := request.GetArguments()["category"].(string)

		opts := domain.FixOptions{
			DryRun:   dryRun,
			AutoOnly: false,
			Category: category,
		}

		plan, err := fixSvc.PlanFixes(projectPath, opts)
		if err != nil {
			return errorResult(fmt.Sprintf("fix failed: %v", err)), nil
		}
		return jsonResult(plan)
	}
}

func handleValidate(projectPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		changedStr, err := request.RequireString("changed")
		if err != nil {
			return errorResult(err.Error()), nil
		}

		sc := scanner.New()
		det := detector.New()
		par := parser.New()
		cfg := config.New()

		cacheSt := cacheAdapter.New()
		scoreSvc := application.NewScoreService(sc, det, par, cfg)
		validateSvc := application.NewValidateService(sc, det, par, scoreSvc, cacheSt, cfg)

		changed := splitAndTrim(changedStr)

		var added, deleted []string
		args := request.GetArguments()
		if addedStr, ok := args["added"].(string); ok && addedStr != "" {
			added = splitAndTrim(addedStr)
		}
		if deletedStr, ok := args["deleted"].(string); ok && deletedStr != "" {
			deleted = splitAndTrim(deletedStr)
		}
		strict, _ := args["strict"].(bool)

		result, err := validateSvc.Validate(projectPath, changed, added, deleted, strict)
		if err != nil {
			return errorResult(fmt.Sprintf("validate failed: %v", err)), nil
		}
		return jsonResult(result)
	}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// analyzeFiles runs the code analyzer on all Go files in the scan result.
func analyzeFiles(scan *domain.ScanResult, analyzer domain.CodeAnalyzer) map[string]*domain.AnalyzedFile {
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		absPath := filepath.Join(scan.RootPath, f)
		af, err := analyzer.AnalyzeFile(absPath)
		if err != nil {
			continue
		}
		analyzed[f] = af
	}
	return analyzed
}

// jsonResult marshals v to JSON and returns it as a text content result.
func jsonResult(v interface{}) (*mcplib.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}
	return &mcplib.CallToolResult{
		Content: []mcplib.Content{mcplib.NewTextContent(string(data))},
	}, nil
}

// textResult returns a plain text content result.
func textResult(text string) *mcplib.CallToolResult {
	return &mcplib.CallToolResult{
		Content: []mcplib.Content{mcplib.NewTextContent(text)},
	}
}

// errorResult returns a tool result that indicates an error occurred.
func errorResult(msg string) *mcplib.CallToolResult {
	return &mcplib.CallToolResult{
		Content: []mcplib.Content{mcplib.NewTextContent(msg)},
		IsError: true,
	}
}
