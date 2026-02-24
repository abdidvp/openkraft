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
