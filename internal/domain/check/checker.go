package check

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/abdidvp/openkraft/internal/domain"
)

const (
	penaltyMissingFile      = 10
	penaltyMissingStruct    = 5
	penaltyMissingMethod    = 5
	penaltyMissingInterface = 5
	penaltyPatternViolation = 3
)

// CheckModule compares a target module against a blueprint extracted from a
// golden module and returns a completeness report with score.
func CheckModule(
	target domain.DetectedModule,
	blueprint *domain.Blueprint,
	analyzed map[string]*domain.AnalyzedFile,
) *domain.CheckReport {
	report := &domain.CheckReport{
		Module:       target.Name,
		GoldenModule: blueprint.Name,
		Score:        100,
	}

	entityName := detectEntityName(target, analyzed)

	// Build a set of target files keyed by their path relative to the module.
	targetRelFiles := make(map[string]string) // relPath -> original key in analyzed
	modulePrefix := filepath.ToSlash(target.Path) + "/"
	for _, f := range target.Files {
		norm := filepath.ToSlash(f)
		if strings.HasPrefix(norm, modulePrefix) {
			rel := norm[len(modulePrefix):]
			targetRelFiles[rel] = f
		}
	}

	// 1. File manifest comparison
	for _, bpFile := range blueprint.Files {
		candidates := resolvePaths(bpFile.PathPattern, entityName, target.Name)

		var matchedKey string
		var matchedPath string
		for _, candidate := range candidates {
			if key, ok := targetRelFiles[candidate]; ok {
				matchedKey = key
				matchedPath = candidate
				break
			}
		}

		if matchedKey == "" {
			displayPath := candidates[0]
			report.MissingFiles = append(report.MissingFiles, domain.MissingItem{
				Name:        displayPath,
				Expected:    bpFile.Type,
				Description: fmt.Sprintf("expected file %s (%s)", displayPath, bpFile.Type),
			})
			report.Issues = append(report.Issues, domain.Issue{
				Severity: domain.SeverityError,
				Category: "completeness",
				Message:  fmt.Sprintf("missing %s file: %s", bpFile.Type, displayPath),
			})

			// If the file is missing, all its required elements are also missing.
			addMissingStructural(report, bpFile, displayPath, entityName, target.Name)
			continue
		}

		// 2. Structural comparison for files that exist.
		af := analyzed[matchedKey]
		if af == nil {
			continue
		}
		checkStructural(report, bpFile, af, matchedPath, entityName, target.Name)
	}

	// 3. Pattern compliance checks.
	checkPatterns(report, target, analyzed)

	// 4. Compute score.
	deductions := len(report.MissingFiles)*penaltyMissingFile +
		len(report.MissingStructs)*penaltyMissingStruct +
		len(report.MissingMethods)*penaltyMissingMethod +
		len(report.MissingInterfaces)*penaltyMissingInterface +
		len(report.PatternViolations)*penaltyPatternViolation

	report.Score = 100 - deductions
	if report.Score < 0 {
		report.Score = 0
	}

	return report
}

// resolvePaths returns all possible concrete file paths for a blueprint pattern.
// {entity} may map to the entity snake_case or module name, and {module} may
// map to the module name or entity snake_case, so we try all combinations.
func resolvePaths(pattern, entityName, moduleName string) []string {
	entitySnake := toSnakeCase(entityName)
	moduleSnake := strings.ToLower(moduleName)

	paths := make(map[string]bool)

	// Primary: {entity} -> entity snake, {module} -> module snake
	p := pattern
	p = strings.ReplaceAll(p, "{entity}", entitySnake)
	p = strings.ReplaceAll(p, "{module}", moduleSnake)
	paths[p] = true

	// Also try: {module} -> entity snake (for cases like product_errors.go)
	if entitySnake != moduleSnake {
		p2 := pattern
		p2 = strings.ReplaceAll(p2, "{entity}", entitySnake)
		p2 = strings.ReplaceAll(p2, "{module}", entitySnake)
		paths[p2] = true
	}

	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	return result
}

// resolveNames returns all possible concrete names for a blueprint pattern.
// {Entity} may stand for the entity PascalCase name (e.g. "TaxRule") or
// the module PascalCase name (e.g. "Tax"), so we return both variants.
func resolveNames(pattern, entityName, moduleName string) []string {
	if !strings.Contains(pattern, "{Entity}") {
		return []string{pattern}
	}

	pascalEntity := entityName
	pascalModule := toPascalCase(moduleName)

	names := make(map[string]bool)
	names[strings.ReplaceAll(pattern, "{Entity}", pascalEntity)] = true
	if pascalModule != "" && pascalModule != pascalEntity {
		names[strings.ReplaceAll(pattern, "{Entity}", pascalModule)] = true
	}

	result := make([]string, 0, len(names))
	for n := range names {
		result = append(result, n)
	}
	return result
}

// anyInSet returns true if any of the names is in the set.
func anyInSet(names []string, set map[string]bool) bool {
	for _, n := range names {
		if set[n] {
			return true
		}
	}
	return false
}

// addMissingStructural adds missing structs/methods/interfaces for a file
// that does not exist at all.
func addMissingStructural(report *domain.CheckReport, bpFile domain.BlueprintFile, filePath, entityName, moduleName string) {
	for _, s := range bpFile.RequiredStructs {
		names := resolveNames(s, entityName, moduleName)
		report.MissingStructs = append(report.MissingStructs, domain.MissingItem{
			Name:     names[0],
			Expected: s,
			File:     filePath,
		})
	}
	for _, m := range bpFile.RequiredMethods {
		names := resolveNames(m, entityName, moduleName)
		report.MissingMethods = append(report.MissingMethods, domain.MissingItem{
			Name:     names[0],
			Expected: m,
			File:     filePath,
		})
	}
	for _, fn := range bpFile.RequiredFunctions {
		names := resolveNames(fn, entityName, moduleName)
		report.MissingMethods = append(report.MissingMethods, domain.MissingItem{
			Name:     names[0],
			Expected: fn,
			File:     filePath,
		})
	}
	for _, i := range bpFile.RequiredInterfaces {
		names := resolveNames(i, entityName, moduleName)
		report.MissingInterfaces = append(report.MissingInterfaces, domain.MissingItem{
			Name:     names[0],
			Expected: i,
			File:     filePath,
		})
	}
}

// checkStructural compares the analyzed file against blueprint requirements.
func checkStructural(report *domain.CheckReport, bpFile domain.BlueprintFile, af *domain.AnalyzedFile, filePath, entityName, moduleName string) {
	structSet := toSet(af.Structs)
	ifaceSet := toSet(af.Interfaces)

	methodSet := make(map[string]bool)
	funcSet := make(map[string]bool)
	for _, fn := range af.Functions {
		if fn.Receiver != "" {
			methodSet[fn.Name] = true
		} else {
			funcSet[fn.Name] = true
		}
	}

	for _, s := range bpFile.RequiredStructs {
		names := resolveNames(s, entityName, moduleName)
		if !anyInSet(names, structSet) {
			report.MissingStructs = append(report.MissingStructs, domain.MissingItem{
				Name:     names[0],
				Expected: s,
				File:     filePath,
			})
			report.Issues = append(report.Issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "completeness",
				File:     af.Path,
				Message:  fmt.Sprintf("missing struct %s (expected from blueprint pattern %s)", names[0], s),
			})
		}
	}

	// Check required functions (free functions like constructors).
	for _, f := range bpFile.RequiredFunctions {
		names := resolveNames(f, entityName, moduleName)
		if !anyInSet(names, funcSet) {
			report.MissingMethods = append(report.MissingMethods, domain.MissingItem{
				Name:     names[0],
				Expected: f,
				File:     filePath,
			})
			report.Issues = append(report.Issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "completeness",
				File:     af.Path,
				Message:  fmt.Sprintf("missing function %s (expected from blueprint pattern %s)", names[0], f),
			})
		}
	}

	for _, m := range bpFile.RequiredMethods {
		names := resolveNames(m, entityName, moduleName)
		if !anyInSet(names, methodSet) {
			report.MissingMethods = append(report.MissingMethods, domain.MissingItem{
				Name:     names[0],
				Expected: m,
				File:     filePath,
			})
			report.Issues = append(report.Issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "completeness",
				File:     af.Path,
				Message:  fmt.Sprintf("missing method %s (expected from blueprint pattern %s)", names[0], m),
			})
		}
	}

	for _, i := range bpFile.RequiredInterfaces {
		names := resolveNames(i, entityName, moduleName)
		if !anyInSet(names, ifaceSet) {
			report.MissingInterfaces = append(report.MissingInterfaces, domain.MissingItem{
				Name:     names[0],
				Expected: i,
				File:     filePath,
			})
			report.Issues = append(report.Issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "completeness",
				File:     af.Path,
				Message:  fmt.Sprintf("missing interface %s (expected from blueprint pattern %s)", names[0], i),
			})
		}
	}
}

// checkPatterns verifies auto-detected patterns: Validate method, constructors,
// interfaces.
func checkPatterns(report *domain.CheckReport, target domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) {
	hasValidate := false
	hasConstructor := false
	hasInterface := false

	for _, f := range target.Files {
		af, ok := analyzed[f]
		if !ok {
			continue
		}
		for _, fn := range af.Functions {
			if fn.Name == "Validate" && fn.Receiver != "" {
				hasValidate = true
			}
			if strings.HasPrefix(fn.Name, "New") && fn.Receiver == "" {
				hasConstructor = true
			}
		}
		if len(af.Interfaces) > 0 {
			hasInterface = true
		}
	}

	if !hasValidate {
		report.PatternViolations = append(report.PatternViolations, domain.MissingItem{
			Name:        "Validate",
			Expected:    "method",
			Description: "domain entities should have a Validate() error method",
		})
		report.Issues = append(report.Issues, domain.Issue{
			Severity: domain.SeverityWarning,
			Category: "pattern",
			Message:  "missing Validate() method on domain entity",
			Pattern:  "validate_method",
		})
	}

	if !hasConstructor {
		report.PatternViolations = append(report.PatternViolations, domain.MissingItem{
			Name:        "New{Entity}",
			Expected:    "function",
			Description: "modules should have constructor functions (New...)",
		})
		report.Issues = append(report.Issues, domain.Issue{
			Severity: domain.SeverityWarning,
			Category: "pattern",
			Message:  "missing constructor function (New...)",
			Pattern:  "constructor",
		})
	}

	if !hasInterface {
		report.PatternViolations = append(report.PatternViolations, domain.MissingItem{
			Name:        "port interfaces",
			Expected:    "interface",
			Description: "modules should define port interfaces",
		})
		report.Issues = append(report.Issues, domain.Issue{
			Severity: domain.SeverityWarning,
			Category: "pattern",
			Message:  "no interfaces defined (expected port interfaces)",
			Pattern:  "port_interfaces",
		})
	}
}

// detectEntityName finds the primary entity name from the domain layer of the
// target module. It looks for the first struct in a non-test, non-errors
// domain file.
func detectEntityName(module domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) string {
	modulePrefix := filepath.ToSlash(module.Path) + "/"
	for _, filePath := range module.Files {
		normalized := filepath.ToSlash(filePath)
		if !strings.HasPrefix(normalized, modulePrefix) {
			continue
		}
		rel := normalized[len(modulePrefix):]
		if !strings.HasPrefix(rel, "domain/") {
			continue
		}
		base := filepath.Base(rel)
		if strings.HasSuffix(base, "_test.go") || strings.Contains(base, "error") {
			continue
		}
		af, ok := analyzed[filePath]
		if !ok {
			continue
		}
		if len(af.Structs) > 0 {
			return af.Structs[0]
		}
	}
	return ""
}

// toSnakeCase converts PascalCase to snake_case.
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c-'A'+'a'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// toPascalCase converts a lowercase string to PascalCase.
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
