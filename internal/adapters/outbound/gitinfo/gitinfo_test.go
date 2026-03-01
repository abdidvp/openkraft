package gitinfo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/abdidvp/openkraft/internal/adapters/outbound/gitinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitInfo_IsGitRepo_True(t *testing.T) {
	// Create a temp git repo to test
	dir := t.TempDir()
	runGit(t, dir, "init")

	gi := gitinfo.New()
	assert.True(t, gi.IsGitRepo(dir))
}

func TestGitInfo_IsGitRepo_False(t *testing.T) {
	dir := t.TempDir()
	gi := gitinfo.New()
	assert.False(t, gi.IsGitRepo(dir))
}

func TestGitInfo_CommitHash_ReturnsHash(t *testing.T) {
	// Create a temp git repo with a commit
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello"), 0644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	gi := gitinfo.New()
	hash, err := gi.CommitHash(dir)
	require.NoError(t, err)
	assert.Len(t, hash, 40, "should be a full SHA-1 hash")
}

func TestGitInfo_CommitHash_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	gi := gitinfo.New()
	_, err := gi.CommitHash(dir)
	assert.Error(t, err)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}
