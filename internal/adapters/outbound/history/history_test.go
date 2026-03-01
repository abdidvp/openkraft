package history_test

import (
	"path/filepath"
	"testing"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/history"
	"github.com/abdidvp/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	h := history.New()

	entry := domain.ScoreEntry{
		Timestamp:  "2026-02-25T10:00:00Z",
		CommitHash: "abc1234",
		Overall:    47,
		Grade:      "D",
	}

	err := h.Save(dir, entry)
	require.NoError(t, err)

	entries, err := h.Load(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 47, entries[0].Overall)
	assert.Equal(t, "abc1234", entries[0].CommitHash)
}

func TestHistory_AppendMultiple(t *testing.T) {
	dir := t.TempDir()
	h := history.New()

	require.NoError(t, h.Save(dir, domain.ScoreEntry{Timestamp: "t1", Overall: 47, Grade: "D"}))
	require.NoError(t, h.Save(dir, domain.ScoreEntry{Timestamp: "t2", Overall: 62, Grade: "C"}))
	require.NoError(t, h.Save(dir, domain.ScoreEntry{Timestamp: "t3", Overall: 85, Grade: "A"}))

	entries, err := h.Load(dir)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, 47, entries[0].Overall)
	assert.Equal(t, 85, entries[2].Overall)
}

func TestHistory_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	h := history.New()

	entries, err := h.Load(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestHistory_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "deep", "nested")
	h := history.New()

	err := h.Save(nestedDir, domain.ScoreEntry{Timestamp: "t1", Overall: 50, Grade: "D"})
	require.NoError(t, err)

	entries, err := h.Load(nestedDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
