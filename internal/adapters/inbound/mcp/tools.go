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

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/application"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/golden"
	"github.com/openkraft/openkraft/internal/domain/scoring"
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
}

// newServices creates the standard set of outbound adapters and services.
func newServices() (*application.ScoreService, *application.CheckService) {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	return application.NewScoreService(sc, det, par),
		application.NewCheckService(sc, det, par)
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
		conventions := scoring.ScoreConventions(scan, analyzed)
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
