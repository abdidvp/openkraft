package scoring_test

import (
	"testing"

	"github.com/abdidvp/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestHasVerbNounPattern_CommonGoVerbs(t *testing.T) {
	passing := []string{
		"ScoreCodeHealth", "AnalyzeFile", "NewScoreService",
		"ComputeOverallScore", "LoadConfig", "SaveResult",
		"CheckModule", "BuildReport", "RenderScore",
		"WriteFile", "ReadConfig", "OpenConnection",
		"StartServer", "StopWorker", "RegisterHandler",
		"DetectModules", "ScanProject", "ExtractErrors",
		"FilterFiles", "SortResults", "CollectIssues",
		"ResolveImport", "SetWeight", "AddCategory",
		"RemoveSkipped", "ApplyConfig", "FormatOutput",
		"HandleRequest", "ProcessEvent", "RunPipeline",
		"IsValid", "HasPrefix", "MarshalJSON",
	}
	for _, name := range passing {
		assert.True(t, scoring.HasVerbNounPattern(name), "%s should match verb+noun", name)
	}
}

func TestHasVerbNounPattern_Rejects(t *testing.T) {
	failing := []string{
		"String", // single CamelCase word
		"Error",  // single word
		"Len",    // single word
		"",       // empty
		"lower",  // unexported
		"x",      // unexported single char
	}
	for _, name := range failing {
		assert.False(t, scoring.HasVerbNounPattern(name), "%s should NOT match", name)
	}
}

func TestWordCountScore(t *testing.T) {
	assert.Equal(t, 1.0, scoring.WordCountScore("CreateUser"))               // 2 words
	assert.Equal(t, 1.0, scoring.WordCountScore("ScoreCodeHealth"))          // 3 words
	assert.Equal(t, 0.5, scoring.WordCountScore("Score"))                    // 1 word
	assert.Equal(t, 0.7, scoring.WordCountScore("VeryLongFunctionNameHere")) // 5 words
}

func TestVocabularySpecificity(t *testing.T) {
	// "Handle" is vague, "User" is not.
	assert.Less(t, scoring.VocabularySpecificity("HandleData"), 1.0)
	// "Score" and "Project" are not vague.
	assert.Equal(t, 1.0, scoring.VocabularySpecificity("ScoreProject"))
}

func TestShannonEntropy(t *testing.T) {
	// All unique names → high entropy.
	unique := []string{"CreateUser", "DeleteUser", "UpdateUser", "ListUsers"}
	assert.Greater(t, scoring.ShannonEntropy(unique), 0.8)

	// All same → zero entropy.
	same := []string{"Foo", "Foo", "Foo"}
	assert.Equal(t, 0.0, scoring.ShannonEntropy(same))

	// Single name → zero.
	assert.Equal(t, 0.0, scoring.ShannonEntropy([]string{"One"}))
}
