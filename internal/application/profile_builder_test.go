package application_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/application"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestBuildProfile_EmptyConfigReturnsDefaults(t *testing.T) {
	cfg := domain.DefaultConfig()
	p := application.BuildProfile(cfg)

	base := domain.DefaultProfile()
	assert.Equal(t, base.ExpectedLayers, p.ExpectedLayers)
	assert.Equal(t, base.MaxFunctionLines, p.MaxFunctionLines)
	assert.Equal(t, base.MaxParameters, p.MaxParameters)
	assert.Equal(t, base.NamingConvention, p.NamingConvention)
	assert.Equal(t, base.MinTestRatio, p.MinTestRatio)
}

func TestBuildProfile_CLITypeReturnsCliDefaults(t *testing.T) {
	cfg := domain.ProjectConfig{ProjectType: domain.ProjectTypeCLI}
	p := application.BuildProfile(cfg)

	assert.Equal(t, []string{"domain", "application"}, p.ExpectedLayers)
	assert.Equal(t, []string{"_model", "_service"}, p.ExpectedFileSuffixes)
}

func TestBuildProfile_SingleOverrideMerges(t *testing.T) {
	maxFunc := 80
	cfg := domain.ProjectConfig{
		ProjectType: domain.ProjectTypeAPI,
		Profile: &domain.ProfileOverrides{
			MaxFunctionLines: &maxFunc,
		},
	}
	p := application.BuildProfile(cfg)

	// Overridden field
	assert.Equal(t, 80, p.MaxFunctionLines)
	// Non-overridden fields keep defaults
	assert.Equal(t, 300, p.MaxFileLines)
	assert.Equal(t, []string{"domain", "application", "adapters"}, p.ExpectedLayers)
}

func TestBuildProfile_MultipleOverrides(t *testing.T) {
	maxFunc := 100
	maxFile := 500
	ratio := 0.9
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			MaxFunctionLines: &maxFunc,
			MaxFileLines:     &maxFile,
			MinTestRatio:     &ratio,
			NamingConvention: "bare",
		},
	}
	p := application.BuildProfile(cfg)

	assert.Equal(t, 100, p.MaxFunctionLines)
	assert.Equal(t, 500, p.MaxFileLines)
	assert.Equal(t, 0.9, p.MinTestRatio)
	assert.Equal(t, "bare", p.NamingConvention)
}

func TestBuildProfile_LayersOverrideReplaces(t *testing.T) {
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			ExpectedLayers: []string{"domain", "infra"},
		},
	}
	p := application.BuildProfile(cfg)

	assert.Equal(t, []string{"domain", "infra"}, p.ExpectedLayers)
}

func TestBuildProfile_ContextFilesOverrideReplaces(t *testing.T) {
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			ContextFiles: []domain.ContextFileSpec{
				{Name: "CUSTOM.md", Points: 20, MinSize: 100},
			},
		},
	}
	p := application.BuildProfile(cfg)

	assert.Len(t, p.ContextFiles, 1)
	assert.Equal(t, "CUSTOM.md", p.ContextFiles[0].Name)
	assert.Equal(t, 20, p.ContextFiles[0].Points)
}

func TestBuildProfile_LayerAliasesOverride(t *testing.T) {
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			LayerAliases: map[string]string{"gateway": "adapters"},
		},
	}
	p := application.BuildProfile(cfg)

	assert.Equal(t, "adapters", p.LayerAliases["gateway"])
	// Original aliases are replaced, not merged
	_, hasAdapter := p.LayerAliases["adapter"]
	assert.False(t, hasAdapter, "original aliases should be replaced")
}

func TestBuildProfile_NewCognitiveComplexityOverride(t *testing.T) {
	maxCC := 20
	maxDup := 10
	minClone := 80
	cfg := domain.ProjectConfig{
		Profile: &domain.ProfileOverrides{
			MaxCognitiveComplexity: &maxCC,
			MaxDuplicationPercent:  &maxDup,
			MinCloneTokens:         &minClone,
		},
	}
	p := application.BuildProfile(cfg)

	assert.Equal(t, 20, p.MaxCognitiveComplexity)
	assert.Equal(t, 10, p.MaxDuplicationPercent)
	assert.Equal(t, 80, p.MinCloneTokens)
	// Non-overridden fields keep defaults
	assert.Equal(t, 50, p.MaxFunctionLines)
}

func TestBuildProfile_TypePlusOverride(t *testing.T) {
	maxParams := 6
	cfg := domain.ProjectConfig{
		ProjectType: domain.ProjectTypeLibrary,
		Profile: &domain.ProfileOverrides{
			MaxParameters: &maxParams,
		},
	}
	p := application.BuildProfile(cfg)

	// Library default is 3, but override is 6
	assert.Equal(t, 6, p.MaxParameters)
	// Library defaults preserved for non-overridden fields
	assert.Equal(t, 40, p.MaxFunctionLines)
	assert.Equal(t, []string{"pkg"}, p.ExpectedDirs)
}
