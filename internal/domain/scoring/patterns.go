package scoring

import (
	"fmt"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScorePatterns evaluates pattern compliance across modules.
// It auto-detects patterns present in >50% of modules and measures compliance.
// Weight: 0.20 (20% of overall score).
func ScorePatterns(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "patterns",
		Weight: 0.20,
	}

	sm1 := scoreEntityPatterns(modules, analyzed)
	sm2 := scoreRepositoryPatterns(modules, analyzed)
	sm3 := scoreServicePatterns(modules, analyzed)
	sm4 := scorePortPatterns(modules, analyzed)
	sm5 := scoreHandlerPatterns(modules, analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectPatternIssues(modules, analyzed)

	return cat
}

// scoreEntityPatterns (30 pts) checks that domain entities have Validate()
// method and New{Entity} constructor.
func scoreEntityPatterns(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "entity_patterns",
		Points: 30,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	totalModules := 0
	compliantModules := 0

	for _, m := range modules {
		type entityInfo struct {
			hasValidate    bool
			hasConstructor bool
		}
		entities := make(map[string]*entityInfo)

		// Collect structs from domain files in this module.
		for _, f := range m.Files {
			if !isDomainLayerFile(f) {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			for _, s := range af.Structs {
				entities[s] = &entityInfo{}
			}
		}

		if len(entities) == 0 {
			continue
		}
		totalModules++

		// Check for Validate methods and New* constructors in domain files.
		for _, f := range m.Files {
			if !isDomainLayerFile(f) {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			for _, fn := range af.Functions {
				if fn.Name == "Validate" && fn.Receiver != "" {
					recv := strings.TrimPrefix(fn.Receiver, "*")
					if info, ok := entities[recv]; ok {
						info.hasValidate = true
					}
				}
				if strings.HasPrefix(fn.Name, "New") && fn.Receiver == "" {
					entityName := strings.TrimPrefix(fn.Name, "New")
					if info, ok := entities[entityName]; ok {
						info.hasConstructor = true
					}
				}
			}
		}

		// Module complies if at least one entity has both Validate and constructor.
		for _, info := range entities {
			if info.hasValidate && info.hasConstructor {
				compliantModules++
				break
			}
		}
	}

	if totalModules == 0 {
		sm.Detail = "no modules with domain entities found"
		return sm
	}

	sm.Score = int(float64(compliantModules) / float64(totalModules) * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d modules with domain entities follow entity patterns", compliantModules, totalModules)
	return sm
}

// scoreRepositoryPatterns (25 pts) checks for getQuerier function and
// consistent CRUD methods in repository adapter files.
func scoreRepositoryPatterns(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "repository_patterns",
		Points: 25,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	totalModules := 0
	compliantModules := 0

	for _, m := range modules {
		hasRepoFiles := false
		hasGetQuerier := false
		hasCRUD := false

		for _, f := range m.Files {
			if !isRepositoryFile(f) {
				continue
			}
			hasRepoFiles = true

			af, ok := analyzed[f]
			if !ok {
				continue
			}

			hasCreate := false
			hasGetByID := false
			hasList := false

			for _, fn := range af.Functions {
				if fn.Name == "getQuerier" && fn.Receiver == "" {
					hasGetQuerier = true
				}
				if fn.Name == "Create" && fn.Receiver != "" {
					hasCreate = true
				}
				if fn.Name == "GetByID" && fn.Receiver != "" {
					hasGetByID = true
				}
				if fn.Name == "List" && fn.Receiver != "" {
					hasList = true
				}
			}

			if hasCreate && hasGetByID && hasList {
				hasCRUD = true
			}
		}

		if !hasRepoFiles {
			continue
		}
		totalModules++

		if hasGetQuerier && hasCRUD {
			compliantModules++
		}
	}

	if totalModules == 0 {
		sm.Detail = "no modules with repository adapters found"
		return sm
	}

	sm.Score = int(float64(compliantModules) / float64(totalModules) * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d modules with repositories follow repository patterns", compliantModules, totalModules)
	return sm
}

// scoreServicePatterns (20 pts) checks for constructor injection:
// New{Service} functions taking interface params in application layer.
func scoreServicePatterns(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "service_patterns",
		Points: 20,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	totalModules := 0
	compliantModules := 0

	for _, m := range modules {
		hasServiceFiles := false
		hasServiceConstructor := false

		for _, f := range m.Files {
			if !isApplicationFile(f) {
				continue
			}

			af, ok := analyzed[f]
			if !ok {
				continue
			}

			hasStructs := len(af.Structs) > 0
			if !hasStructs {
				continue
			}
			hasServiceFiles = true

			for _, fn := range af.Functions {
				if strings.HasPrefix(fn.Name, "New") && fn.Receiver == "" {
					hasServiceConstructor = true
				}
			}
		}

		if !hasServiceFiles {
			continue
		}
		totalModules++

		if hasServiceConstructor {
			compliantModules++
		}
	}

	if totalModules == 0 {
		sm.Detail = "no modules with application services found"
		return sm
	}

	sm.Score = int(float64(compliantModules) / float64(totalModules) * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d modules with services follow constructor injection pattern", compliantModules, totalModules)
	return sm
}

// scorePortPatterns (15 pts) checks that port interfaces are defined in the
// application layer.
func scorePortPatterns(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "port_patterns",
		Points: 15,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	totalModules := 0
	compliantModules := 0

	for _, m := range modules {
		hasAppLayer := false
		hasInterfaces := false

		for _, f := range m.Files {
			if !isApplicationFile(f) {
				continue
			}
			hasAppLayer = true

			af, ok := analyzed[f]
			if !ok {
				continue
			}

			if len(af.Interfaces) > 0 {
				hasInterfaces = true
			}
		}

		if !hasAppLayer {
			continue
		}
		totalModules++

		if hasInterfaces {
			compliantModules++
		}
	}

	if totalModules == 0 {
		sm.Detail = "no modules with application layer found"
		return sm
	}

	sm.Score = int(float64(compliantModules) / float64(totalModules) * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d modules with application layer define port interfaces", compliantModules, totalModules)
	return sm
}

// scoreHandlerPatterns (10 pts) checks that HTTP handlers follow consistent
// structure: a handler struct with a New{Handler} constructor.
func scoreHandlerPatterns(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "handler_patterns",
		Points: 10,
	}

	if len(modules) == 0 {
		sm.Detail = "no modules detected"
		return sm
	}

	totalModules := 0
	compliantModules := 0

	for _, m := range modules {
		hasHandlerFiles := false
		hasHandlerStruct := false
		hasHandlerConstructor := false

		for _, f := range m.Files {
			if !isHTTPAdapterFile(f) {
				continue
			}

			af, ok := analyzed[f]
			if !ok {
				continue
			}

			if len(af.Structs) > 0 {
				hasHandlerFiles = true
				for _, s := range af.Structs {
					if strings.HasSuffix(s, "Handler") {
						hasHandlerStruct = true
					}
				}
			}

			for _, fn := range af.Functions {
				if strings.HasPrefix(fn.Name, "New") && strings.HasSuffix(fn.Name, "Handler") && fn.Receiver == "" {
					hasHandlerConstructor = true
				}
			}
		}

		if !hasHandlerFiles {
			continue
		}
		totalModules++

		if hasHandlerStruct && hasHandlerConstructor {
			compliantModules++
		}
	}

	if totalModules == 0 {
		sm.Detail = "no modules with HTTP handlers found"
		return sm
	}

	sm.Score = int(float64(compliantModules) / float64(totalModules) * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d modules with handlers follow handler patterns", compliantModules, totalModules)
	return sm
}

// collectPatternIssues generates issues for modules missing expected patterns.
func collectPatternIssues(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	if len(modules) == 0 {
		issues = append(issues, domain.Issue{
			Severity: domain.SeverityWarning,
			Category: "patterns",
			Message:  "no modules detected; cannot evaluate patterns",
		})
		return issues
	}

	for _, m := range modules {
		// Check entity patterns.
		hasDomainEntities := false
		hasEntityPattern := false
		for _, f := range m.Files {
			if !isDomainLayerFile(f) {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			if len(af.Structs) > 0 {
				hasDomainEntities = true
			}
			for _, fn := range af.Functions {
				if fn.Name == "Validate" && fn.Receiver != "" {
					hasEntityPattern = true
				}
			}
		}
		if hasDomainEntities && !hasEntityPattern {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "patterns",
				Message:  fmt.Sprintf("module %q: domain entities missing Validate() method or New* constructor", m.Name),
			})
		}

		// Check repository patterns.
		hasRepoFiles := false
		hasRepoPattern := false
		for _, f := range m.Files {
			if !isRepositoryFile(f) {
				continue
			}
			hasRepoFiles = true
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			for _, fn := range af.Functions {
				if fn.Name == "getQuerier" {
					hasRepoPattern = true
				}
			}
		}
		if hasRepoFiles && !hasRepoPattern {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityInfo,
				Category: "patterns",
				Message:  fmt.Sprintf("module %q: repository adapter missing getQuerier function", m.Name),
			})
		}

		// Check service patterns.
		hasServiceFiles := false
		hasServicePattern := false
		for _, f := range m.Files {
			if !isApplicationFile(f) {
				continue
			}
			af, ok := analyzed[f]
			if !ok {
				continue
			}
			if len(af.Structs) > 0 {
				hasServiceFiles = true
				for _, fn := range af.Functions {
					if strings.HasPrefix(fn.Name, "New") && fn.Receiver == "" {
						hasServicePattern = true
					}
				}
			}
		}
		if hasServiceFiles && !hasServicePattern {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "patterns",
				Message:  fmt.Sprintf("module %q: application service missing New* constructor injection", m.Name),
			})
		}
	}

	return issues
}

// --- helpers ---

// isDomainLayerFile checks if a file is in a domain layer directory.
func isDomainLayerFile(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	return strings.Contains(normalized, "/domain/")
}

// isRepositoryFile checks if a file is in a repository adapter directory.
func isRepositoryFile(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	return strings.Contains(normalized, "/adapters/repository/") ||
		strings.Contains(normalized, "/adapter/repository/")
}

// isApplicationFile checks if a file is in an application layer directory.
func isApplicationFile(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	return strings.Contains(normalized, "/application/")
}

// isHTTPAdapterFile checks if a file is in an HTTP adapter directory.
func isHTTPAdapterFile(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	return strings.Contains(normalized, "/adapters/http/") ||
		strings.Contains(normalized, "/adapter/http/")
}
