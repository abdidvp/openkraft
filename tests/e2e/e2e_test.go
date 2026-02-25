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
	assert.Contains(t, out, "openkraft")
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

	// Verify category names
	catNames := make(map[string]bool)
	for _, c := range score.Categories {
		catNames[c.Name] = true
	}
	assert.True(t, catNames["code_health"])
	assert.True(t, catNames["discoverability"])
	assert.True(t, catNames["structure"])
	assert.True(t, catNames["verifiability"])
	assert.True(t, catNames["context_quality"])
	assert.True(t, catNames["predictability"])
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

	// Ensure meaningful gaps between tiers.
	assert.GreaterOrEqual(t, perfect.Overall-incomplete.Overall, 5, "perfect - incomplete gap >= 5")
	assert.GreaterOrEqual(t, incomplete.Overall-empty.Overall, 5, "incomplete - empty gap >= 5")
}

func TestE2E_ScorePerCategoryOrdering(t *testing.T) {
	perfectOut, _ := run(t, "score", fixturePath("perfect"), "--json")
	defer os.RemoveAll(filepath.Join(fixturePath("perfect"), ".openkraft"))

	incompleteOut, _ := run(t, "score", fixturePath("incomplete"), "--json")
	defer os.RemoveAll(filepath.Join(fixturePath("incomplete"), ".openkraft"))

	var perfect, incomplete domain.Score
	require.NoError(t, json.Unmarshal([]byte(perfectOut), &perfect))
	require.NoError(t, json.Unmarshal([]byte(incompleteOut), &incomplete))

	// Build category maps.
	perfectCats := make(map[string]int)
	for _, c := range perfect.Categories {
		perfectCats[c.Name] = c.Score
	}
	incompleteCats := make(map[string]int)
	for _, c := range incomplete.Categories {
		incompleteCats[c.Name] = c.Score
	}

	// Perfect should beat or tie incomplete on every category.
	for name, pScore := range perfectCats {
		iScore := incompleteCats[name]
		assert.GreaterOrEqual(t, pScore, iScore,
			"category %s: perfect (%d) should be >= incomplete (%d)", name, pScore, iScore)
	}
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

// --- Init Tests ---

func TestE2E_Init(t *testing.T) {
	tmpDir := t.TempDir()
	out, code := run(t, "init", tmpDir, "--type", "cli-tool")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "Created")

	data, err := os.ReadFile(filepath.Join(tmpDir, ".openkraft.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "project_type: cli-tool")
}

func TestE2E_InitAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".openkraft.yaml"), []byte("old"), 0644))

	_, code := run(t, "init", tmpDir)
	assert.Equal(t, 1, code, "should fail when config already exists")
}

// --- Config-Aware Score Tests ---

func TestE2E_ScoreWithCLIConfig(t *testing.T) {
	fp := fixturePath("perfect")
	cfgPath := filepath.Join(fp, ".openkraft.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("project_type: cli-tool\n"), 0644))
	defer os.Remove(cfgPath)
	defer os.RemoveAll(filepath.Join(fp, ".openkraft"))

	out, code := run(t, "score", fp, "--json")
	assert.Equal(t, 0, code)

	var score domain.Score
	require.NoError(t, json.Unmarshal([]byte(out), &score))

	// CLI config should include applied_config
	assert.NotNil(t, score.AppliedConfig)
	assert.Equal(t, "cli-tool", string(score.AppliedConfig.ProjectType))

	// Weights should reflect CLI defaults
	for _, cat := range score.Categories {
		if cat.Name == "discoverability" {
			assert.InDelta(t, 0.20, cat.Weight, 0.001)
		}
		if cat.Name == "structure" {
			assert.InDelta(t, 0.10, cat.Weight, 0.001)
		}
	}
}

func TestE2E_ScoreSkippedCategory(t *testing.T) {
	fp := fixturePath("perfect")
	cfgPath := filepath.Join(fp, ".openkraft.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("skip:\n  categories:\n    - context_quality\n"), 0644))
	defer os.Remove(cfgPath)
	defer os.RemoveAll(filepath.Join(fp, ".openkraft"))

	out, code := run(t, "score", fp, "--json")
	assert.Equal(t, 0, code)

	var score domain.Score
	require.NoError(t, json.Unmarshal([]byte(out), &score))

	assert.Len(t, score.Categories, 5, "should have 5 categories when context_quality is skipped")
	for _, cat := range score.Categories {
		assert.NotEqual(t, "context_quality", cat.Name)
	}
}

// --- Version Test ---

func TestE2E_Version(t *testing.T) {
	out, code := run(t, "version")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "openkraft")
}
