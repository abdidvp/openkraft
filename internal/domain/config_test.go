package domain_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig_ChangesNothing(t *testing.T) {
	cfg := domain.DefaultConfig()
	assert.Empty(t, cfg.ProjectType)
	assert.Nil(t, cfg.Weights)
	assert.Empty(t, cfg.Skip.Categories)
	assert.Empty(t, cfg.Skip.SubMetrics)
	assert.Empty(t, cfg.ExcludePaths)
	assert.Nil(t, cfg.MinThresholds)
}

func TestDefaultConfigForType_API(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeAPI)
	assert.Equal(t, domain.ProjectTypeAPI, cfg.ProjectType)
	assert.Empty(t, cfg.Skip.SubMetrics)
	assert.InDelta(t, 0.25, cfg.Weights["code_health"], 0.001)
	assert.InDelta(t, 0.15, cfg.Weights["structure"], 0.001)
}

func TestDefaultConfigForType_CLI(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeCLI)
	assert.Equal(t, domain.ProjectTypeCLI, cfg.ProjectType)
	assert.Contains(t, cfg.Skip.SubMetrics, "interface_contracts")
	assert.Contains(t, cfg.Skip.SubMetrics, "module_completeness")
	assert.InDelta(t, 0.20, cfg.Weights["discoverability"], 0.001)
	assert.InDelta(t, 0.10, cfg.Weights["structure"], 0.001)
}

func TestDefaultConfigForType_Library(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeLibrary)
	assert.Equal(t, domain.ProjectTypeLibrary, cfg.ProjectType)
	assert.Contains(t, cfg.Skip.SubMetrics, "interface_contracts")
	assert.InDelta(t, 0.25, cfg.Weights["verifiability"], 0.001)
}

func TestDefaultConfigForType_Microservice(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeMicroservice)
	assert.Equal(t, domain.ProjectTypeMicroservice, cfg.ProjectType)
	assert.Empty(t, cfg.Skip.SubMetrics)
	assert.InDelta(t, 0.20, cfg.Weights["structure"], 0.001)
}

func TestIsSkippedCategory(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{Categories: []string{"structure", "context_quality"}},
	}
	assert.True(t, cfg.IsSkippedCategory("structure"))
	assert.True(t, cfg.IsSkippedCategory("context_quality"))
	assert.False(t, cfg.IsSkippedCategory("verifiability"))
}

func TestIsSkippedCategory_Empty(t *testing.T) {
	cfg := domain.DefaultConfig()
	assert.False(t, cfg.IsSkippedCategory("verifiability"))
}

func TestIsSkippedSubMetric(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{SubMetrics: []string{"interface_contracts", "module_completeness"}},
	}
	assert.True(t, cfg.IsSkippedSubMetric("interface_contracts"))
	assert.True(t, cfg.IsSkippedSubMetric("module_completeness"))
	assert.False(t, cfg.IsSkippedSubMetric("expected_layers"))
}

func TestEffectiveWeight_Override(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{"verifiability": 0.30},
	}
	assert.InDelta(t, 0.30, cfg.EffectiveWeight("verifiability", 0.15), 0.001)
}

func TestEffectiveWeight_FallbackToDefault(t *testing.T) {
	cfg := domain.DefaultConfig()
	assert.InDelta(t, 0.15, cfg.EffectiveWeight("verifiability", 0.15), 0.001)
}

// --- Validation tests ---

func TestValidate_EmptyConfigIsValid(t *testing.T) {
	cfg := domain.DefaultConfig()
	assert.NoError(t, cfg.Validate())
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeCLI)
	assert.NoError(t, cfg.Validate())
}

func TestValidate_UnknownProjectType(t *testing.T) {
	cfg := domain.ProjectConfig{ProjectType: "webapp"}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown project_type")
	assert.Contains(t, err.Error(), "webapp")
}

func TestValidate_WeightsSumTooHigh(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{
			"code_health": 0.50, "discoverability": 0.50, "structure": 0.50,
			"verifiability": 0.15, "context_quality": 0.10, "predictability": 0.10,
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weights sum")
}

func TestValidate_WeightsSumTooLow(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{
			"code_health": 0.05, "discoverability": 0.05, "structure": 0.05,
			"verifiability": 0.05, "context_quality": 0.05, "predictability": 0.05,
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weights sum")
}

func TestValidate_InvalidCategoryInWeights(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{"nonexistent": 0.25},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown category")
}

func TestValidate_InvalidCategoryInSkip(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{Categories: []string{"fake_category"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown category")
}

func TestValidate_InvalidSubMetricInSkip(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{SubMetrics: []string{"http_handlers"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown sub-metric")
}

func TestValidate_ThresholdOutOfRange(t *testing.T) {
	cfg := domain.ProjectConfig{
		MinThresholds: map[string]int{"verifiability": 150},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "between 0 and 100")
}

func TestValidate_ThresholdInvalidCategory(t *testing.T) {
	cfg := domain.ProjectConfig{
		MinThresholds: map[string]int{"fake": 50},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown category")
}

func TestValidate_AllCategoriesSkipped(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{Categories: domain.ValidCategories},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot skip all categories")
}

func TestValidate_PartialWeightsValid(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{"verifiability": 0.30},
	}
	assert.NoError(t, cfg.Validate())
}

// --- Profile validation tests ---

func TestValidate_ProfileNamingConventionValid(t *testing.T) {
	for _, nc := range []string{"", "auto", "bare", "suffixed"} {
		cfg := domain.ProjectConfig{Profile: &domain.ProfileOverrides{NamingConvention: nc}}
		assert.NoError(t, cfg.Validate(), "naming_convention %q should be valid", nc)
	}
}

func TestValidate_ProfileNamingConventionInvalid(t *testing.T) {
	cfg := domain.ProjectConfig{Profile: &domain.ProfileOverrides{NamingConvention: "camelCase"}}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown naming_convention")
}

func TestValidate_ProfileNegativeThreshold(t *testing.T) {
	neg := -1
	cfg := domain.ProjectConfig{Profile: &domain.ProfileOverrides{MaxFunctionLines: &neg}}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_function_lines")
	assert.Contains(t, err.Error(), "> 0")
}

func TestValidate_ProfileZeroThreshold(t *testing.T) {
	zero := 0
	cfg := domain.ProjectConfig{Profile: &domain.ProfileOverrides{MaxParameters: &zero}}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_parameters")
}

func TestValidate_ProfileTestRatioOutOfRange(t *testing.T) {
	ratio := 1.5
	cfg := domain.ProjectConfig{Profile: &domain.ProfileOverrides{MinTestRatio: &ratio}}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_test_ratio")
}

func TestValidate_ProfileContextFileEmptyName(t *testing.T) {
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			ContextFiles: []domain.ContextFileSpec{{Name: "", Points: 10}},
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestValidate_ProfileContextFileZeroPoints(t *testing.T) {
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			ContextFiles: []domain.ContextFileSpec{{Name: "CLAUDE.md", Points: 0}},
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "points must be > 0")
}

func TestValidate_ProfileValidOverrides(t *testing.T) {
	maxFunc := 80
	maxFile := 500
	ratio := 0.7
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			ExpectedLayers:   []string{"domain", "infra"},
			NamingConvention: "bare",
			MaxFunctionLines: &maxFunc,
			MaxFileLines:     &maxFile,
			MinTestRatio:     &ratio,
			ContextFiles: []domain.ContextFileSpec{
				{Name: "CLAUDE.md", Points: 15, MinSize: 500},
			},
		},
	}
	assert.NoError(t, cfg.Validate())
}

func TestDefaultConfigForType_WeightsSum(t *testing.T) {
	for _, pt := range []domain.ProjectType{
		domain.ProjectTypeAPI,
		domain.ProjectTypeCLI,
		domain.ProjectTypeLibrary,
		domain.ProjectTypeMicroservice,
	} {
		cfg := domain.DefaultConfigForType(pt)
		sum := 0.0
		for _, w := range cfg.Weights {
			sum += w
		}
		assert.InDelta(t, 1.0, sum, 0.05, "weights for %s should sum to ~1.0", pt)
	}
}
