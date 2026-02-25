# Phase 4: Scoring Calibration — Layout-Agnostic Architecture Analysis

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the scoring pipeline layout-agnostic — correctly scoring both per-feature modules (`internal/payments/domain/`) and cross-cutting layers (`internal/domain/scoring/`) — and add interface satisfaction as the definitive hexagonal architecture signal.

**Architecture:** The root cause is the module detector. It only understands one layout pattern. All downstream scorers inherit its blind spots. We fix the detector to recognize both layouts, then fix the scorers that depend on it. Finally we add the one metric that actually proves hexagonal architecture works: interface satisfaction.

**Tech Stack:** Go, go/ast, hexagonal architecture patterns

---

## Problem Analysis

When openkraft scores itself, the detector produces:

```json
{"name": "adapters", "layers": [], "file_count": 30}
{"name": "application", "layers": [], "file_count": 4}
{"name": "domain", "layers": [], "file_count": 27}
```

Three "modules" with **zero layers each**. This cascades:

| Scorer | What it sees | What it should see |
|--------|-------------|-------------------|
| `expected_layers` | "2/5 layers" — only counts `internal/` and `cmd/` | 5/5 — domain, application, adapters all present |
| `predictable_structure` | Compares domain vs adapters suffixes → 0% | Should compare feature sub-packages within each layer |
| `interface_contracts` | "1/2 modules have interfaces" | Should check that domain ports have adapter implementations |
| `module_completeness` | Compares domain file count vs adapters → 52% | Should compare feature sub-packages against each other |
| `file_naming_conventions` | 32/63 with narrow suffix list | Should derive conventions from the project itself |

---

## Layout Detection Strategy

There are two canonical hexagonal Go layouts:

**Layout A: Per-feature modules** (what our fixture uses)
```
internal/
  payments/
    domain/      ← module "payments", layer "domain"
    application/ ← module "payments", layer "application"
    adapters/    ← module "payments", layer "adapters"
  inventory/
    domain/
    application/
    adapters/
```

**Layout B: Cross-cutting layers** (what openkraft itself uses)
```
internal/
  domain/          ← layer "domain"
    scoring/       ← feature sub-package
    check/         ← feature sub-package
    golden/        ← feature sub-package
  application/     ← layer "application"
  adapters/        ← layer "adapters"
    inbound/cli/   ← adapter sub-package
    outbound/scanner/
```

**Detection heuristic:** If the second segment after `internal/` is itself a known layer name (`domain`, `application`, `adapters`), it's Layout B. Otherwise it's Layout A.

For Layout B, "modules" are the feature sub-packages *within* each layer (e.g., `scoring`, `check`, `golden` inside `domain/`). The layers are the top-level segments themselves.

---

## Task 1: Teach the detector to recognize both layouts

This is the foundation. Everything else depends on it.

**Files:**
- Modify: `internal/adapters/outbound/detector/detector.go`
- Modify: `internal/adapters/outbound/detector/detector_test.go`
- Modify: `internal/domain/ports.go` — add `Layout` field to `ScanResult`

### Step 1: Add Layout field to ScanResult

In `internal/domain/ports.go`, add a `Layout` type and field:

```go
// ArchLayout describes the project's architectural layout.
type ArchLayout string

const (
	LayoutPerFeature   ArchLayout = "per-feature"   // internal/{feature}/{layer}/
	LayoutCrossCutting ArchLayout = "cross-cutting"  // internal/{layer}/{feature}/
)
```

Add to `ScanResult`:

```go
Layout ArchLayout `json:"layout"`
```

### Step 2: Rewrite detector to handle both layouts

Replace `parseModulePath` with a two-phase approach:

1. **Phase 1 — Classify layout:** Scan all `internal/` paths. If the majority of second-level segments are known layer names (`domain`, `application`, `adapters`), it's cross-cutting. Otherwise, per-feature.

2. **Phase 2 — Extract modules:**
   - **Per-feature:** Same as today — `internal/{module}/{layer}/file.go`
   - **Cross-cutting:** Module = sub-package within a layer — `internal/{layer}/{subpkg}/file.go`. Files directly under a layer (e.g., `internal/domain/ports.go`) belong to a special module named after the layer itself.

The full `Detect` rewrite:

```go
func (d *ModuleDetector) Detect(scan *domain.ScanResult) ([]domain.DetectedModule, error) {
	layout := detectLayout(scan.GoFiles)
	scan.Layout = layout

	switch layout {
	case domain.LayoutCrossCutting:
		return detectCrossCutting(scan.GoFiles)
	default:
		return detectPerFeature(scan.GoFiles)
	}
}

// detectLayout classifies the project layout by inspecting second-level
// segments under internal/. If most are known layer names, it's cross-cutting.
func detectLayout(goFiles []string) domain.ArchLayout {
	layerNames := map[string]bool{
		"domain": true, "application": true, "adapters": true,
		"adapter": true, "app": true, "core": true, "ports": true,
		"infrastructure": true, "infra": true,
	}

	secondSegments := map[string]bool{}
	for _, f := range goFiles {
		parts := strings.Split(filepath.ToSlash(f), "/")
		idx := sliceIndex(parts, "internal")
		if idx == -1 || idx+1 >= len(parts) {
			continue
		}
		secondSegments[parts[idx+1]] = true
	}

	if len(secondSegments) == 0 {
		return domain.LayoutPerFeature
	}

	layerCount := 0
	for seg := range secondSegments {
		if layerNames[seg] {
			layerCount++
		}
	}

	// If majority of second-level segments are layer names → cross-cutting.
	if float64(layerCount)/float64(len(secondSegments)) >= 0.5 {
		return domain.LayoutCrossCutting
	}
	return domain.LayoutPerFeature
}

func sliceIndex(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}
```

**`detectPerFeature`** — same logic as current `Detect`, extracted into a function.

**`detectCrossCutting`** — new function:

```go
// detectCrossCutting handles internal/{layer}/{subpkg}/file.go layouts.
// Modules are the sub-packages within each layer. Each module gets assigned
// the layers it appears in.
func detectCrossCutting(goFiles []string) ([]domain.DetectedModule, error) {
	// Map: feature name → {layers, files}
	type moduleBuilder struct {
		layers map[string]bool
		files  []string
	}
	builders := map[string]*moduleBuilder{}

	for _, f := range goFiles {
		parts := strings.Split(filepath.ToSlash(f), "/")
		idx := sliceIndex(parts, "internal")
		if idx == -1 || idx+1 >= len(parts) {
			continue
		}

		layer := parts[idx+1]                           // "domain", "application", "adapters"
		remaining := parts[idx+2:]                       // everything after the layer

		// Determine feature name: first sub-directory under the layer,
		// or the layer itself for files directly under it.
		featureName := layer // default: files like internal/domain/ports.go
		if len(remaining) >= 2 {
			// Has at least one sub-directory: internal/domain/scoring/code_health.go
			featureName = remaining[0]
		}

		// For adapters, go one level deeper if inbound/outbound exists.
		// internal/adapters/outbound/scanner/scanner.go → feature="scanner"
		if (layer == "adapters" || layer == "adapter") && len(remaining) >= 3 {
			direction := remaining[0] // "inbound" or "outbound"
			if direction == "inbound" || direction == "outbound" {
				featureName = remaining[1]
			}
		}

		mb, ok := builders[featureName]
		if !ok {
			mb = &moduleBuilder{layers: map[string]bool{}}
			builders[featureName] = mb
		}
		mb.layers[normalizeLayer(layer)] = true
		mb.files = append(mb.files, f)
	}

	modules := make([]domain.DetectedModule, 0, len(builders))
	for name, mb := range builders {
		layers := make([]string, 0, len(mb.layers))
		for l := range mb.layers {
			layers = append(layers, l)
		}
		sort.Strings(layers)
		modules = append(modules, domain.DetectedModule{
			Name:   name,
			Path:   "internal/" + name,
			Layers: layers,
			Files:  mb.files,
		})
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})
	return modules, nil
}

func normalizeLayer(raw string) string {
	switch raw {
	case "adapter":
		return "adapters"
	case "app", "core":
		return "application"
	case "infra", "infrastructure":
		return "adapters"
	default:
		return raw
	}
}
```

### Step 3: Write tests for the detector

Add to `detector_test.go`:

```go
func TestModuleDetector_CrossCuttingLayout(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"internal/domain/ports.go",
			"internal/domain/model.go",
			"internal/domain/scoring/code_health.go",
			"internal/domain/scoring/structure.go",
			"internal/domain/check/checker.go",
			"internal/application/score_service.go",
			"internal/application/check_service.go",
			"internal/adapters/outbound/scanner/scanner.go",
			"internal/adapters/outbound/parser/go_parser.go",
			"internal/adapters/inbound/cli/root.go",
			"internal/adapters/inbound/cli/score.go",
		},
	}

	d := detector.New()
	modules, err := d.Detect(scan)
	require.NoError(t, err)

	// Should detect cross-cutting layout.
	assert.Equal(t, domain.LayoutCrossCutting, scan.Layout)

	// Should find feature sub-packages as modules.
	names := map[string]bool{}
	for _, m := range modules {
		names[m.Name] = true
	}
	assert.True(t, names["scoring"], "should detect 'scoring' module")
	assert.True(t, names["check"], "should detect 'check' module")
	assert.True(t, names["scanner"], "should detect 'scanner' module")
	assert.True(t, names["cli"], "should detect 'cli' module")

	// Scoring module should have the "domain" layer.
	for _, m := range modules {
		if m.Name == "scoring" {
			assert.Contains(t, m.Layers, "domain")
		}
	}
}

func TestModuleDetector_PerFeatureLayout(t *testing.T) {
	// The perfect fixture uses per-feature layout — existing tests cover this.
	s := scanner.New()
	scanResult, err := s.Scan(fixtureDir)
	require.NoError(t, err)

	d := detector.New()
	_, err = d.Detect(scanResult)
	require.NoError(t, err)

	assert.Equal(t, domain.LayoutPerFeature, scanResult.Layout)
}
```

### Step 4: Verify

```bash
go test ./internal/adapters/outbound/detector/ -v -count=1
go test ./... -race -count=1
```

---

## Task 2: Fix `scoreExpectedLayers` for cross-cutting layouts

Currently checks `expectedDirs` + `expectedLayers` as a flat count. For cross-cutting layouts, the layers ARE the top-level dirs, so detecting them is different.

**Files:**
- Modify: `internal/domain/scoring/structure.go` — `scoreExpectedLayers`
- Modify: `internal/domain/scoring/structure_test.go`

### Step 1: Pass `ScanResult.Layout` into the scorer

The `scoreExpectedLayers` already receives `scan *domain.ScanResult`. After Task 1, `scan.Layout` will be set. Use it:

```go
func scoreExpectedLayers(modules []domain.DetectedModule, scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "expected_layers", Points: 25}

	if scan == nil {
		sm.Detail = "no scan data"
		return sm
	}

	// Check top-level directories.
	hasInternal := false
	hasCmd := false
	for _, f := range scan.AllFiles {
		if strings.HasPrefix(f, "internal/") {
			hasInternal = true
		}
		if strings.HasPrefix(f, "cmd/") {
			hasCmd = true
		}
	}

	// Check architectural layers — method depends on layout.
	expectedLayers := []string{"domain", "application", "adapters"}
	layerFound := make(map[string]bool)

	if scan.Layout == domain.LayoutCrossCutting {
		// In cross-cutting, layers are top-level dirs under internal/.
		for _, f := range scan.AllFiles {
			if !strings.HasPrefix(f, "internal/") {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(f, "internal/"), "/", 2)
			if len(parts) > 0 {
				seg := parts[0]
				normalized := normalizeLayerName(seg)
				layerFound[normalized] = true
			}
		}
	} else {
		// In per-feature, layers are nested under each module.
		for _, m := range modules {
			for _, l := range m.Layers {
				normalized := normalizeLayerName(l)
				layerFound[normalized] = true
			}
		}
	}

	found := 0
	if hasInternal { found++ }
	if hasCmd { found++ }
	for _, l := range expectedLayers {
		if layerFound[l] { found++ }
	}
	total := 2 + len(expectedLayers) // internal/ + cmd/ + 3 layers = 5

	ratio := float64(found) / float64(total)
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d expected directories/layers present", found, total)
	return sm
}

func normalizeLayerName(name string) string {
	switch name {
	case "adapter", "infra", "infrastructure":
		return "adapters"
	case "app", "core":
		return "application"
	default:
		return name
	}
}
```

### Step 2: Add test

```go
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
	assert.Equal(t, 25, layers.Score, "all 5 layers should be found: internal/, cmd/, domain, application, adapters")
}
```

### Step 3: Verify

```bash
go test ./internal/domain/scoring/ -run TestScoreStructure -v -count=1
```

---

## Task 3: Derive file naming conventions from the project itself

The hardcoded suffix list is fundamentally wrong for world-class scoring. Instead, we measure **internal consistency**: does the project follow *its own* naming convention?

**Approach:** Extract the naming pattern from the majority of files, then measure what percentage conform to that majority pattern. A project where 90% of files use `noun.go` and 10% use `noun_role.go` is consistent. A project where it's 50/50 is inconsistent.

**Files:**
- Modify: `internal/domain/scoring/discoverability.go` — `scoreFileNamingConventions`
- Modify: `internal/domain/scoring/discoverability_test.go`

### Step 1: Classify each Go file into a naming pattern

```go
// fileNamePattern classifies a Go filename into its naming pattern.
type fileNamePattern int

const (
	patternBare       fileNamePattern = iota // "scanner.go", "model.go"
	patternSuffixed                          // "user_handler.go", "tax_service.go"
	patternPrefixed                          // not common in Go
	patternMain                              // "main.go"
)

func classifyFileName(name string) fileNamePattern {
	name = strings.TrimSuffix(name, ".go")
	name = strings.TrimSuffix(name, "_test")
	if name == "main" {
		return patternMain
	}
	if strings.Contains(name, "_") {
		return patternSuffixed
	}
	return patternBare
}
```

### Step 2: Rewrite `scoreFileNamingConventions`

```go
// scoreFileNamingConventions (25 pts): measures internal naming consistency.
// Instead of checking against a hardcoded suffix list, detects the project's
// dominant naming pattern and scores conformance to it.
func scoreFileNamingConventions(scan *domain.ScanResult) domain.SubMetric {
	sm := domain.SubMetric{Name: "file_naming_conventions", Points: 25}

	if scan == nil || len(scan.GoFiles) == 0 {
		sm.Detail = "no Go files to evaluate"
		return sm
	}

	// Classify all non-test Go files.
	counts := map[fileNamePattern]int{}
	total := 0
	for _, f := range scan.GoFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		pattern := classifyFileName(base)
		if pattern == patternMain {
			continue // main.go is always valid, don't count
		}
		counts[pattern]++
		total++
	}

	if total == 0 {
		sm.Detail = "no non-test Go files to evaluate"
		return sm
	}

	// Find dominant pattern.
	dominant := patternBare
	dominantCount := 0
	for p, c := range counts {
		if c > dominantCount {
			dominant = p
			dominantCount = c
		}
	}

	// Score: percentage of files conforming to the dominant pattern.
	// Plus a bonus if the naming is snake_case consistent.
	conforming := counts[dominant]
	ratio := float64(conforming) / float64(total)

	// Bonus: check that suffixed files use consistent suffixes.
	if dominant == patternSuffixed {
		ratio = (ratio + suffixConsistencyRatio(scan.GoFiles)) / 2.0
	}

	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}

	patternName := "bare"
	if dominant == patternSuffixed {
		patternName = "suffixed"
	}
	sm.Detail = fmt.Sprintf("%d/%d files follow %s naming pattern (%.0f%% consistency)",
		conforming, total, patternName, ratio*100)
	return sm
}

// suffixConsistencyRatio measures how reusable the suffixes are across files.
// If the same suffixes appear repeatedly (_service, _handler), that's consistent.
func suffixConsistencyRatio(goFiles []string) float64 {
	suffixCounts := map[string]int{}
	total := 0
	for _, f := range goFiles {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if idx := strings.LastIndex(name, "_"); idx >= 0 {
			suffix := name[idx:]
			suffixCounts[suffix]++
			total++
		}
	}
	if total == 0 {
		return 1.0
	}
	// Ratio of suffixes that appear more than once (reused across files).
	reused := 0
	for _, count := range suffixCounts {
		if count > 1 {
			reused += count
		}
	}
	return float64(reused) / float64(total)
}
```

### Step 3: Add tests

```go
func TestScoreDiscoverability_BareNamingConsistency(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"scanner.go", "detector.go", "parser.go", "renderer.go",
			"config.go", "model.go", "ports.go", "helpers.go",
		},
	}

	result := scoring.ScoreDiscoverability(nil, scan, nil)

	fileNaming := result.SubMetrics[1]
	assert.Equal(t, "file_naming_conventions", fileNaming.Name)
	assert.GreaterOrEqual(t, fileNaming.Score, 22,
		"all-bare naming is 100% consistent and should score high")
}

func TestScoreDiscoverability_MixedNamingReducesScore(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"scanner.go", "detector.go",       // bare
			"user_handler.go", "tax_service.go", // suffixed
			"parser.go", "config.go",           // bare
		},
	}

	result := scoring.ScoreDiscoverability(nil, scan, nil)

	fileNaming := result.SubMetrics[1]
	assert.Equal(t, "file_naming_conventions", fileNaming.Name)
	// 4 bare vs 2 suffixed = 66% consistency. Score ≈ 16/25.
	assert.Less(t, fileNaming.Score, 22, "mixed naming should score lower than uniform")
	assert.Greater(t, fileNaming.Score, 10, "but not too low since majority is consistent")
}
```

### Step 4: Verify

```bash
go test ./internal/domain/scoring/ -run TestScoreDiscoverability -v -count=1
```

---

## Task 4: Fix `predictable_structure` to compare same-level modules

Currently compares `domain` vs `adapters` vs `application` — these are architecturally *supposed to be different*. The metric should only compare modules at the same architectural level.

**Files:**
- Modify: `internal/domain/scoring/discoverability.go` — `scorePredictableStructure`
- Modify: `internal/domain/scoring/discoverability_test.go`

### Step 1: Rewrite to filter comparable modules

For per-feature layouts, all modules are at the same level (they're feature modules). For cross-cutting layouts, we should only compare modules within the same layer — but the detector already handles this in Task 1 by making feature sub-packages the modules.

The key change: **only compare module pairs that share at least one layer**. Modules with completely different layers (like a pure-domain module vs a pure-adapter module) are not comparable.

```go
func scorePredictableStructure(modules []domain.DetectedModule) domain.SubMetric {
	sm := domain.SubMetric{Name: "predictable_structure", Points: 25}

	if len(modules) <= 1 {
		if len(modules) == 1 {
			sm.Score = sm.Points
			sm.Detail = "single module, nothing to compare"
		} else {
			sm.Detail = "no modules detected"
		}
		return sm
	}

	// Build per-module data.
	type moduleData struct {
		layers    map[string]bool
		filenames map[string]bool
		fileCount int
	}
	data := make([]moduleData, len(modules))
	for i, m := range modules {
		md := moduleData{
			layers:    make(map[string]bool),
			filenames: make(map[string]bool),
		}
		for _, l := range m.Layers {
			md.layers[l] = true
		}
		for _, f := range m.Files {
			base := filepath.Base(f)
			name := strings.TrimSuffix(base, ".go")
			if strings.HasSuffix(name, "_test") {
				continue
			}
			md.fileCount++
			// Use suffix if available, bare name otherwise.
			if idx := strings.LastIndex(name, "_"); idx >= 0 {
				md.filenames[name[idx:]] = true
			} else {
				md.filenames[name] = true
			}
		}
		data[i] = md
	}

	// Only compare pairs that share at least one layer.
	var totalLayer, totalFilename, totalFileCount float64
	pairs := 0
	for i := 0; i < len(data); i++ {
		for j := i + 1; j < len(data); j++ {
			if !sharesLayer(data[i].layers, data[j].layers) {
				continue
			}
			pairs++
			totalLayer += jaccard(data[i].layers, data[j].layers)
			totalFilename += jaccard(data[i].filenames, data[j].filenames)
			a, b := float64(data[i].fileCount), float64(data[j].fileCount)
			if a > 0 || b > 0 {
				mn, mx := min(a, b), max(a, b)
				totalFileCount += mn / mx
			} else {
				totalFileCount += 1.0
			}
		}
	}

	if pairs == 0 {
		// No comparable pairs — give full credit (modules are all unique layers).
		sm.Score = sm.Points
		sm.Detail = fmt.Sprintf("no comparable module pairs across %d modules", len(modules))
		return sm
	}

	avgLayer := totalLayer / float64(pairs)
	avgFilename := totalFilename / float64(pairs)
	avgFileCount := totalFileCount / float64(pairs)

	composite := avgLayer*0.5 + avgFilename*0.3 + avgFileCount*0.2
	sm.Score = int(composite * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("layers=%.0f%%, names=%.0f%%, size=%.0f%% across %d comparable pairs",
		avgLayer*100, avgFilename*100, avgFileCount*100, pairs)
	return sm
}

func sharesLayer(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}
```

### Step 2: Add test

```go
func TestScoreDiscoverability_IncomparableModulesGetFullCredit(t *testing.T) {
	// Cross-cutting layout: modules are in different layers, not comparable.
	modules := []domain.DetectedModule{
		{Name: "scoring", Layers: []string{"domain"}, Files: []string{"internal/domain/scoring/code_health.go"}},
		{Name: "scanner", Layers: []string{"adapters"}, Files: []string{"internal/adapters/outbound/scanner/scanner.go"}},
		{Name: "cli", Layers: []string{"adapters"}, Files: []string{"internal/adapters/inbound/cli/root.go"}},
	}

	result := scoring.ScoreDiscoverability(modules, &domain.ScanResult{}, nil)

	predictable := result.SubMetrics[2]
	assert.Equal(t, "predictable_structure", predictable.Name)
	// scanner and cli share "adapters" layer and can be compared.
	// scoring has no peer in "domain" — not compared.
	assert.Greater(t, predictable.Score, 0)
}
```

### Step 3: Verify

```bash
go test ./internal/domain/scoring/ -run TestScoreDiscoverability -v -count=1
```

---

## Task 5: Add interface satisfaction metric

This is the most important metric for hexagonal architecture. It answers: **do the adapter implementations actually satisfy the domain port interfaces?**

This requires AST-level analysis: for each interface in `ports.go`, find concrete types that implement all its methods.

**Files:**
- Modify: `internal/domain/ports.go` — add `InterfaceDef` to `AnalyzedFile`
- Modify: `internal/adapters/outbound/parser/go_parser.go` — extract interface method signatures
- Modify: `internal/domain/scoring/structure.go` — replace `scoreInterfaceContracts` with `scoreInterfaceSatisfaction`
- Add tests for each

### Step 1: Add InterfaceDef to domain

In `internal/domain/ports.go`, add:

```go
// InterfaceDef represents an interface with its method signatures.
type InterfaceDef struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"` // method signatures: "MethodName(paramTypes) returnTypes"
}
```

Add to `AnalyzedFile`:

```go
InterfaceDefs []InterfaceDef `json:"interface_defs,omitempty"`
```

### Step 2: Extract interface methods in the parser

In `go_parser.go`, in `processGenDecl`, when processing `*ast.InterfaceType`:

```go
case *ast.InterfaceType:
	result.Interfaces = append(result.Interfaces, s.Name.Name)
	iface := domain.InterfaceDef{Name: s.Name.Name}
	if itype, ok := s.Type.(*ast.InterfaceType); ok && itype.Methods != nil {
		for _, method := range itype.Methods.List {
			if len(method.Names) > 0 {
				iface.Methods = append(iface.Methods, method.Names[0].Name)
			}
		}
	}
	result.InterfaceDefs = append(result.InterfaceDefs, iface)
```

### Step 3: Rewrite `scoreInterfaceContracts` → `scoreInterfaceSatisfaction`

```go
// scoreInterfaceSatisfaction (25 pts): checks that domain port interfaces
// have concrete implementations in the codebase.
func scoreInterfaceSatisfaction(modules []domain.DetectedModule, analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "interface_contracts", Points: 25}

	// Collect all interfaces from domain/application layers.
	var portInterfaces []domain.InterfaceDef
	for _, af := range analyzed {
		if !isDomainFile(af.Path) && !strings.Contains(af.Path, "/application/") {
			continue
		}
		portInterfaces = append(portInterfaces, af.InterfaceDefs...)
	}

	if len(portInterfaces) == 0 {
		sm.Detail = "no port interfaces found"
		return sm
	}

	// Collect all methods-by-receiver from concrete types.
	receiverMethods := map[string]map[string]bool{} // receiver → set of method names
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			if fn.Receiver == "" {
				continue
			}
			recv := strings.TrimPrefix(fn.Receiver, "*")
			if _, ok := receiverMethods[recv]; !ok {
				receiverMethods[recv] = map[string]bool{}
			}
			receiverMethods[recv][fn.Name] = true
		}
	}

	// For each port interface, check if any concrete type implements all methods.
	satisfied := 0
	for _, iface := range portInterfaces {
		if len(iface.Methods) == 0 {
			satisfied++ // empty interface is trivially satisfied
			continue
		}
		for _, methods := range receiverMethods {
			if implementsAll(iface.Methods, methods) {
				satisfied++
				break
			}
		}
	}

	ratio := float64(satisfied) / float64(len(portInterfaces))
	sm.Score = int(ratio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%d/%d port interfaces have implementations", satisfied, len(portInterfaces))
	return sm
}

func implementsAll(required []string, available map[string]bool) bool {
	for _, m := range required {
		if !available[m] {
			return false
		}
	}
	return true
}
```

### Step 4: Add tests

Parser test — verify interface methods are extracted:

```go
func TestGoParser_ExtractsInterfaceMethods(t *testing.T) {
	// Use the perfect fixture's ports file.
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/application/tax_ports.go")
	require.NoError(t, err)

	assert.NotEmpty(t, af.InterfaceDefs, "should extract interface definitions")
	for _, iface := range af.InterfaceDefs {
		assert.NotEmpty(t, iface.Methods, "interface %s should have methods", iface.Name)
	}
}
```

Scoring test — interface satisfaction:

```go
func TestScoreStructure_InterfaceSatisfaction(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"internal/domain/ports.go": {
			Path:    "internal/domain/ports.go",
			Package: "domain",
			InterfaceDefs: []domain.InterfaceDef{
				{Name: "UserRepository", Methods: []string{"Save", "FindByID", "Delete"}},
				{Name: "EventPublisher", Methods: []string{"Publish"}},
			},
		},
		"internal/adapters/outbound/postgres/user_repo.go": {
			Path:    "internal/adapters/outbound/postgres/user_repo.go",
			Package: "postgres",
			Functions: []domain.Function{
				{Name: "Save", Receiver: "*PostgresUserRepo", Exported: true},
				{Name: "FindByID", Receiver: "*PostgresUserRepo", Exported: true},
				{Name: "Delete", Receiver: "*PostgresUserRepo", Exported: true},
			},
		},
		// EventPublisher NOT implemented — should reduce score.
	}

	modules := []domain.DetectedModule{{Name: "app", Files: []string{"internal/domain/ports.go"}}}
	result := scoring.ScoreStructure(modules, &domain.ScanResult{}, analyzed)

	contracts := result.SubMetrics[2]
	assert.Equal(t, "interface_contracts", contracts.Name)
	// 1/2 interfaces satisfied = 50% = 12/25 pts.
	assert.Equal(t, 12, contracts.Score)
	assert.Contains(t, contracts.Detail, "1/2")
}
```

### Step 5: Verify

```bash
go test ./internal/adapters/outbound/parser/ -run TestGoParser_ExtractsInterface -v
go test ./internal/domain/scoring/ -run TestScoreStructure_InterfaceSatisfaction -v
go test ./... -race -count=1
```

---

## Task 6: Fix `scoreModuleCompleteness` to compare feature peers

Currently picks the module with the most files as "golden" and compares all others against it. This is meaningless when modules are architectural layers (domain has 27 files, application has 4 — by design). The fix: use the golden module selector we already have, and only compare modules that share layers.

**Files:**
- Modify: `internal/domain/scoring/structure.go` — `scoreModuleCompleteness`
- Modify: `internal/domain/scoring/structure_test.go`

### Step 1: Rewrite

```go
// scoreModuleCompleteness (25 pts): compare feature modules against the best
// peer that shares at least one layer. Skip if no comparable pairs exist.
func scoreModuleCompleteness(modules []domain.DetectedModule, _ map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "module_completeness", Points: 25}

	if len(modules) <= 1 {
		if len(modules) == 1 {
			sm.Score = sm.Points
			sm.Detail = "single module, nothing to compare"
		} else {
			sm.Detail = "no modules detected"
		}
		return sm
	}

	// Group modules by layer. Only compare modules sharing a layer.
	layerModules := map[string][]int{} // layer → indices of modules
	for i, m := range modules {
		for _, l := range m.Layers {
			layerModules[l] = append(layerModules[l], i)
		}
	}

	// For each layer group with 2+ modules, compute file-count similarity.
	var totalRatio float64
	comparisons := 0
	for _, indices := range layerModules {
		if len(indices) < 2 {
			continue
		}
		// Find max file count in this group.
		maxFiles := 0
		for _, idx := range indices {
			if len(modules[idx].Files) > maxFiles {
				maxFiles = len(modules[idx].Files)
			}
		}
		if maxFiles == 0 {
			continue
		}
		for _, idx := range indices {
			ratio := float64(len(modules[idx].Files)) / float64(maxFiles)
			if ratio > 1.0 {
				ratio = 1.0
			}
			totalRatio += ratio
			comparisons++
		}
	}

	if comparisons == 0 {
		sm.Score = sm.Points
		sm.Detail = "no comparable module pairs for completeness check"
		return sm
	}

	avgRatio := totalRatio / float64(comparisons)
	sm.Score = int(avgRatio * float64(sm.Points))
	if sm.Score > sm.Points {
		sm.Score = sm.Points
	}
	sm.Detail = fmt.Sprintf("%.0f%% average completeness across %d comparable modules", avgRatio*100, comparisons)
	return sm
}
```

### Step 2: Test

```go
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
	// Domain: scoring=4, check=2 → check/scoring = 0.5. scoring/scoring = 1.0. avg = 0.75.
	// Adapters: scanner=1, parser=2 → scanner/parser = 0.5, parser/parser = 1.0. avg = 0.75.
	// Overall: (0.75*2 + 0.75*2) / 4 = 0.75 → 18/25.
	assert.GreaterOrEqual(t, completeness.Score, 15)
	assert.LessOrEqual(t, completeness.Score, 20)
}
```

### Step 3: Verify

```bash
go test ./internal/domain/scoring/ -run TestScoreStructure -v -count=1
```

---

## Execution Order

1. **Task 1** (detector layout detection) — foundation, all others depend on it
2. **Task 2** (expected layers) — uses layout from Task 1
3. **Task 5** (interface satisfaction) — independent, can go early
4. **Task 3** (file naming conventions) — independent
5. **Task 4** (predictable structure) — uses improved modules from Task 1
6. **Task 6** (module completeness) — uses improved modules from Task 1

## Verification

After all tasks:

```bash
go clean -testcache
go test ./... -race -count=1
go build -o ./openkraft ./cmd/openkraft
./openkraft score . --json
./openkraft score testdata/go-hexagonal/perfect --json
```

**Expected results:**
- openkraft scoring itself: structure rises from 49 to ~75-85, discoverability from 70 to ~80-85, overall from 69 to ~78-85
- perfect fixture: should still score higher than incomplete and empty (e2e tests verify this)
- The tool should score a well-structured hexagonal project ≥80 regardless of layout variant
