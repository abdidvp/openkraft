package domain_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestProjectCache_IsInvalidated(t *testing.T) {
	cache := &domain.ProjectCache{
		GoModHash:  "abc123",
		ConfigHash: "def456",
	}

	t.Run("same hashes", func(t *testing.T) {
		assert.False(t, cache.IsInvalidated("abc123", "def456"))
	})

	t.Run("different goModHash", func(t *testing.T) {
		assert.True(t, cache.IsInvalidated("changed", "def456"))
	})

	t.Run("different configHash", func(t *testing.T) {
		assert.True(t, cache.IsInvalidated("abc123", "changed"))
	})
}
