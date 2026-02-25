package scanner_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../../../testdata/go-hexagonal/perfect"

func TestFileScanner_Scan(t *testing.T) {
	s := scanner.New()
	result, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	assert.Equal(t, "go", result.Language)
	assert.True(t, len(result.GoFiles) > 0, "should find Go files")
	assert.True(t, len(result.TestFiles) > 0, "should find test files")
	assert.True(t, result.HasGoMod, "should detect go.mod")
}

func TestFileScanner_DetectsAIContextFiles(t *testing.T) {
	s := scanner.New()
	result, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	assert.True(t, result.HasClaudeMD, "should detect CLAUDE.md")
	assert.True(t, result.HasCursorRules, "should detect .cursorrules")
}

func TestFileScanner_ExcludesVendorAndGit(t *testing.T) {
	s := scanner.New()
	result, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	for _, f := range result.AllFiles {
		assert.NotContains(t, f, "vendor/")
		assert.NotContains(t, f, ".git/")
		assert.NotContains(t, f, "node_modules/")
	}
}

func TestFileScanner_TestFilesAreSubsetOfGoFiles(t *testing.T) {
	s := scanner.New()
	result, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	goFileSet := make(map[string]bool)
	for _, f := range result.GoFiles {
		goFileSet[f] = true
	}
	for _, tf := range result.TestFiles {
		assert.True(t, goFileSet[tf], "test file %s should be in GoFiles", tf)
	}
}

func TestFileScanner_ExcludesTestdata(t *testing.T) {
	// Scan the project root which contains testdata/
	s := scanner.New()
	result, err := s.Scan("../../../..")
	require.NoError(t, err)

	for _, f := range result.GoFiles {
		assert.NotContains(t, f, "testdata/", "should exclude testdata/ from Go files: %s", f)
	}
	for _, f := range result.AllFiles {
		assert.NotContains(t, f, "testdata/", "should exclude testdata/ from all files: %s", f)
	}
}

func TestFileScanner_CustomExcludePaths(t *testing.T) {
	s := scanner.New()
	// The perfect fixture has internal/ — exclude it
	result, err := s.Scan(fixtureDir, "internal")
	require.NoError(t, err)

	for _, f := range result.GoFiles {
		assert.NotContains(t, f, "internal/", "should exclude internal/ via custom exclude: %s", f)
	}
}

func TestFileScanner_PopulatesFileMetadata(t *testing.T) {
	s := scanner.New()
	result, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	// Perfect fixture has CLAUDE.md and .cursorrules.
	assert.Greater(t, result.ClaudeMDSize, 0, "should read CLAUDE.md size")
	assert.NotEmpty(t, result.ClaudeMDContent, "should read CLAUDE.md content")
	assert.Greater(t, result.CursorRulesSize, 0, "should read .cursorrules size")
}

func TestFileScanner_AIContextOnlyFromRoot(t *testing.T) {
	// The perfect fixture has CLAUDE.md and .cursorrules,
	// but when scanning the project root, those should not
	// cause HasClaudeMD=true (they're in testdata/, not root).
	s := scanner.New()
	result, err := s.Scan("../../../..")
	require.NoError(t, err)

	// The project root does NOT have CLAUDE.md or .cursorrules
	assert.False(t, result.HasClaudeMD, "should not detect CLAUDE.md from testdata/")
	assert.False(t, result.HasCursorRules, "should not detect .cursorrules from testdata/")
}
