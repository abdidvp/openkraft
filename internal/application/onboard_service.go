package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/golden"
)

// OnboardService orchestrates the onboard pipeline:
// scan → detect modules → analyze AST → select golden → extract blueprint → build report.
type OnboardService struct {
	scanner      domain.ProjectScanner
	detector     domain.ModuleDetector
	analyzer     domain.CodeAnalyzer
	configLoader domain.ConfigLoader
}

func NewOnboardService(
	scanner domain.ProjectScanner,
	detector domain.ModuleDetector,
	analyzer domain.CodeAnalyzer,
	configLoader domain.ConfigLoader,
) *OnboardService {
	return &OnboardService{
		scanner:      scanner,
		detector:     detector,
		analyzer:     analyzer,
		configLoader: configLoader,
	}
}

func (s *OnboardService) GenerateReport(projectPath string) (*domain.OnboardReport, error) {
	// 1. Load config
	cfg, err := s.configLoader.Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// 2. Scan
	scan, err := s.scanner.Scan(projectPath, cfg.ExcludePaths...)
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}

	// 3. Detect modules
	modules, err := s.detector.Detect(scan)
	if err != nil {
		return nil, fmt.Errorf("detecting modules: %w", err)
	}

	// 4. Analyze files (same pattern as score_service.go lines 56-65)
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		absPath := filepath.Join(scan.RootPath, f)
		af, err := s.analyzer.AnalyzeFile(absPath)
		if err != nil {
			continue
		}
		af.Path = f
		analyzed[f] = af
	}

	// 5. Select golden module
	var goldenModulePath string
	var moduleBlueprint []string
	goldenMod, err := golden.SelectGolden(modules, analyzed)
	if err == nil && goldenMod != nil {
		goldenModulePath = goldenMod.Module.Path
		// 6. Extract blueprint
		bp, bpErr := golden.ExtractBlueprint(goldenMod.Module, analyzed)
		if bpErr == nil && bp != nil {
			moduleBlueprint = bp.Patterns
		}
	}

	// 7. Detect naming convention
	namingConvention, namingPct := detectNamingConvention(scan)

	// 8. Compute norms
	norms := domain.ComputeNorms(analyzed)
	norms.NamingStyle = namingConvention
	norms.NamingPct = namingPct

	// 9. Detect build commands
	buildCmds, testCmds := detectBuildCommands(scan)

	// 10. Detect dependency rules
	depRules := detectDependencyRules(modules)

	// 11. Find interface mappings
	interfaces := detectInterfaces(analyzed)

	// 12. Detect architecture style
	archStyle := detectArchitectureStyle(modules)

	// Determine project name from directory name
	projectName := filepath.Base(projectPath)

	// Determine project type from config
	projectType := string(cfg.ProjectType)
	if projectType == "" {
		projectType = "go"
	}

	return &domain.OnboardReport{
		ProjectName:       projectName,
		ProjectType:       projectType,
		ArchitectureStyle: archStyle,
		LayoutStyle:       scan.Layout,
		Modules:           modules,
		NamingConvention:  namingConvention,
		NamingPercentage:  namingPct,
		GoldenModule:      goldenModulePath,
		ModuleBlueprint:   moduleBlueprint,
		BuildCommands:     buildCmds,
		TestCommands:      testCmds,
		DependencyRules:   depRules,
		Interfaces:        interfaces,
		Norms:             norms,
	}, nil
}

// RenderContract renders the onboard report as a prescriptive Markdown contract.
func (s *OnboardService) RenderContract(report *domain.OnboardReport) string {
	funcMap := template.FuncMap{
		"mul":        func(a float64, b float64) float64 { return a * b },
		"joinLayers": func(layers []string) string { return strings.Join(layers, ", ") },
	}
	tmpl := template.Must(template.New("contract").Funcs(funcMap).Parse(contractTemplate))
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, report)
	return buf.String()
}

// RenderJSON renders the onboard report as indented JSON.
func (s *OnboardService) RenderJSON(report *domain.OnboardReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// detectNamingConvention classifies file naming as "bare" or "suffixed"
// based on whether the majority of non-test Go files use underscores.
func detectNamingConvention(scan *domain.ScanResult) (convention string, pct float64) {
	bare, suffixed, total := 0, 0, 0
	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if name == "main" || name == "doc" {
			continue
		}
		total++
		if strings.Contains(name, "_") {
			suffixed++
		} else {
			bare++
		}
	}
	if total == 0 {
		return "bare", 0
	}
	if bare >= suffixed {
		return "bare", float64(bare) / float64(total)
	}
	return "suffixed", float64(suffixed) / float64(total)
}

// detectBuildCommands checks AllFiles for build tool markers and returns
// discovered build and test commands.
func detectBuildCommands(scan *domain.ScanResult) (build, test []string) {
	for _, f := range scan.AllFiles {
		base := filepath.Base(f)
		switch strings.ToLower(base) {
		case "makefile":
			build = append(build, "make build")
			test = append(test, "make test")
		case "taskfile.yml", "taskfile.yaml":
			build = append(build, "task build")
			test = append(test, "task test")
		case "justfile":
			build = append(build, "just build")
			test = append(test, "just test")
		}
	}
	if scan.HasGoMod {
		build = append(build, "go build ./...")
		test = append(test, "go test ./...")
	}
	return build, test
}

// detectDependencyRules infers import restrictions based on detected layers.
func detectDependencyRules(modules []domain.DetectedModule) []domain.DependencyRule {
	hasDomain, hasApplication, hasAdapters := false, false, false
	for _, m := range modules {
		for _, l := range m.Layers {
			switch l {
			case "domain":
				hasDomain = true
			case "application":
				hasApplication = true
			case "adapters":
				hasAdapters = true
			}
		}
	}

	var rules []domain.DependencyRule
	if hasDomain && hasApplication {
		rules = append(rules, domain.DependencyRule{
			Source:  "domain",
			Forbids: "application",
			Reason:  "domain layer must not depend on application layer",
		})
	}
	if hasDomain && hasAdapters {
		rules = append(rules, domain.DependencyRule{
			Source:  "domain",
			Forbids: "adapters",
			Reason:  "domain layer must not depend on adapter layer",
		})
	}
	if hasApplication && hasAdapters {
		rules = append(rules, domain.DependencyRule{
			Source:  "application",
			Forbids: "adapters",
			Reason:  "application layer must not depend on adapter layer",
		})
	}
	return rules
}

// detectInterfaces matches interface definitions to struct implementations
// across packages.
func detectInterfaces(analyzed map[string]*domain.AnalyzedFile) []domain.InterfaceMapping {
	type ifaceInfo struct {
		name    string
		pkg     string
		methods []string
	}
	var ifaces []ifaceInfo
	for _, af := range analyzed {
		for _, idef := range af.InterfaceDefs {
			ifaces = append(ifaces, ifaceInfo{
				name:    idef.Name,
				pkg:     af.Package,
				methods: idef.Methods,
			})
		}
	}

	var mappings []domain.InterfaceMapping
	for _, iface := range ifaces {
		if len(iface.methods) == 0 {
			continue
		}
		for _, af := range analyzed {
			// Build method set for each struct in this file
			structMethods := make(map[string]map[string]bool)
			for _, fn := range af.Functions {
				if fn.Receiver != "" {
					recv := strings.TrimPrefix(fn.Receiver, "*")
					if structMethods[recv] == nil {
						structMethods[recv] = make(map[string]bool)
					}
					structMethods[recv][fn.Name] = true
				}
			}
			// Check if any struct implements all interface methods
			for structName, methods := range structMethods {
				allMatch := true
				for _, m := range iface.methods {
					if !methods[m] {
						allMatch = false
						break
					}
				}
				if allMatch && af.Package != iface.pkg {
					mappings = append(mappings, domain.InterfaceMapping{
						Interface:      iface.name,
						Implementation: structName,
						Package:        af.Package,
					})
				}
			}
		}
	}
	return mappings
}

// detectArchitectureStyle determines hexagonal vs layered vs flat
// based on the layers present across modules.
func detectArchitectureStyle(modules []domain.DetectedModule) string {
	if len(modules) == 0 {
		return "flat"
	}
	hasDomain, hasApplication, hasAdapters := false, false, false
	for _, m := range modules {
		for _, l := range m.Layers {
			switch l {
			case "domain":
				hasDomain = true
			case "application":
				hasApplication = true
			case "adapters":
				hasAdapters = true
			}
		}
	}
	if hasDomain && hasApplication && hasAdapters {
		return "hexagonal"
	}
	if hasDomain || hasApplication {
		return "layered"
	}
	return "flat"
}

const contractTemplate = `# Project: {{.ProjectName}}

{{.ProjectType}} using {{.ArchitectureStyle}} architecture with {{.LayoutStyle}} layout.
{{- if .GoldenModule}}
Golden module: ` + "`" + `{{.GoldenModule}}` + "`" + ` -- all new modules MUST follow this structure.
{{- end}}

## Naming Rules

- File naming: ALWAYS use {{.NamingConvention}} naming ({{printf "%.0f" (mul .NamingPercentage 100)}}% of files follow this convention)
- Functions MUST be under {{.Norms.FunctionLines}} lines (project p90)
- Functions MUST have at most {{.Norms.Parameters}} parameters (project p90)
- Files MUST be under {{.Norms.FileLines}} lines (project p90)
{{- if .ModuleBlueprint}}

## Module Blueprint

Every module MUST contain these layers:
{{- range .ModuleBlueprint}}
- {{.}}
{{- end}}
{{- end}}
{{- if .Modules}}

## Modules

| Module | Path | Layers |
|--------|------|--------|
{{- range .Modules}}
| {{.Name}} | ` + "`" + `{{.Path}}` + "`" + ` | {{joinLayers .Layers}} |
{{- end}}
{{- end}}
{{- if .DependencyRules}}

## Dependency Rules

{{- range .DependencyRules}}
- {{.Source}}/ MUST NOT import from {{.Forbids}}/ -- {{.Reason}}
{{- end}}
{{- end}}
{{- if .Interfaces}}

## Key Interfaces

| Port | Implementation | Package |
|------|---------------|---------|
{{- range .Interfaces}}
| {{.Interface}} | {{.Implementation}} | {{.Package}} |
{{- end}}
{{- end}}
{{- if or .BuildCommands .TestCommands}}

## Build & Test

{{- if .BuildCommands}}
Build:
{{- range .BuildCommands}}
- ` + "`" + `{{.}}` + "`" + `
{{- end}}
{{- end}}
{{- if .TestCommands}}
Test:
{{- range .TestCommands}}
- ` + "`" + `{{.}}` + "`" + `
{{- end}}
{{- end}}
{{- end}}
`
