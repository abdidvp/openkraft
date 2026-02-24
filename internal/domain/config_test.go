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
	// API is the baseline — no skips, standard weights
	assert.Empty(t, cfg.Skip.SubMetrics)
	assert.InDelta(t, 0.25, cfg.Weights["architecture"], 0.001)
	assert.InDelta(t, 0.20, cfg.Weights["patterns"], 0.001)
}

func TestDefaultConfigForType_CLI(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeCLI)
	assert.Equal(t, domain.ProjectTypeCLI, cfg.ProjectType)
	assert.Contains(t, cfg.Skip.SubMetrics, "handler_patterns")
	assert.Contains(t, cfg.Skip.SubMetrics, "repository_patterns")
	assert.InDelta(t, 0.25, cfg.Weights["conventions"], 0.001)
	assert.InDelta(t, 0.10, cfg.Weights["patterns"], 0.001)
}

func TestDefaultConfigForType_Library(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeLibrary)
	assert.Equal(t, domain.ProjectTypeLibrary, cfg.ProjectType)
	assert.Contains(t, cfg.Skip.SubMetrics, "handler_patterns")
	assert.Contains(t, cfg.Skip.SubMetrics, "repository_patterns")
	assert.Contains(t, cfg.Skip.SubMetrics, "port_patterns")
	assert.InDelta(t, 0.25, cfg.Weights["tests"], 0.001)
}

func TestDefaultConfigForType_Microservice(t *testing.T) {
	cfg := domain.DefaultConfigForType(domain.ProjectTypeMicroservice)
	assert.Equal(t, domain.ProjectTypeMicroservice, cfg.ProjectType)
	assert.Empty(t, cfg.Skip.SubMetrics)
	assert.InDelta(t, 0.15, cfg.Weights["completeness"], 0.001)
}

func TestIsSkippedCategory(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{Categories: []string{"completeness", "ai_context"}},
	}
	assert.True(t, cfg.IsSkippedCategory("completeness"))
	assert.True(t, cfg.IsSkippedCategory("ai_context"))
	assert.False(t, cfg.IsSkippedCategory("tests"))
}

func TestIsSkippedCategory_Empty(t *testing.T) {
	cfg := domain.DefaultConfig()
	assert.False(t, cfg.IsSkippedCategory("tests"))
}

func TestIsSkippedSubMetric(t *testing.T) {
	cfg := domain.ProjectConfig{
		Skip: domain.SkipConfig{SubMetrics: []string{"handler_patterns", "repository_patterns"}},
	}
	assert.True(t, cfg.IsSkippedSubMetric("handler_patterns"))
	assert.True(t, cfg.IsSkippedSubMetric("repository_patterns"))
	assert.False(t, cfg.IsSkippedSubMetric("service_patterns"))
}

func TestEffectiveWeight_Override(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{"tests": 0.30},
	}
	assert.InDelta(t, 0.30, cfg.EffectiveWeight("tests", 0.15), 0.001)
}

func TestEffectiveWeight_FallbackToDefault(t *testing.T) {
	cfg := domain.DefaultConfig()
	assert.InDelta(t, 0.15, cfg.EffectiveWeight("tests", 0.15), 0.001)
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
			"architecture": 0.50, "conventions": 0.50, "patterns": 0.50,
			"tests": 0.15, "ai_context": 0.10, "completeness": 0.10,
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weights sum")
}

func TestValidate_WeightsSumTooLow(t *testing.T) {
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{
			"architecture": 0.05, "conventions": 0.05, "patterns": 0.05,
			"tests": 0.05, "ai_context": 0.05, "completeness": 0.05,
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
		MinThresholds: map[string]int{"tests": 150},
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
	// Only specifying some weights is OK — they're merged with defaults at runtime
	cfg := domain.ProjectConfig{
		Weights: map[string]float64{"tests": 0.30},
	}
	// Partial weights don't need to sum to 1.0
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
