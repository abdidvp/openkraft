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
	"code_health", "discoverability", "structure",
	"verifiability", "context_quality", "predictability",
}

// ValidSubMetrics enumerates all scoring sub-metric names.
var ValidSubMetrics = []string{
	// code_health
	"function_size", "file_size", "nesting_depth",
	"parameter_count", "complex_conditionals",
	// discoverability
	"naming_uniqueness", "file_naming_conventions",
	"predictable_structure", "dependency_direction",
	// structure
	"expected_layers", "expected_files",
	"interface_contracts", "module_completeness",
	// verifiability
	"test_presence", "test_naming",
	"build_reproducibility", "type_safety_signals",
	// context_quality
	"ai_context_files", "package_documentation",
	"architecture_docs", "canonical_examples",
	// predictability
	"self_describing_names", "explicit_dependencies",
	"error_message_quality", "consistent_patterns",
}

// ProjectConfig holds project-level configuration loaded from .openkraft.yaml.
type ProjectConfig struct {
	ProjectType   ProjectType        `yaml:"project_type"    json:"project_type,omitempty"`
	Weights       map[string]float64 `yaml:"weights"         json:"weights,omitempty"`
	Skip          SkipConfig         `yaml:"skip"            json:"skip,omitempty"`
	ExcludePaths  []string           `yaml:"exclude_paths"   json:"exclude_paths,omitempty"`
	MinThresholds map[string]int     `yaml:"min_thresholds"  json:"min_thresholds,omitempty"`
	Profile       *ProfileOverrides  `yaml:"profile,omitempty" json:"profile,omitempty"`
}

// ProfileOverrides allows users to override specific scoring profile parameters.
// Pointer types distinguish "not specified" from zero values.
type ProfileOverrides struct {
	ExpectedLayers       []string          `yaml:"expected_layers,omitempty"        json:"expected_layers,omitempty"`
	ExpectedDirs         []string          `yaml:"expected_dirs,omitempty"          json:"expected_dirs,omitempty"`
	LayerAliases         map[string]string `yaml:"layer_aliases,omitempty"          json:"layer_aliases,omitempty"`
	ExpectedFileSuffixes []string          `yaml:"expected_file_suffixes,omitempty" json:"expected_file_suffixes,omitempty"`
	NamingConvention     string            `yaml:"naming_convention,omitempty"      json:"naming_convention,omitempty"`
	MaxFunctionLines     *int              `yaml:"max_function_lines,omitempty"     json:"max_function_lines,omitempty"`
	MaxFileLines         *int              `yaml:"max_file_lines,omitempty"         json:"max_file_lines,omitempty"`
	MaxNestingDepth      *int              `yaml:"max_nesting_depth,omitempty"      json:"max_nesting_depth,omitempty"`
	MaxParameters        *int              `yaml:"max_parameters,omitempty"         json:"max_parameters,omitempty"`
	MaxConditionalOps    *int              `yaml:"max_conditional_ops,omitempty"    json:"max_conditional_ops,omitempty"`
	ExemptParamPatterns  []string          `yaml:"exempt_param_patterns,omitempty"  json:"exempt_param_patterns,omitempty"`
	ContextFiles         []ContextFileSpec `yaml:"context_files,omitempty"          json:"context_files,omitempty"`
	MinTestRatio         *float64          `yaml:"min_test_ratio,omitempty"         json:"min_test_ratio,omitempty"`
	MaxGlobalVarPenalty  *int              `yaml:"max_global_var_penalty,omitempty" json:"max_global_var_penalty,omitempty"`
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
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.10,
			"verifiability": 0.20, "context_quality": 0.10, "predictability": 0.15,
		}
		cfg.Skip.SubMetrics = []string{"interface_contracts", "module_completeness"}

	case ProjectTypeLibrary:
		cfg.Weights = map[string]float64{
			"code_health": 0.20, "discoverability": 0.20, "structure": 0.10,
			"verifiability": 0.25, "context_quality": 0.15, "predictability": 0.10,
		}
		cfg.Skip.SubMetrics = []string{"interface_contracts"}

	case ProjectTypeMicroservice:
		cfg.Weights = map[string]float64{
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.20,
			"verifiability": 0.15, "context_quality": 0.10, "predictability": 0.10,
		}

	default: // api or unrecognized
		cfg.Weights = map[string]float64{
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.15,
			"verifiability": 0.15, "context_quality": 0.15, "predictability": 0.10,
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

	// 8. profile validation
	if c.Profile != nil {
		if err := c.Profile.validate(); err != nil {
			return err
		}
	}

	return nil
}

// validNamingConventions lists allowed values for NamingConvention.
var validNamingConventions = []string{"", "auto", "bare", "suffixed"}

func (p ProfileOverrides) validate() error {
	// naming_convention must be known
	if p.NamingConvention != "" {
		valid := false
		for _, v := range validNamingConventions {
			if p.NamingConvention == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unknown naming_convention %q in profile (valid: auto, bare, suffixed)", p.NamingConvention)
		}
	}

	// int pointer fields must be > 0 if set
	intFields := map[string]*int{
		"max_function_lines":   p.MaxFunctionLines,
		"max_file_lines":       p.MaxFileLines,
		"max_nesting_depth":    p.MaxNestingDepth,
		"max_parameters":       p.MaxParameters,
		"max_conditional_ops":  p.MaxConditionalOps,
		"max_global_var_penalty": p.MaxGlobalVarPenalty,
	}
	for name, ptr := range intFields {
		if ptr != nil && *ptr <= 0 {
			return fmt.Errorf("profile.%s must be > 0 (got %d)", name, *ptr)
		}
	}

	// min_test_ratio must be in [0.0, 1.0]
	if p.MinTestRatio != nil {
		if *p.MinTestRatio < 0.0 || *p.MinTestRatio > 1.0 {
			return fmt.Errorf("profile.min_test_ratio must be between 0.0 and 1.0 (got %.2f)", *p.MinTestRatio)
		}
	}

	// context_files validation
	for i, cf := range p.ContextFiles {
		if cf.Name == "" {
			return fmt.Errorf("profile.context_files[%d].name must not be empty", i)
		}
		if cf.Points <= 0 {
			return fmt.Errorf("profile.context_files[%d].points must be > 0 (got %d)", i, cf.Points)
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
