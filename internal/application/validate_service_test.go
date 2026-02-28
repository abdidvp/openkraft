package application

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkraft/openkraft/internal/adapters/outbound/cache"
	"github.com/openkraft/openkraft/internal/adapters/outbound/config"
	"github.com/openkraft/openkraft/internal/adapters/outbound/detector"
	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/openkraft/openkraft/internal/adapters/outbound/scanner"
	"github.com/openkraft/openkraft/internal/domain"
)

func newValidateService() *ValidateService {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	cfg := config.New()
	cacheSt := cache.New()
	scoreSvc := NewScoreService(sc, det, par, cfg)
	return NewValidateService(sc, det, par, scoreSvc, cacheSt, cfg)
}

func TestValidate_FirstCallCreatesCache(t *testing.T) {
	svc := newValidateService()
	fixturePath := "../../testdata/go-hexagonal/perfect"

	// Clean up any leftover cache
	_ = cache.New().Invalidate(fixturePath)
	defer func() { _ = cache.New().Invalidate(fixturePath) }()

	result, err := svc.Validate(fixturePath, []string{"internal/tax/domain/tax_rule.go"}, nil, nil, false)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "pass", result.Status)
	assert.Contains(t, result.FilesChecked, "internal/tax/domain/tax_rule.go")
}

func TestValidate_StrictMode(t *testing.T) {
	svc := newValidateService()
	fixturePath := "../../testdata/go-hexagonal/incomplete"

	_ = cache.New().Invalidate(fixturePath)
	defer func() { _ = cache.New().Invalidate(fixturePath) }()

	result, err := svc.Validate(fixturePath, []string{"internal/shipping/application/shipping_service.go"}, nil, nil, true)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// In strict mode, warnings become failures
	if result.Status == "warn" {
		t.Error("strict mode should convert warn to fail")
	}
}

func TestClassifyDrift_NewNamingIssue(t *testing.T) {
	baseline := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "discoverability", Issues: []domain.Issue{}},
		},
	}
	current := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "discoverability", Issues: []domain.Issue{
				{
					Severity:  "warning",
					Category:  "discoverability",
					SubMetric: "file_naming_conventions",
					File:      "payment_service.go",
					Message:   "inconsistent naming",
				},
			}},
		},
	}

	drifts := classifyDrift(baseline, current, domain.ProjectNorms{})
	require.Len(t, drifts, 1)
	assert.Equal(t, "naming_drift", drifts[0].DriftType)
}

func TestClassifyDrift_ExistingIssueNotReported(t *testing.T) {
	issue := domain.Issue{
		Severity: "warning",
		Category: "discoverability",
		File:     "foo.go",
		Message:  "same issue",
	}
	baseline := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "discoverability", Issues: []domain.Issue{issue}},
		},
	}
	current := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "discoverability", Issues: []domain.Issue{issue}},
		},
	}

	drifts := classifyDrift(baseline, current, domain.ProjectNorms{})
	assert.Empty(t, drifts, "existing issues should not be reported as drift")
}

func TestClassifyDrift_DependencyViolation(t *testing.T) {
	baseline := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "discoverability", Issues: []domain.Issue{}},
		},
	}
	current := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "discoverability", Issues: []domain.Issue{
				{
					Severity:  "error",
					Category:  "discoverability",
					SubMetric: "dependency_direction",
					File:      "domain/user.go",
					Message:   "domain imports adapters",
				},
			}},
		},
	}

	drifts := classifyDrift(baseline, current, domain.ProjectNorms{})
	require.Len(t, drifts, 1)
	assert.Equal(t, "dependency_drift", drifts[0].DriftType)
}

func TestClassifyDrift_SizeDrift(t *testing.T) {
	baseline := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "code_health", Issues: []domain.Issue{}},
		},
	}
	current := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "code_health", Issues: []domain.Issue{
				{
					Severity:  "warning",
					Category:  "code_health",
					SubMetric: "function_size",
					File:      "service.go",
					Line:      10,
					Message:   "function too long",
				},
			}},
		},
	}

	drifts := classifyDrift(baseline, current, domain.ProjectNorms{FunctionLines: 50})
	require.Len(t, drifts, 1)
	assert.Equal(t, "size_drift", drifts[0].DriftType)
}

func TestClassifyDrift_NilBaseline(t *testing.T) {
	current := &domain.Score{
		Categories: []domain.CategoryScore{
			{Name: "code_health", Issues: []domain.Issue{
				{Severity: "warning", Message: "test"},
			}},
		},
	}

	drifts := classifyDrift(nil, current, domain.ProjectNorms{})
	assert.Nil(t, drifts, "nil baseline should produce no drift")
}

func TestScoreWithData(t *testing.T) {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	cfgLoader := config.New()
	svc := NewScoreService(sc, det, par, cfgLoader)

	score1, err := svc.ScoreProject("../../testdata/go-hexagonal/perfect")
	require.NoError(t, err)

	cfg, _ := cfgLoader.Load("../../testdata/go-hexagonal/perfect")
	scan, _ := sc.Scan("../../testdata/go-hexagonal/perfect", cfg.ExcludePaths...)
	modules, _ := det.Detect(scan)
	analyzed := make(map[string]*domain.AnalyzedFile)
	for _, f := range scan.GoFiles {
		af, err := par.AnalyzeFile(filepath.Join(scan.RootPath, f))
		if err != nil {
			continue
		}
		af.Path = f
		analyzed[f] = af
	}
	profile := BuildProfile(cfg)
	score2 := svc.ScoreWithData(cfg, profile, scan, modules, analyzed)

	assert.Equal(t, score1.Overall, score2.Overall, "ScoreWithData should produce same result")
}

func TestScoreWithData_NilModules(t *testing.T) {
	sc := scanner.New()
	det := detector.New()
	par := parser.New()
	cfgLoader := config.New()
	svc := NewScoreService(sc, det, par, cfgLoader)

	cfg := domain.DefaultConfig()
	profile := domain.DefaultProfile()
	scan := &domain.ScanResult{}

	// Should not panic with nil modules
	score := svc.ScoreWithData(cfg, profile, scan, nil, nil)
	assert.NotNil(t, score)
}
