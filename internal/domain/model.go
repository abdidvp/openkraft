package domain

import (
	"math"
	"time"
)

// Score represents the overall AI-readiness score of a project.
type Score struct {
	Overall      int             `json:"overall"`
	Categories   []CategoryScore `json:"categories"`
	Timestamp    time.Time       `json:"timestamp"`
	CommitHash   string          `json:"commit_hash,omitempty"`
	ModuleScores []ModuleScore   `json:"module_scores,omitempty"`
}

func (s Score) Grade() string { return GradeFor(s.Overall) }

func GradeFor(score int) string {
	switch {
	case score >= 90:
		return "A+"
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

func BadgeColor(score int) string {
	switch {
	case score >= 90:
		return "brightgreen"
	case score >= 80:
		return "green"
	case score >= 70:
		return "yellow"
	case score >= 60:
		return "orange"
	case score >= 50:
		return "red"
	default:
		return "critical"
	}
}

// CategoryScore represents a single scoring category (e.g. architecture, tests).
type CategoryScore struct {
	Name       string      `json:"name"`
	Score      int         `json:"score"`
	Weight     float64     `json:"weight"`
	SubMetrics []SubMetric `json:"sub_metrics,omitempty"`
	Issues     []Issue     `json:"issues,omitempty"`
}

type SubMetric struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Points int    `json:"points"`
	Detail string `json:"detail,omitempty"`
}

type ModuleScore struct {
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	Score          int     `json:"score"`
	FileCount      int     `json:"file_count"`
	MissingFiles   int     `json:"missing_files"`
	MissingMethods int     `json:"missing_methods"`
	Issues         []Issue `json:"issues,omitempty"`
}

func ComputeOverallScore(categories []CategoryScore) int {
	var totalWeighted, totalWeight float64
	for _, c := range categories {
		totalWeighted += float64(c.Score) * c.Weight
		totalWeight += c.Weight
	}
	if totalWeight == 0 {
		return 0
	}
	return int(math.Round(totalWeighted / totalWeight))
}

// Issue represents a problem found during analysis.
type Issue struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	File         string `json:"file,omitempty"`
	Line         int    `json:"line,omitempty"`
	Message      string `json:"message"`
	Pattern      string `json:"pattern,omitempty"`
	FixAvailable bool   `json:"fix_available"`
}

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Module represents a detected module in the project.
type Module struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Language string       `json:"language"`
	Files    []ModuleFile `json:"files"`
	Layers   []string     `json:"layers,omitempty"`
}

type ModuleFile struct {
	Path         string   `json:"path"`
	RelativePath string   `json:"relative_path"`
	Layer        string   `json:"layer,omitempty"`
	Type         string   `json:"type,omitempty"`
	HasTest      bool     `json:"has_test"`
	Functions    []string `json:"functions,omitempty"`
	Structs      []string `json:"structs,omitempty"`
	Interfaces   []string `json:"interfaces,omitempty"`
	Imports      []string `json:"imports,omitempty"`
}

// Blueprint represents the structural template extracted from a golden module.
type Blueprint struct {
	Name          string          `json:"name"`
	ExtractedFrom string          `json:"extracted_from"`
	Files         []BlueprintFile `json:"files"`
	Patterns      []string        `json:"patterns,omitempty"`
}

// CheckReport holds the result of comparing a module against a blueprint.
type CheckReport struct {
	Module            string        `json:"module"`
	GoldenModule      string        `json:"golden_module"`
	Score             int           `json:"score"`
	MissingFiles      []MissingItem `json:"missing_files"`
	MissingStructs    []MissingItem `json:"missing_structs"`
	MissingMethods    []MissingItem `json:"missing_methods"`
	MissingInterfaces []MissingItem `json:"missing_interfaces"`
	PatternViolations []MissingItem `json:"pattern_violations"`
	Issues            []Issue       `json:"issues"`
}

// MissingItem describes a single missing structural element.
type MissingItem struct {
	Name        string `json:"name"`
	Expected    string `json:"expected"`
	File        string `json:"file,omitempty"`
	Description string `json:"description,omitempty"`
}

// BlueprintFile describes a file pattern within a blueprint.
type BlueprintFile struct {
	PathPattern        string   `json:"path_pattern"`
	Type               string   `json:"type"`
	Required           bool     `json:"required"`
	RequiredStructs    []string `json:"required_structs,omitempty"`
	RequiredFunctions  []string `json:"required_functions,omitempty"`
	RequiredMethods    []string `json:"required_methods,omitempty"`
	RequiredInterfaces []string `json:"required_interfaces,omitempty"`
}
