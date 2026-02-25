package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreStructure_NilInputs(t *testing.T) {
	result := scoring.ScoreStructure(nil, nil, nil)

	assert.Equal(t, "structure", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.GreaterOrEqual(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)
}

func TestScoreStructure_EmptyInputs(t *testing.T) {
	modules := []domain.DetectedModule{}
	scan := &domain.ScanResult{}
	analyzed := map[string]*domain.AnalyzedFile{}

	result := scoring.ScoreStructure(modules, scan, analyzed)

	assert.Equal(t, "structure", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Equal(t, 0, result.Score)
}

func TestScoreStructure_WellStructuredProject(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:   "user",
			Path:   "internal/user",
			Layers: []string{"domain", "application", "adapters"},
			Files: []string{
				"internal/user/domain/user_model.go",
				"internal/user/domain/user_ports.go",
				"internal/user/application/user_service.go",
				"internal/user/adapters/user_handler.go",
			},
		},
		{
			Name:   "order",
			Path:   "internal/order",
			Layers: []string{"domain", "application", "adapters"},
			Files: []string{
				"internal/order/domain/order_model.go",
				"internal/order/domain/order_ports.go",
				"internal/order/application/order_service.go",
			},
		},
	}
	scan := &domain.ScanResult{
		AllFiles: []string{
			"internal/user/domain/user_model.go",
			"internal/user/domain/user_ports.go",
			"internal/user/application/user_service.go",
			"internal/user/adapters/user_handler.go",
			"internal/order/domain/order_model.go",
			"internal/order/domain/order_ports.go",
			"internal/order/application/order_service.go",
			"cmd/main.go",
		},
	}
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/user/domain/user_ports.go": {
			Path:       "internal/user/domain/user_ports.go",
			Package:    "domain",
			Interfaces: []string{"UserRepository", "UserService"},
		},
		"internal/order/domain/order_ports.go": {
			Path:       "internal/order/domain/order_ports.go",
			Package:    "domain",
			Interfaces: []string{"OrderRepository"},
		},
	}

	result := scoring.ScoreStructure(modules, scan, analyzed)

	assert.Equal(t, "structure", result.Name)
	assert.Equal(t, 0.15, result.Weight)
	assert.Len(t, result.SubMetrics, 4)
	assert.Greater(t, result.Score, 0)
	assert.LessOrEqual(t, result.Score, 100)

	expectedNames := []string{
		"expected_layers", "expected_files",
		"interface_contracts", "module_completeness",
	}
	for i, name := range expectedNames {
		assert.Equal(t, name, result.SubMetrics[i].Name)
	}
}

func TestScoreStructure_SubMetricPointsSum(t *testing.T) {
	result := scoring.ScoreStructure(nil, nil, nil)

	totalPoints := 0
	for _, sm := range result.SubMetrics {
		totalPoints += sm.Points
	}
	assert.Equal(t, 100, totalPoints, "sub-metric points should sum to 100")
}

func TestScoreStructure_NoModulesGeneratesIssue(t *testing.T) {
	result := scoring.ScoreStructure(nil, &domain.ScanResult{}, nil)

	assert.NotEmpty(t, result.Issues)
	found := false
	for _, issue := range result.Issues {
		if issue.Category == "structure" {
			found = true
		}
	}
	assert.True(t, found, "expected a structure issue when no modules detected")
}

func TestScoreStructure_CrossCuttingLayoutFullLayers(t *testing.T) {
	modules := []domain.DetectedModule{
		{Name: "scoring", Layers: []string{"domain"}},
		{Name: "scanner", Layers: []string{"adapters"}},
	}
	scan := &domain.ScanResult{
		Layout: domain.LayoutCrossCutting,
		AllFiles: []string{
			"cmd/openkraft/main.go",
			"internal/domain/ports.go",
			"internal/application/score_service.go",
			"internal/adapters/outbound/scanner/scanner.go",
		},
	}
	result := scoring.ScoreStructure(modules, scan, nil)
	layers := result.SubMetrics[0]
	assert.Equal(t, "expected_layers", layers.Name)
	assert.Equal(t, 25, layers.Score, "all 5 items found: internal/, cmd/, domain, application, adapters")
}

func TestScoreStructure_InterfaceSatisfaction(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/domain/ports.go": {
			Path: "internal/domain/ports.go", Package: "domain",
			InterfaceDefs: []domain.InterfaceDef{
				{Name: "UserRepo", Methods: []string{"Save", "FindByID", "Delete"}},
				{Name: "EventPub", Methods: []string{"Publish"}},
			},
		},
		"internal/adapters/outbound/pg/repo.go": {
			Path: "internal/adapters/outbound/pg/repo.go", Package: "pg",
			Functions: []domain.Function{
				{Name: "Save", Receiver: "*PgRepo", Exported: true},
				{Name: "FindByID", Receiver: "*PgRepo", Exported: true},
				{Name: "Delete", Receiver: "*PgRepo", Exported: true},
			},
		},
		// EventPub NOT implemented.
	}
	modules := []domain.DetectedModule{{Name: "app"}}
	result := scoring.ScoreStructure(modules, &domain.ScanResult{}, analyzed)
	contracts := result.SubMetrics[2]
	assert.Equal(t, "interface_contracts", contracts.Name)
	assert.Equal(t, 12, contracts.Score, "1/2 satisfied = 50% = 12/25")
}

func TestScoreStructure_ModuleCompletenessComparesWithinLayer(t *testing.T) {
	modules := []domain.DetectedModule{
		{Name: "scoring", Layers: []string{"domain"}, Files: []string{"a.go", "b.go", "c.go", "d.go"}},
		{Name: "check", Layers: []string{"domain"}, Files: []string{"a.go", "b.go"}},
		{Name: "scanner", Layers: []string{"adapters"}, Files: []string{"x.go"}},
		{Name: "parser", Layers: []string{"adapters"}, Files: []string{"y.go", "z.go"}},
	}
	result := scoring.ScoreStructure(modules, &domain.ScanResult{Layout: "cross-cutting"}, nil)
	completeness := result.SubMetrics[3]
	assert.Equal(t, "module_completeness", completeness.Name)
	assert.GreaterOrEqual(t, completeness.Score, 15)
	assert.LessOrEqual(t, completeness.Score, 20)
}

func TestScoreStructure_SingleModuleGetsFullCompleteness(t *testing.T) {
	modules := []domain.DetectedModule{
		{
			Name:  "single",
			Path:  "internal/single",
			Files: []string{"internal/single/main.go"},
		},
	}

	result := scoring.ScoreStructure(modules, &domain.ScanResult{}, nil)

	// module_completeness should be full for single module.
	completeness := result.SubMetrics[3]
	assert.Equal(t, "module_completeness", completeness.Name)
	assert.Equal(t, completeness.Points, completeness.Score)
}
