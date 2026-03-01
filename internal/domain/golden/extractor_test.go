package golden_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/abdidvp/openkraft/internal/domain/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTaxModule(t *testing.T) (domain.DetectedModule, map[string]*domain.AnalyzedFile) {
	t.Helper()

	modules, analyzed := loadFixture(t)

	var taxModule domain.DetectedModule
	for _, m := range modules {
		if m.Name == "tax" {
			taxModule = m
			break
		}
	}
	require.Equal(t, "tax", taxModule.Name, "tax module must be found")

	return taxModule, analyzed
}

func TestExtractBlueprint_HasAllLayers(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)
	require.NotNil(t, bp)

	assert.Equal(t, "tax", bp.Name)
	assert.Equal(t, "internal/tax", bp.ExtractedFrom)
	assert.True(t, len(bp.Files) >= 7, "should have files for all layers, got %d", len(bp.Files))

	// Collect all file types
	types := make(map[string]bool)
	for _, f := range bp.Files {
		types[f.Type] = true
	}
	assert.True(t, types["domain_entity"], "should have domain_entity file type")
	assert.True(t, types["domain_test"], "should have domain_test file type")
	assert.True(t, types["domain_errors"], "should have domain_errors file type")
	assert.True(t, types["service"], "should have service file type")
	assert.True(t, types["ports"], "should have ports file type")
	assert.True(t, types["handler"], "should have handler file type")
	assert.True(t, types["routes"], "should have routes file type")
	assert.True(t, types["repository"], "should have repository file type")
}

func TestExtractBlueprint_DomainEntityHasRequiredStructs(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	var entityFile *domain.BlueprintFile
	for i := range bp.Files {
		if bp.Files[i].Type == "domain_entity" {
			entityFile = &bp.Files[i]
			break
		}
	}
	require.NotNil(t, entityFile, "must have domain_entity file")
	assert.Contains(t, entityFile.RequiredStructs, "{Entity}")
}

func TestExtractBlueprint_DomainEntityHasNewConstructor(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	var entityFile *domain.BlueprintFile
	for i := range bp.Files {
		if bp.Files[i].Type == "domain_entity" {
			entityFile = &bp.Files[i]
			break
		}
	}
	require.NotNil(t, entityFile, "must have domain_entity file")
	assert.Contains(t, entityFile.RequiredFunctions, "New{Entity}")
}

func TestExtractBlueprint_DomainEntityHasValidateMethod(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	var entityFile *domain.BlueprintFile
	for i := range bp.Files {
		if bp.Files[i].Type == "domain_entity" {
			entityFile = &bp.Files[i]
			break
		}
	}
	require.NotNil(t, entityFile, "must have domain_entity file")
	assert.Contains(t, entityFile.RequiredMethods, "Validate")
}

func TestExtractBlueprint_PathPatternsAreGeneralized(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	paths := make(map[string]bool)
	for _, f := range bp.Files {
		paths[f.PathPattern] = true
	}

	assert.True(t, paths["domain/{entity}.go"], "should have domain/{entity}.go, got paths: %v", paths)
	assert.True(t, paths["application/{module}_service.go"], "should have application/{module}_service.go")
	assert.True(t, paths["application/{module}_ports.go"], "should have application/{module}_ports.go")
}

func TestExtractBlueprint_ServiceFileHasStructAndConstructor(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	var serviceFile *domain.BlueprintFile
	for i := range bp.Files {
		if bp.Files[i].Type == "service" {
			serviceFile = &bp.Files[i]
			break
		}
	}
	require.NotNil(t, serviceFile, "must have service file")
	assert.Contains(t, serviceFile.RequiredStructs, "{Entity}Service")
	assert.Contains(t, serviceFile.RequiredFunctions, "New{Entity}Service")
}

func TestExtractBlueprint_PortsFileHasInterface(t *testing.T) {
	taxModule, analyzed := setupTaxModule(t)

	bp, err := golden.ExtractBlueprint(taxModule, analyzed)
	require.NoError(t, err)

	var portsFile *domain.BlueprintFile
	for i := range bp.Files {
		if bp.Files[i].Type == "ports" {
			portsFile = &bp.Files[i]
			break
		}
	}
	require.NotNil(t, portsFile, "must have ports file")
	assert.True(t, len(portsFile.RequiredInterfaces) > 0, "ports file should have interfaces")
}
