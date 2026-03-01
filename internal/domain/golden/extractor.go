package golden

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/abdidvp/openkraft/internal/domain"
)

// ExtractBlueprint analyzes a golden module and produces a Blueprint that
// describes the structural pattern (files, structs, functions, interfaces)
// with names generalized using {Entity} and {module} placeholders.
func ExtractBlueprint(module domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) (*domain.Blueprint, error) {
	if module.Name == "" {
		return nil, fmt.Errorf("module name is empty")
	}
	if len(module.Files) == 0 {
		return nil, fmt.Errorf("module has no files")
	}

	entityName := detectEntityName(module, analyzed)
	if entityName == "" {
		return nil, fmt.Errorf("could not detect entity name for module %s", module.Name)
	}

	bp := &domain.Blueprint{
		Name:          module.Name,
		ExtractedFrom: module.Path,
	}

	for _, filePath := range module.Files {
		af, ok := analyzed[filePath]
		if !ok {
			continue
		}

		fileType := classifyFile(filePath, af)
		pathPattern := generalizePath(filePath, module.Name, entityName, module.Path)

		bpFile := domain.BlueprintFile{
			PathPattern: pathPattern,
			Type:        fileType,
			Required:    true,
		}

		// Generalize structs
		for _, s := range af.Structs {
			bpFile.RequiredStructs = append(bpFile.RequiredStructs, generalizeName(s, entityName, module.Name))
		}

		// Generalize functions (free functions vs methods)
		for _, fn := range af.Functions {
			genName := generalizeName(fn.Name, entityName, module.Name)
			if fn.Receiver != "" {
				bpFile.RequiredMethods = append(bpFile.RequiredMethods, genName)
			} else {
				bpFile.RequiredFunctions = append(bpFile.RequiredFunctions, genName)
			}
		}

		// Generalize interfaces
		for _, iface := range af.Interfaces {
			bpFile.RequiredInterfaces = append(bpFile.RequiredInterfaces, generalizeName(iface, entityName, module.Name))
		}

		bp.Files = append(bp.Files, bpFile)
	}

	// Collect patterns from layers
	bp.Patterns = append(bp.Patterns, module.Layers...)

	return bp, nil
}

// detectEntityName finds the primary entity name from the domain layer.
// It looks for the first struct in a non-test, non-errors domain file.
func detectEntityName(module domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) string {
	for _, filePath := range module.Files {
		if !isDomainEntityFile(filePath, module.Path) {
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

// isDomainEntityFile returns true if the file is the primary domain entity file
// (in the domain/ directory, not a test, not an errors file).
func isDomainEntityFile(filePath, modulePath string) bool {
	normalized := filepath.ToSlash(filePath)
	modulePrefix := filepath.ToSlash(modulePath)

	// Must be under the module's domain/ directory
	domainPrefix := modulePrefix + "/domain/"
	if !strings.HasPrefix(normalized, domainPrefix) {
		return false
	}

	base := filepath.Base(normalized)
	if strings.HasSuffix(base, "_test.go") {
		return false
	}
	if strings.Contains(base, "error") {
		return false
	}
	return true
}

// classifyFile determines the type of a file based on its path and content.
func classifyFile(filePath string, af *domain.AnalyzedFile) string {
	normalized := filepath.ToSlash(filePath)
	base := filepath.Base(normalized)

	switch {
	case strings.Contains(normalized, "/domain/") && strings.HasSuffix(base, "_test.go"):
		return "domain_test"
	case strings.Contains(normalized, "/domain/") && strings.Contains(base, "error"):
		return "domain_errors"
	case strings.Contains(normalized, "/domain/"):
		return "domain_entity"
	case strings.Contains(normalized, "/application/") && strings.Contains(base, "service"):
		return "service"
	case strings.Contains(normalized, "/application/") && strings.Contains(base, "port"):
		return "ports"
	case strings.Contains(normalized, "/adapters/http/") && strings.Contains(base, "handler"):
		return "handler"
	case strings.Contains(normalized, "/adapters/http/") && strings.Contains(base, "route"):
		return "routes"
	case strings.Contains(normalized, "/adapters/repository/"):
		return "repository"
	default:
		return "unknown"
	}
}

// generalizePath converts a concrete file path to a pattern using {entity} and {module}.
// e.g., "internal/tax/domain/tax_rule.go" -> "domain/{entity}.go"
func generalizePath(filePath, moduleName, entityName, modulePath string) string {
	normalized := filepath.ToSlash(filePath)
	modulePrefix := filepath.ToSlash(modulePath) + "/"

	// Get the path relative to the module
	relPath := normalized
	if strings.HasPrefix(normalized, modulePrefix) {
		relPath = normalized[len(modulePrefix):]
	}

	// Convert entity name to snake_case for file matching
	entitySnake := toSnakeCase(entityName)
	moduleSnake := strings.ToLower(moduleName)

	// Replace the entity-specific filename parts
	// e.g., "tax_rule" -> "{entity}", "tax" -> "{module}"
	base := filepath.Base(relPath)
	dir := filepath.Dir(relPath)

	// Remove .go extension for manipulation
	nameNoExt := strings.TrimSuffix(base, ".go")
	isTest := strings.HasSuffix(nameNoExt, "_test")
	if isTest {
		nameNoExt = strings.TrimSuffix(nameNoExt, "_test")
	}

	// Replace entity snake_case name first (more specific), then module name
	generalized := nameNoExt
	if entitySnake != "" {
		generalized = strings.ReplaceAll(generalized, entitySnake, "{entity}")
	}
	if moduleSnake != "" && moduleSnake != entitySnake {
		generalized = strings.ReplaceAll(generalized, moduleSnake, "{module}")
	}

	// Rebuild filename
	suffix := ".go"
	if isTest {
		suffix = "_test.go"
	}
	newBase := generalized + suffix

	if dir == "." {
		return newBase
	}
	return filepath.ToSlash(filepath.Join(dir, newBase))
}

// generalizeName replaces concrete entity/module names with placeholders.
// e.g., "TaxRule" -> "{Entity}", "NewTaxRule" -> "New{Entity}",
//
//	"TaxService" -> "{Entity}Service", "TaxRuleRepository" -> "{Entity}Repository"
func generalizeName(name, entityName, moduleName string) string {
	// Replace entity name (PascalCase) with {Entity}
	if entityName != "" && strings.Contains(name, entityName) {
		return strings.ReplaceAll(name, entityName, "{Entity}")
	}

	// Replace module name (PascalCase) with {Entity} for service/handler/etc names
	pascalModule := toPascalCase(moduleName)
	if pascalModule != "" && strings.Contains(name, pascalModule) {
		return strings.ReplaceAll(name, pascalModule, "{Entity}")
	}

	return name
}

// toSnakeCase converts PascalCase to snake_case.
// e.g., "TaxRule" -> "tax_rule"
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
// e.g., "tax" -> "Tax"
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
