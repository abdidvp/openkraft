package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build binary before running tests
	dir, err := os.MkdirTemp("", "openkraft-e2e")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	binaryPath = filepath.Join(dir, "openkraft")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/openkraft")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	os.Exit(m.Run())
}

func fixturePath(name string) string {
	abs, _ := filepath.Abs(filepath.Join("../../testdata/go-hexagonal", name))
	return abs
}

func run(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return string(out), exitCode
}

// --- Score Tests ---

func TestE2E_Score(t *testing.T) {
	out, code := run(t, "score", fixturePath("perfect"))
	defer os.RemoveAll(filepath.Join(fixturePath("perfect"), ".openkraft"))
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "OpenKraft")
	assert.Contains(t, out, "100")
}

func TestE2E_ScoreJSON(t *testing.T) {
	out, code := run(t, "score", fixturePath("perfect"), "--json")
	defer os.RemoveAll(filepath.Join(fixturePath("perfect"), ".openkraft"))
	assert.Equal(t, 0, code)

	var score domain.Score
	err := json.Unmarshal([]byte(out), &score)
	require.NoError(t, err)
	assert.Len(t, score.Categories, 6, "should have 6 categories")
	assert.True(t, score.Overall > 0, "overall should be positive")
	assert.True(t, score.Overall <= 100, "overall should not exceed 100")
}

func TestE2E_ScoreCI(t *testing.T) {
	_, code := run(t, "score", fixturePath("perfect"), "--ci", "--min", "999")
	defer os.RemoveAll(filepath.Join(fixturePath("perfect"), ".openkraft"))
	assert.Equal(t, 1, code, "should exit 1 when below minimum")
}

func TestE2E_ScoreOrdering(t *testing.T) {
	perfectOut, _ := run(t, "score", fixturePath("perfect"), "--json")
	defer os.RemoveAll(filepath.Join(fixturePath("perfect"), ".openkraft"))

	incompleteOut, _ := run(t, "score", fixturePath("incomplete"), "--json")
	defer os.RemoveAll(filepath.Join(fixturePath("incomplete"), ".openkraft"))

	emptyOut, _ := run(t, "score", fixturePath("empty"), "--json")
	defer os.RemoveAll(filepath.Join(fixturePath("empty"), ".openkraft"))

	var perfect, incomplete, empty domain.Score
	require.NoError(t, json.Unmarshal([]byte(perfectOut), &perfect))
	require.NoError(t, json.Unmarshal([]byte(incompleteOut), &incomplete))
	require.NoError(t, json.Unmarshal([]byte(emptyOut), &empty))

	assert.Greater(t, perfect.Overall, incomplete.Overall, "perfect > incomplete")
	assert.Greater(t, incomplete.Overall, empty.Overall, "incomplete > empty")
}

// --- Check Tests ---

func TestE2E_CheckModule(t *testing.T) {
	out, code := run(t, "check", "payments", "--path", fixturePath("perfect"))
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "payments")
}

func TestE2E_CheckAll(t *testing.T) {
	out, code := run(t, "check", "--all", "--path", fixturePath("perfect"))
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "payments")
}

func TestE2E_CheckJSON(t *testing.T) {
	out, code := run(t, "check", "payments", "--path", fixturePath("perfect"), "--json")
	assert.Equal(t, 0, code)

	var report domain.CheckReport
	err := json.Unmarshal([]byte(out), &report)
	require.NoError(t, err)
	assert.Equal(t, "payments", report.Module)
}

// --- Version Test ---

func TestE2E_Version(t *testing.T) {
	out, code := run(t, "version")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "openkraft")
}
