package domain_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestComputeNorms(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {
			TotalLines: 100,
			Functions: []domain.Function{
				{LineStart: 1, LineEnd: 20, Params: []domain.Param{{Name: "a", Type: "int"}}},
				{LineStart: 22, LineEnd: 51, Params: []domain.Param{{Name: "a", Type: "int"}, {Name: "b", Type: "string"}}},
			},
		},
		"b.go": {
			TotalLines: 200,
			Functions: []domain.Function{
				{LineStart: 1, LineEnd: 50, Params: []domain.Param{{Name: "ctx", Type: "context.Context"}, {Name: "id", Type: "string"}, {Name: "opts", Type: "Options"}}},
			},
		},
	}

	norms := domain.ComputeNorms(analyzed)

	// fileLines: [100, 200] -> p90 index = int(1*0.9)=0 -> sorted[0]=100? No: idx = int(0.9) = 0
	// Actually: len=2, idx = int(float64(2-1)*0.9) = int(0.9) = 0 -> sorted[0] = 100
	// Wait, sorted = [100, 200], idx = int(1 * 0.9) = int(0.9) = 0 -> 100
	// funcLines: [20, 30, 50] -> sorted = [20, 30, 50], idx = int(2*0.9) = int(1.8) = 1 -> 30
	// params: [1, 2, 3] -> sorted = [1, 2, 3], idx = int(2*0.9) = int(1.8) = 1 -> 2

	assert.Equal(t, 30, norms.FunctionLines)
	assert.Equal(t, 100, norms.FileLines)
	assert.Equal(t, 2, norms.Parameters)
}

func TestPercentile90_Empty(t *testing.T) {
	// ComputeNorms with empty analyzed files should yield zero norms.
	norms := domain.ComputeNorms(map[string]*domain.AnalyzedFile{})
	assert.Equal(t, 0, norms.FunctionLines)
	assert.Equal(t, 0, norms.FileLines)
	assert.Equal(t, 0, norms.Parameters)
}

func TestPercentile90_SingleValue(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"a.go": {
			TotalLines: 42,
			Functions: []domain.Function{
				{LineStart: 1, LineEnd: 42, Params: []domain.Param{{Name: "x", Type: "int"}}},
			},
		},
	}

	norms := domain.ComputeNorms(analyzed)
	// Single value: idx = int(0 * 0.9) = 0 -> sorted[0] = 42
	assert.Equal(t, 42, norms.FunctionLines)
	assert.Equal(t, 42, norms.FileLines)
	assert.Equal(t, 1, norms.Parameters)
}

func TestPercentile90_KnownDistribution(t *testing.T) {
	// [1,2,3,4,5,6,7,8,9,10] -> idx = int(9*0.9) = int(8.1) = 8 -> sorted[8] = 9
	analyzed := map[string]*domain.AnalyzedFile{}
	for i := 1; i <= 10; i++ {
		name := string(rune('a'+i-1)) + ".go"
		analyzed[name] = &domain.AnalyzedFile{
			TotalLines: i,
			Functions: []domain.Function{
				{LineStart: 1, LineEnd: i},
			},
		}
	}

	norms := domain.ComputeNorms(analyzed)
	assert.Equal(t, 9, norms.FunctionLines)
	assert.Equal(t, 9, norms.FileLines)
}

func TestPercentile90_Unsorted(t *testing.T) {
	// Same values as KnownDistribution but in scrambled order via map iteration.
	// Maps in Go have non-deterministic iteration, so this inherently tests unsorted input.
	analyzed := map[string]*domain.AnalyzedFile{
		"z.go": {TotalLines: 7, Functions: []domain.Function{{LineStart: 1, LineEnd: 7}}},
		"a.go": {TotalLines: 3, Functions: []domain.Function{{LineStart: 1, LineEnd: 3}}},
		"m.go": {TotalLines: 10, Functions: []domain.Function{{LineStart: 1, LineEnd: 10}}},
		"b.go": {TotalLines: 1, Functions: []domain.Function{{LineStart: 1, LineEnd: 1}}},
		"x.go": {TotalLines: 5, Functions: []domain.Function{{LineStart: 1, LineEnd: 5}}},
		"c.go": {TotalLines: 9, Functions: []domain.Function{{LineStart: 1, LineEnd: 9}}},
		"d.go": {TotalLines: 2, Functions: []domain.Function{{LineStart: 1, LineEnd: 2}}},
		"e.go": {TotalLines: 8, Functions: []domain.Function{{LineStart: 1, LineEnd: 8}}},
		"f.go": {TotalLines: 4, Functions: []domain.Function{{LineStart: 1, LineEnd: 4}}},
		"g.go": {TotalLines: 6, Functions: []domain.Function{{LineStart: 1, LineEnd: 6}}},
	}

	norms := domain.ComputeNorms(analyzed)
	assert.Equal(t, 9, norms.FunctionLines)
	assert.Equal(t, 9, norms.FileLines)
}
