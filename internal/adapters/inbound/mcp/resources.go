package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/golden"
	"github.com/openkraft/openkraft/internal/domain/scoring"
)

// registerResources registers all OpenKraft MCP resources on the given server.
func registerResources(s *server.MCPServer, projectPath string) {
	// 1. openkraft://score - current project score
	s.AddResource(
		mcplib.NewResource(
			"openkraft://score",
			"Project Score",
			mcplib.WithResourceDescription("Current AI-readiness score for the project"),
			mcplib.WithMIMEType("application/json"),
		),
		handleScoreResource(projectPath),
	)

	// 2. openkraft://blueprint - extracted blueprint
	s.AddResource(
		mcplib.NewResource(
			"openkraft://blueprint",
			"Blueprint",
			mcplib.WithResourceDescription("Structural blueprint extracted from the golden module"),
			mcplib.WithMIMEType("application/json"),
		),
		handleBlueprintResource(projectPath),
	)

	// 3. openkraft://conventions - project conventions
	s.AddResource(
		mcplib.NewResource(
			"openkraft://conventions",
			"Conventions",
			mcplib.WithResourceDescription("Detected coding conventions for the project"),
			mcplib.WithMIMEType("application/json"),
		),
		handleConventionsResource(projectPath),
	)

	// 4. openkraft://modules/{name} - per-module completeness (resource template)
	s.AddResourceTemplate(
		mcplib.NewResourceTemplate(
			"openkraft://modules/{name}",
			"Module Check",
			mcplib.WithTemplateDescription("Completeness report for a specific module"),
			mcplib.WithTemplateMIMEType("application/json"),
		),
		handleModuleResource(projectPath),
	)
}

func handleScoreResource(projectPath string) server.ResourceHandlerFunc {
	return func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		scoreSvc, _ := newServices()
		score, err := scoreSvc.ScoreProject(projectPath)
		if err != nil {
			return nil, fmt.Errorf("scoring failed: %w", err)
		}

		data, err := json.MarshalIndent(score, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling score: %w", err)
		}

		return []mcplib.ResourceContents{
			mcplib.TextResourceContents{
				URI:      "openkraft://score",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
}

func handleBlueprintResource(projectPath string) server.ResourceHandlerFunc {
	return func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		sc := scanner.New()
		det := detector.New()
		par := parser.New()

		scan, err := sc.Scan(projectPath)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		modules, err := det.Detect(scan)
		if err != nil {
			return nil, fmt.Errorf("detect failed: %w", err)
		}

		analyzed := analyzeFiles(scan, par)

		goldenMod, err := golden.SelectGolden(modules, analyzed)
		if err != nil {
			return nil, fmt.Errorf("golden selection failed: %w", err)
		}

		blueprint, err := golden.ExtractBlueprint(goldenMod.Module, analyzed)
		if err != nil {
			return nil, fmt.Errorf("blueprint extraction failed: %w", err)
		}

		data, err := json.MarshalIndent(blueprint, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling blueprint: %w", err)
		}

		return []mcplib.ResourceContents{
			mcplib.TextResourceContents{
				URI:      "openkraft://blueprint",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
}

func handleConventionsResource(projectPath string) server.ResourceHandlerFunc {
	return func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		sc := scanner.New()
		par := parser.New()

		scan, err := sc.Scan(projectPath)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		analyzed := analyzeFiles(scan, par)
		p := domain.DefaultProfile()
		conventions := scoring.ScoreDiscoverability(&p, nil, scan, analyzed)

		data, err := json.MarshalIndent(conventions, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling conventions: %w", err)
		}

		return []mcplib.ResourceContents{
			mcplib.TextResourceContents{
				URI:      "openkraft://conventions",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
}

func handleModuleResource(projectPath string) server.ResourceTemplateHandlerFunc {
	return func(_ context.Context, request mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		// Extract module name from the arguments (populated by template matching)
		moduleName, ok := request.Params.Arguments["name"].(string)
		if !ok || moduleName == "" {
			return nil, fmt.Errorf("module name is required")
		}

		_, checkSvc := newServices()
		report, err := checkSvc.CheckModule(projectPath, moduleName)
		if err != nil {
			return nil, fmt.Errorf("check failed: %w", err)
		}

		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling report: %w", err)
		}

		return []mcplib.ResourceContents{
			mcplib.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	}
}
