package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const aiContextFixtureDir = "../../../testdata/go-hexagonal/perfect"

func TestScoreAIContext_PerfectFixture(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	assert.Equal(t, "ai_context", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Len(t, result.SubMetrics, 4)

	// Perfect fixture has CLAUDE.md (875 bytes, ## headers) and .cursorrules (301 bytes, actionable).
	// No AGENTS.md, no .openkraft/ directory.
	// Expected: 25 + 25 + 0 + 0 = 50
	assert.Equal(t, 50, result.Score, "CLAUDE.md (25) + .cursorrules (25) + no AGENTS.md (0) + no .openkraft (0)")
}

func TestScoreAIContext_SubMetricNames(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	names := make([]string, len(result.SubMetrics))
	for i, sm := range result.SubMetrics {
		names[i] = sm.Name
	}

	assert.Contains(t, names, "claude_md")
	assert.Contains(t, names, "cursor_rules")
	assert.Contains(t, names, "agents_md")
	assert.Contains(t, names, "openkraft_dir")
}

func TestScoreAIContext_SubMetricMaxPoints(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	pointsMap := make(map[string]int)
	for _, sm := range result.SubMetrics {
		pointsMap[sm.Name] = sm.Points
	}

	assert.Equal(t, 25, pointsMap["claude_md"])
	assert.Equal(t, 25, pointsMap["cursor_rules"])
	assert.Equal(t, 25, pointsMap["agents_md"])
	assert.Equal(t, 25, pointsMap["openkraft_dir"])
}

func TestScoreAIContext_ClaudeMD(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	for _, sm := range result.SubMetrics {
		if sm.Name == "claude_md" {
			// CLAUDE.md exists (10) + >500 bytes (10) + has ## headers (5) = 25
			assert.Equal(t, 25, sm.Score)
			return
		}
	}
	t.Fatal("claude_md sub-metric not found")
}

func TestScoreAIContext_CursorRules(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	for _, sm := range result.SubMetrics {
		if sm.Name == "cursor_rules" {
			// .cursorrules exists (10) + >200 bytes (10) + has actionable content (5) = 25
			assert.Equal(t, 25, sm.Score)
			return
		}
	}
	t.Fatal("cursor_rules sub-metric not found")
}

func TestScoreAIContext_AgentsMDMissing(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	for _, sm := range result.SubMetrics {
		if sm.Name == "agents_md" {
			assert.Equal(t, 0, sm.Score, "no AGENTS.md in perfect fixture")
			return
		}
	}
	t.Fatal("agents_md sub-metric not found")
}

func TestScoreAIContext_OpenkraftDirMissing(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	for _, sm := range result.SubMetrics {
		if sm.Name == "openkraft_dir" {
			assert.Equal(t, 0, sm.Score, "no .openkraft/ in perfect fixture")
			return
		}
	}
	t.Fatal("openkraft_dir sub-metric not found")
}

func TestScoreAIContext_NilScan(t *testing.T) {
	result := scoring.ScoreAIContext(nil)

	assert.Equal(t, "ai_context", result.Name)
	assert.Equal(t, 0.10, result.Weight)
	assert.Equal(t, 0, result.Score)
}

func TestScoreAIContext_ScoreDoesNotExceed100(t *testing.T) {
	scan, err := scanner.New().Scan(aiContextFixtureDir)
	require.NoError(t, err)

	result := scoring.ScoreAIContext(scan)

	assert.LessOrEqual(t, result.Score, 100)

	for _, sm := range result.SubMetrics {
		assert.LessOrEqual(t, sm.Score, sm.Points,
			"sub-metric %s score should not exceed max points", sm.Name)
	}
}
