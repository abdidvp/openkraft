package cache

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/openkraft/openkraft/internal/domain"
)

// Store is a file-based implementation of domain.CacheStore.
type Store struct{}

// New creates a new file-based cache store.
func New() *Store {
	return &Store{}
}

// Load reads a project cache from disk. Returns (nil, nil) if no cache exists.
func (s *Store) Load(projectPath string) (*domain.ProjectCache, error) {
	path := cachePath(projectPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no cache is not an error
		}
		return nil, err
	}

	var cache domain.ProjectCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// Save writes a project cache to disk, creating directories as needed.
func (s *Store) Save(cache *domain.ProjectCache) error {
	dir := cacheDir(cache.ProjectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath(cache.ProjectPath), data, 0644)
}

// Invalidate removes the cache file for the given project path.
func (s *Store) Invalidate(projectPath string) error {
	path := cachePath(projectPath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func cacheDir(projectPath string) string {
	return filepath.Join(projectPath, ".openkraft", "cache")
}

func cachePath(projectPath string) string {
	return filepath.Join(projectPath, ".openkraft", "cache", "baseline.json")
}
