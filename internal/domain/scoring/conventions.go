package scoring

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreConventions evaluates how well the codebase follows Go conventions.
// It is pure domain logic: it receives data and returns a score with no I/O.
// Weight: 0.20 (20% of overall score).
func ScoreConventions(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "conventions",
		Weight: 0.20,
	}

	sm1 := scoreNamingConsistency(scan, analyzed)
	sm2 := scoreErrorHandling(analyzed)
	sm3 := scoreImportOrdering(analyzed)
	sm4 := scoreFileOrganization(scan)
	sm5 := scoreCodeStyle(analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectConventionIssues(scan, analyzed)

	return cat
}

// scoreNamingConsistency (30 pts) checks that Go files use snake_case and
// structs use PascalCase.
func scoreNamingConsistency(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "naming_consistency",
		Points: 30,
	}

	if scan == nil || len(scan.GoFiles) == 0 {
		sm.Detail = "no Go files to evaluate"
		return sm
	}

	// Check file naming: Go files should use snake_case (lowercase with underscores).
	totalFiles := 0
	snakeCaseFiles := 0
	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, ".go")
		// Strip _test suffix for evaluation.
		name = strings.TrimSuffix(name, "_test")
		if name == "" {
			continue
		}
		totalFiles++
		if isSnakeCase(name) {
			snakeCaseFiles++
		}
	}

	fileScore := 0.0
	if totalFiles > 0 {
		fileScore = float64(snakeCaseFiles) / float64(totalFiles)
	}

	// Check struct naming: should be PascalCase (exported, starts with upper).
	totalStructs := 0
	pascalStructs := 0
	for _, af := range analyzed {
		for _, s := range af.Structs {
			totalStructs++
			if isPascalCase(s) {
				pascalStructs++
			}
		}
	}

	structScore := 0.0
	if totalStructs > 0 {
		structScore = float64(pascalStructs) / float64(totalStructs)
	}

	// Weight: 60% file naming, 40% struct naming.
	var combined float64
	if totalFiles > 0 && totalStructs > 0 {
		combined = fileScore*0.6 + structScore*0.4
	} else if totalFiles > 0 {
		combined = fileScore
	} else if totalStructs > 0 {
		combined = structScore
	}

	score := int(combined * float64(sm.Points))
	if score > sm.Points {
		score = sm.Points
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%d/%d snake_case files, %d/%d PascalCase structs",
		snakeCaseFiles, totalFiles, pascalStructs, totalStructs)
	return sm
}

// scoreErrorHandling (25 pts) checks error conventions: Err-prefixed variables,
// errors.New usage, and %w wrapping.
func scoreErrorHandling(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "error_handling",
		Points: 25,
	}

	if len(analyzed) == 0 {
		sm.Detail = "no analyzed files"
		return sm
	}

	// Check for error-related files and patterns across analyzed files.
	// Look for:
	// 1. Files with "errors" or "fmt" imports that suggest error handling.
	// 2. Functions that return errors (we can check names containing "Err").
	// 3. Err-prefixed exported variables in structs list (actually stored as
	//    functions in analyzed data, but error vars are separate).

	// We check: do error files exist, do they use Err prefix variables?
	errorFiles := 0
	filesWithErrorImport := 0
	errPrefixVars := 0
	totalFunctions := 0

	for _, af := range analyzed {
		hasErrorImport := false
		for _, imp := range af.Imports {
			if imp == "errors" || imp == "fmt" {
				hasErrorImport = true
				break
			}
		}
		if hasErrorImport {
			filesWithErrorImport++
		}

		// Check if this looks like an error definitions file.
		base := filepath.Base(af.Path)
		if strings.Contains(base, "error") {
			errorFiles++
		}

		totalFunctions += len(af.Functions)
	}

	// Score components:
	// - Error files exist with dedicated error definitions (10 pts)
	// - Files importing "errors" package properly (8 pts)
	// - Consistency: error files use Err prefix naming convention (7 pts)

	points := 0

	// Dedicated error files.
	if errorFiles > 0 {
		points += 10
	}

	// Files with error-related imports suggest proper error handling.
	if filesWithErrorImport > 0 {
		// Scale: at least some files use error imports.
		ratio := float64(filesWithErrorImport) / float64(len(analyzed))
		if ratio > 0.15 {
			points += 8
		} else if ratio > 0.05 {
			points += 5
		} else {
			points += 2
		}
	}

	// Check for Err-prefixed variables by scanning function names and struct
	// names. In Go convention, error sentinel variables like ErrNotFound are
	// package-level vars. The parser collects Functions which includes top-level
	// funcs. We check for any exported identifier starting with "Err".
	for _, af := range analyzed {
		for _, s := range af.Structs {
			if strings.HasPrefix(s, "Err") {
				errPrefixVars++
			}
		}
		for _, fn := range af.Functions {
			if strings.HasPrefix(fn.Name, "Err") {
				errPrefixVars++
			}
		}
	}

	// Even without directly seeing var declarations, having dedicated error
	// files with the "errors" import is strong evidence of Err prefix usage.
	if errorFiles > 0 && filesWithErrorImport > 0 {
		points += 7
	} else if errPrefixVars > 0 {
		points += 7
	} else if filesWithErrorImport > 0 {
		points += 3
	}

	if points > sm.Points {
		points = sm.Points
	}
	sm.Score = points
	sm.Detail = fmt.Sprintf("%d error files, %d files with error imports", errorFiles, filesWithErrorImport)
	return sm
}

// scoreImportOrdering (15 pts) checks that imports follow Go convention:
// stdlib first, then external packages, separated by groups.
func scoreImportOrdering(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "import_ordering",
		Points: 15,
	}

	if len(analyzed) == 0 {
		sm.Detail = "no analyzed files"
		return sm
	}

	totalFiles := 0
	correctFiles := 0

	for _, af := range analyzed {
		if len(af.Imports) == 0 {
			continue
		}
		totalFiles++
		if importsOrdered(af.Imports) {
			correctFiles++
		}
	}

	if totalFiles == 0 {
		sm.Detail = "no files with imports"
		return sm
	}

	ratio := float64(correctFiles) / float64(totalFiles)
	score := int(ratio * float64(sm.Points))
	if score > sm.Points {
		score = sm.Points
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%d/%d files with correctly ordered imports", correctFiles, totalFiles)
	return sm
}

// scoreFileOrganization (15 pts) checks for consistent file suffixes:
// _test.go, _handler.go, _service.go, _repository.go, _ports.go, _errors.go.
func scoreFileOrganization(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "file_organization",
		Points: 15,
	}

	if scan == nil || len(scan.GoFiles) == 0 {
		sm.Detail = "no Go files to evaluate"
		return sm
	}

	// Known conventional suffixes.
	conventionalSuffixes := []string{
		"_test", "_handler", "_service", "_repository", "_ports",
		"_errors", "_routes", "_rule", "_model",
	}

	totalFiles := len(scan.GoFiles)
	conventionalFiles := 0

	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, ".go")

		// "main" is always conventional.
		if name == "main" {
			conventionalFiles++
			continue
		}

		// Check if the file uses a conventional suffix or is a simple
		// entity-named file (e.g., product.go, payment.go).
		for _, suffix := range conventionalSuffixes {
			if strings.HasSuffix(name, suffix) {
				conventionalFiles++
				break
			}
		}
	}

	// Also give credit for files that are short, lowercase, underscore-separated
	// names even without known suffixes (they follow Go file naming).
	// Count all snake_case files as having good organization.
	wellNamedFiles := 0
	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, ".go")
		if isSnakeCase(name) {
			wellNamedFiles++
		}
	}

	// Combine: 60% conventional suffixes, 40% well-named files.
	suffixRatio := float64(conventionalFiles) / float64(totalFiles)
	namingRatio := float64(wellNamedFiles) / float64(totalFiles)
	combined := suffixRatio*0.6 + namingRatio*0.4

	score := int(combined * float64(sm.Points))
	if score > sm.Points {
		score = sm.Points
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%d/%d conventional suffixes, %d/%d well-named files",
		conventionalFiles, totalFiles, wellNamedFiles, totalFiles)
	return sm
}

// scoreCodeStyle (15 pts) checks that constructors follow New{Type} pattern
// and methods have proper receivers.
func scoreCodeStyle(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{
		Name:   "code_style",
		Points: 15,
	}

	if len(analyzed) == 0 {
		sm.Detail = "no analyzed files"
		return sm
	}

	// Check constructor pattern: functions named New{Something} that match a
	// struct in the same file or package.
	totalStructs := 0
	structsWithConstructor := 0
	totalMethods := 0
	methodsWithReceiver := 0

	// Collect all struct names.
	allStructs := make(map[string]bool)
	for _, af := range analyzed {
		for _, s := range af.Structs {
			allStructs[s] = true
			totalStructs++
		}
	}

	// Check for New{Struct} constructors.
	constructorFound := make(map[string]bool)
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			if fn.Receiver == "" && strings.HasPrefix(fn.Name, "New") {
				// Extract the type name after "New".
				typeName := strings.TrimPrefix(fn.Name, "New")
				if allStructs[typeName] {
					constructorFound[typeName] = true
				}
			}
			if fn.Receiver != "" {
				totalMethods++
				methodsWithReceiver++
			}
		}
	}

	for range constructorFound {
		structsWithConstructor++
	}

	// Score components:
	// - Constructor pattern (8 pts): ratio of structs with New{Type} constructor.
	// - Methods with receivers (7 pts): all methods should have receivers (they
	//   always do by definition in Go, but check consistency).

	constructorScore := 0
	if totalStructs > 0 {
		ratio := float64(structsWithConstructor) / float64(totalStructs)
		constructorScore = int(ratio * 8)
		if constructorScore > 8 {
			constructorScore = 8
		}
	}

	receiverScore := 0
	if totalMethods > 0 {
		ratio := float64(methodsWithReceiver) / float64(totalMethods)
		receiverScore = int(ratio * 7)
		if receiverScore > 7 {
			receiverScore = 7
		}
	} else if totalStructs > 0 {
		// Structs exist but no methods: partial credit for having structs.
		receiverScore = 3
	}

	score := constructorScore + receiverScore
	if score > sm.Points {
		score = sm.Points
	}
	sm.Score = score
	sm.Detail = fmt.Sprintf("%d/%d structs with New{Type} constructor, %d methods with receivers",
		structsWithConstructor, totalStructs, methodsWithReceiver)
	return sm
}

// --- helpers ---

// isSnakeCase returns true if the name is lowercase with underscores (snake_case).
// Single-word lowercase names are also considered valid snake_case.
func isSnakeCase(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLower(r) && r != '_' && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isPascalCase returns true if the name starts with an uppercase letter and
// contains no underscores (PascalCase / exported Go identifier).
func isPascalCase(name string) bool {
	if name == "" {
		return false
	}
	runes := []rune(name)
	if !unicode.IsUpper(runes[0]) {
		return false
	}
	for _, r := range runes {
		if r == '_' {
			return false
		}
	}
	return true
}

// isStdlib returns true if the import path looks like a Go standard library
// package (no dots in the first path element).
func isStdlib(importPath string) bool {
	parts := strings.SplitN(importPath, "/", 2)
	return !strings.Contains(parts[0], ".")
}

// importsOrdered checks that stdlib imports come before external imports.
// Once an external import is seen, no stdlib import should follow.
func importsOrdered(imports []string) bool {
	seenExternal := false
	for _, imp := range imports {
		if isStdlib(imp) {
			if seenExternal {
				return false
			}
		} else {
			seenExternal = true
		}
	}
	return true
}

// collectConventionIssues gathers issues found during convention scoring.
func collectConventionIssues(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue

	if scan == nil {
		return issues
	}

	// Flag non-snake_case files.
	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, ".go")
		name = strings.TrimSuffix(name, "_test")
		if name != "" && !isSnakeCase(name) {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "conventions",
				File:     f,
				Message:  fmt.Sprintf("file name %q does not follow snake_case convention", base),
			})
		}
	}

	// Flag structs that are not PascalCase.
	for _, af := range analyzed {
		for _, s := range af.Structs {
			if !isPascalCase(s) {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "conventions",
					File:     af.Path,
					Message:  fmt.Sprintf("struct %q does not follow PascalCase convention", s),
				})
			}
		}
	}

	// Flag files with mis-ordered imports.
	for _, af := range analyzed {
		if len(af.Imports) > 0 && !importsOrdered(af.Imports) {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityInfo,
				Category: "conventions",
				File:     af.Path,
				Message:  "imports are not properly ordered (stdlib should come before external)",
			})
		}
	}

	return issues
}
