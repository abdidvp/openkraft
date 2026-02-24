package domain

import "fmt"

// ProjectType identifies the kind of project for default scoring tuning.
type ProjectType string

const (
	ProjectTypeAPI          ProjectType = "api"
	ProjectTypeCLI          ProjectType = "cli-tool"
	ProjectTypeLibrary      ProjectType = "library"
	ProjectTypeMicroservice ProjectType = "microservice"
)

// ValidProjectTypes enumerates all recognized project types.
var ValidProjectTypes = []ProjectType{
	ProjectTypeAPI,
	ProjectTypeCLI,
	ProjectTypeLibrary,
	ProjectTypeMicroservice,
}

// ValidCategories enumerates all scoring category names.
var ValidCategories = []string{
	"architecture", "conventions", "patterns",
	"tests", "ai_context", "completeness",
}

// ValidSubMetrics enumerates all scoring sub-metric names.
var ValidSubMetrics = []string{
	// architecture
	"consistent_module_structure", "layer_separation",
	"dependency_direction", "module_boundary_clarity", "architecture_documentation",
	// conventions
	"naming_consistency", "error_handling", "import_ordering",
	"file_organization", "code_style",
	// patterns
	"entity_patterns", "repository_patterns", "service_patterns",
	"port_patterns", "handler_patterns",
	// tests
	"unit_test_presence", "integration_tests", "test_helpers",
	"test_fixtures", "ci_config",
	// ai_context
	"claude_md", "cursor_rules", "agents_md", "openkraft_dir",
	// completeness
	"file_completeness", "structural_completeness", "documentation_completeness",
}

// ProjectConfig holds project-level configuration loaded from .openkraft.yaml.
type ProjectConfig struct {
	ProjectType   ProjectType        `yaml:"project_type"    json:"project_type,omitempty"`
	Weights       map[string]float64 `yaml:"weights"         json:"weights,omitempty"`
	Skip          SkipConfig         `yaml:"skip"            json:"skip,omitempty"`
	ExcludePaths  []string           `yaml:"exclude_paths"   json:"exclude_paths,omitempty"`
	MinThresholds map[string]int     `yaml:"min_thresholds"  json:"min_thresholds,omitempty"`
}

// SkipConfig specifies categories and sub-metrics to exclude from scoring.
type SkipConfig struct {
	Categories []string `yaml:"categories"  json:"categories,omitempty"`
	SubMetrics []string `yaml:"sub_metrics" json:"sub_metrics,omitempty"`
}

// DefaultConfig returns a zero-value config that changes nothing.
func DefaultConfig() ProjectConfig {
	return ProjectConfig{}
}

// DefaultConfigForType returns sensible defaults for a given project type.
func DefaultConfigForType(pt ProjectType) ProjectConfig {
	cfg := ProjectConfig{ProjectType: pt}

	switch pt {
	case ProjectTypeCLI:
		cfg.Weights = map[string]float64{
			"architecture": 0.20, "conventions": 0.25, "patterns": 0.10,
			"tests": 0.20, "ai_context": 0.10, "completeness": 0.15,
		}
		cfg.Skip.SubMetrics = []string{"handler_patterns", "repository_patterns"}

	case ProjectTypeLibrary:
		cfg.Weights = map[string]float64{
			"architecture": 0.15, "conventions": 0.25, "patterns": 0.15,
			"tests": 0.25, "ai_context": 0.10, "completeness": 0.10,
		}
		cfg.Skip.SubMetrics = []string{"handler_patterns", "repository_patterns", "port_patterns"}

	case ProjectTypeMicroservice:
		cfg.Weights = map[string]float64{
			"architecture": 0.25, "conventions": 0.20, "patterns": 0.20,
			"tests": 0.15, "ai_context": 0.05, "completeness": 0.15,
		}

	default: // api or unrecognized
		cfg.Weights = map[string]float64{
			"architecture": 0.25, "conventions": 0.20, "patterns": 0.20,
			"tests": 0.15, "ai_context": 0.10, "completeness": 0.10,
		}
	}

	return cfg
}

// IsSkippedCategory reports whether the named category is excluded.
func (c ProjectConfig) IsSkippedCategory(name string) bool {
	for _, s := range c.Skip.Categories {
		if s == name {
			return true
		}
	}
	return false
}

// IsSkippedSubMetric reports whether the named sub-metric is excluded.
func (c ProjectConfig) IsSkippedSubMetric(name string) bool {
	for _, s := range c.Skip.SubMetrics {
		if s == name {
			return true
		}
	}
	return false
}

// Validate checks the config for invalid values and returns a descriptive error.
func (c ProjectConfig) Validate() error {
	// 1. project_type must be known or empty
	if c.ProjectType != "" {
		valid := false
		for _, pt := range ValidProjectTypes {
			if c.ProjectType == pt {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unknown project_type %q (valid: api, cli-tool, library, microservice)", c.ProjectType)
		}
	}

	// 2. weights keys must be valid categories
	for k := range c.Weights {
		if !isValidCategory(k) {
			return fmt.Errorf("unknown category %q in weights", k)
		}
	}

	// 3. if all 6 categories specified, weights must sum to ~1.0
	if len(c.Weights) == len(ValidCategories) {
		sum := 0.0
		for _, w := range c.Weights {
			sum += w
		}
		if sum < 0.95 || sum > 1.05 {
			return fmt.Errorf("weights sum to %.2f (must be between 0.95 and 1.05)", sum)
		}
	}

	// 4. skip.categories must be valid
	for _, cat := range c.Skip.Categories {
		if !isValidCategory(cat) {
			return fmt.Errorf("unknown category %q in skip.categories", cat)
		}
	}

	// 5. cannot skip ALL categories
	if len(c.Skip.Categories) > 0 && len(c.Skip.Categories) >= len(ValidCategories) {
		return fmt.Errorf("cannot skip all categories (must have at least one active)")
	}

	// 6. skip.sub_metrics must be valid
	for _, sm := range c.Skip.SubMetrics {
		if !isValidSubMetric(sm) {
			return fmt.Errorf("unknown sub-metric %q in skip.sub_metrics", sm)
		}
	}

	// 7. min_thresholds keys must be valid categories, values 0-100
	for k, v := range c.MinThresholds {
		if !isValidCategory(k) {
			return fmt.Errorf("unknown category %q in min_thresholds", k)
		}
		if v < 0 || v > 100 {
			return fmt.Errorf("min_thresholds[%q] = %d (must be between 0 and 100)", k, v)
		}
	}

	return nil
}

func isValidCategory(name string) bool {
	for _, c := range ValidCategories {
		if c == name {
			return true
		}
	}
	return false
}

func isValidSubMetric(name string) bool {
	for _, sm := range ValidSubMetrics {
		if sm == name {
			return true
		}
	}
	return false
}

// EffectiveWeight returns the configured weight for a category,
// falling back to defaultWeight if not specified.
func (c ProjectConfig) EffectiveWeight(category string, defaultWeight float64) float64 {
	if w, ok := c.Weights[category]; ok {
		return w
	}
	return defaultWeight
}
