package domain_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDefaultProfile_AllFieldsPopulated(t *testing.T) {
	p := domain.DefaultProfile()

	assert.NotEmpty(t, p.ExpectedLayers, "ExpectedLayers")
	assert.NotEmpty(t, p.ExpectedDirs, "ExpectedDirs")
	assert.NotEmpty(t, p.LayerAliases, "LayerAliases")
	assert.NotEmpty(t, p.ExpectedFileSuffixes, "ExpectedFileSuffixes")
	assert.NotEmpty(t, p.NamingConvention, "NamingConvention")
	assert.Greater(t, p.MaxFunctionLines, 0, "MaxFunctionLines")
	assert.Greater(t, p.MaxFileLines, 0, "MaxFileLines")
	assert.Greater(t, p.MaxNestingDepth, 0, "MaxNestingDepth")
	assert.Greater(t, p.MaxParameters, 0, "MaxParameters")
	assert.Greater(t, p.MaxConditionalOps, 0, "MaxConditionalOps")
	assert.Greater(t, p.MaxCognitiveComplexity, 0, "MaxCognitiveComplexity")
	assert.Greater(t, p.MaxDuplicationPercent, 0, "MaxDuplicationPercent")
	assert.Greater(t, p.MinCloneTokens, 0, "MinCloneTokens")
	assert.NotEmpty(t, p.ContextFiles, "ContextFiles")
	assert.Greater(t, p.MinTestRatio, 0.0, "MinTestRatio")
	assert.Greater(t, p.MaxGlobalVarPenalty, 0, "MaxGlobalVarPenalty")
}

func TestDefaultProfile_LayerAliases(t *testing.T) {
	p := domain.DefaultProfile()

	assert.Equal(t, "adapters", p.LayerAliases["adapter"])
	assert.Equal(t, "adapters", p.LayerAliases["infra"])
	assert.Equal(t, "adapters", p.LayerAliases["infrastructure"])
	assert.Equal(t, "application", p.LayerAliases["app"])
	assert.Equal(t, "application", p.LayerAliases["core"])
}

func TestDefaultProfileForType_API(t *testing.T) {
	p := domain.DefaultProfileForType(domain.ProjectTypeAPI)

	assert.Equal(t, []string{"domain", "application", "adapters"}, p.ExpectedLayers)
	assert.Equal(t, []string{"internal", "cmd"}, p.ExpectedDirs)
	assert.Equal(t, 50, p.MaxFunctionLines)
	assert.Equal(t, 4, p.MaxParameters)
}

func TestDefaultProfileForType_CLI(t *testing.T) {
	p := domain.DefaultProfileForType(domain.ProjectTypeCLI)

	assert.Equal(t, []string{"domain", "application"}, p.ExpectedLayers)
	assert.Equal(t, []string{"internal", "cmd"}, p.ExpectedDirs)
	assert.Equal(t, []string{"_model", "_service"}, p.ExpectedFileSuffixes)
	assert.Equal(t, 50, p.MaxFunctionLines)
}

func TestDefaultProfileForType_Library(t *testing.T) {
	p := domain.DefaultProfileForType(domain.ProjectTypeLibrary)

	assert.Equal(t, []string{"domain"}, p.ExpectedLayers)
	assert.Equal(t, []string{"pkg"}, p.ExpectedDirs)
	assert.Equal(t, 40, p.MaxFunctionLines)
	assert.Equal(t, 250, p.MaxFileLines)
	assert.Equal(t, 3, p.MaxParameters)
	assert.Equal(t, 20, p.MaxCognitiveComplexity, "Library should have stricter CC threshold")
	assert.Equal(t, 0.8, p.MinTestRatio)
}

func TestDefaultProfileForType_Microservice(t *testing.T) {
	p := domain.DefaultProfileForType(domain.ProjectTypeMicroservice)

	assert.Equal(t, []string{"domain", "application", "adapters"}, p.ExpectedLayers)
	assert.Equal(t, []string{"internal", "cmd"}, p.ExpectedDirs)
	assert.Equal(t, 50, p.MaxFunctionLines)
}

func TestDefaultProfileForType_ContextFilesVaryByType(t *testing.T) {
	api := domain.DefaultProfileForType(domain.ProjectTypeAPI)
	cli := domain.DefaultProfileForType(domain.ProjectTypeCLI)
	lib := domain.DefaultProfileForType(domain.ProjectTypeLibrary)

	// API has all 4 context files
	assert.Len(t, api.ContextFiles, 4)
	// CLI has 3 context files (no AGENTS.md)
	assert.Len(t, cli.ContextFiles, 3)
	// Library has 3 context files (no .cursorrules)
	assert.Len(t, lib.ContextFiles, 3)
}

func TestDefaultProfileForType_UnknownTypeReturnsBaseDefaults(t *testing.T) {
	p := domain.DefaultProfileForType("unknown-type")
	base := domain.DefaultProfile()

	assert.Equal(t, base.ExpectedLayers, p.ExpectedLayers)
	assert.Equal(t, base.MaxFunctionLines, p.MaxFunctionLines)
}
