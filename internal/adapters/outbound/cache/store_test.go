package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/cache"
	"github.com/abdidvp/openkraft/internal/domain"
)

func TestStore_SaveAndLoad(t *testing.T) {
	store := cache.New()
	projectPath := t.TempDir()

	original := &domain.ProjectCache{
		ProjectPath: projectPath,
		ConfigHash:  "abc123",
		GoModHash:   "def456",
	}

	err := store.Save(original)
	require.NoError(t, err)

	loaded, err := store.Load(projectPath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, original.ProjectPath, loaded.ProjectPath)
	assert.Equal(t, original.ConfigHash, loaded.ConfigHash)
	assert.Equal(t, original.GoModHash, loaded.GoModHash)
	assert.Nil(t, loaded.ScanResult)
	assert.Nil(t, loaded.AnalyzedFiles)
	assert.Nil(t, loaded.Modules)
	assert.Nil(t, loaded.BaselineScore)
}

func TestStore_LoadNonExistent(t *testing.T) {
	store := cache.New()
	projectPath := t.TempDir()

	loaded, err := store.Load(projectPath)
	assert.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestStore_Invalidate(t *testing.T) {
	store := cache.New()
	projectPath := t.TempDir()

	original := &domain.ProjectCache{
		ProjectPath: projectPath,
		ConfigHash:  "abc123",
		GoModHash:   "def456",
	}

	err := store.Save(original)
	require.NoError(t, err)

	err = store.Invalidate(projectPath)
	require.NoError(t, err)

	loaded, err := store.Load(projectPath)
	assert.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestStore_SaveCreatesDirectory(t *testing.T) {
	store := cache.New()
	projectPath := t.TempDir()

	cacheDir := filepath.Join(projectPath, ".openkraft", "cache")
	_, err := os.Stat(cacheDir)
	require.True(t, os.IsNotExist(err), "cache directory should not exist before save")

	c := &domain.ProjectCache{
		ProjectPath: projectPath,
		ConfigHash:  "hash1",
		GoModHash:   "hash2",
	}

	err = store.Save(c)
	require.NoError(t, err)

	info, err := os.Stat(cacheDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
