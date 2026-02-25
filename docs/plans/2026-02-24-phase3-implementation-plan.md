# Phase 3: AI-Specific Scoring — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace 6 generic software quality scorers with 6 AI-agent-specific scorers backed by empirical research, enhanced AST parsing, and composite metrics.

**Architecture:** Enhanced parser extracts richer AST data (function size, nesting, params, error calls). Six new pure-domain scorers consume this data. Score service calls new scorers instead of old ones. Config system updated with new category/sub-metric names. Old scorers deleted.

**Tech Stack:** Go 1.24, go/ast, go/token, `github.com/fatih/camelcase` (new dep), existing stack (Cobra, Lipgloss, testify, yaml.v3)

**Prerequisite:** Phase 2 complete (config system, init command, all tests passing)

---

## Milestone Map

```
Tasks  1-2:   Foundation (enhanced domain types + parser)
Tasks  3-4:   code_health + discoverability scorers
Tasks  5-6:   structure + verifiability scorers
Tasks  7-8:   context_quality + predictability scorers
Task   9:     Update config registries
Task  10:     Wire new scorers into score_service
Task  11:     Update TUI renderer for new categories
Task  12:     Delete old scorers
Task  13:     Update all tests (unit + E2E)
Task  14:     Final verification
```

## Task Dependency Graph

```
Task 1 (Domain Types) → Task 2 (Parser) → Tasks 3-8 (Scorers, parallelizable)
                                                    ↓
                                              Task 9 (Config) → Task 10 (Wire Service)
                                                                       ↓
                                                                 Task 11 (TUI)
                                                                       ↓
                                                                 Task 12 (Delete Old)
                                                                       ↓
                                                                 Task 13 (Tests)
                                                                       ↓
                                                                 Task 14 (Verify)
```

---

## Task 1: Enhanced Domain Types

**Files:**
- Modify: `internal/domain/ports.go`

**Depends on:** Nothing

The `Function` struct and `AnalyzedFile` must carry richer data for all Phase 3 scorers. This is the foundation — every scorer depends on these types.

**Step 1: Write the enhanced types**

Replace the current `Function` struct and `AnalyzedFile` in `internal/domain/ports.go`:

```go
// Function represents a function or method extracted from source.
type Function struct {
	Name      string   `json:"name"`
	Receiver  string   `json:"receiver,omitempty"`
	Exported  bool     `json:"exported"`
	LineStart int      `json:"line_start"`
	LineEnd   int      `json:"line_end"`
	Params    []Param  `json:"params,omitempty"`
	Returns   []string `json:"returns,omitempty"`
	MaxNesting int     `json:"max_nesting"`
	MaxCondOps int     `json:"max_cond_ops"`
}

// Param represents a function parameter.
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ErrorCall represents an error creation call found in source.
type ErrorCall struct {
	Type       string `json:"type"`       // "fmt.Errorf" or "errors.New"
	HasWrap    bool   `json:"has_wrap"`    // contains %w
	HasContext bool   `json:"has_context"` // has variable interpolation
	Format     string `json:"format"`      // the format string literal
}

// TypeAssert represents a type assertion found in source.
type TypeAssert struct {
	Safe bool `json:"safe"` // true if comma-ok pattern (v, ok := x.(T))
}
```

Update `AnalyzedFile` to add new fields (keep existing fields):

```go
type AnalyzedFile struct {
	Path           string        `json:"path"`
	Package        string        `json:"package"`
	Structs        []string      `json:"structs,omitempty"`
	Functions      []Function    `json:"functions,omitempty"`
	Interfaces     []string      `json:"interfaces,omitempty"`
	Imports        []string      `json:"imports,omitempty"`
	PackageDoc     bool          `json:"package_doc,omitempty"`
	InitFunctions  int           `json:"init_functions,omitempty"`
	GlobalVars     []string      `json:"global_vars,omitempty"`
	ErrorCalls     []ErrorCall   `json:"error_calls,omitempty"`
	TypeAssertions []TypeAssert  `json:"type_assertions,omitempty"`
	TotalLines     int           `json:"total_lines,omitempty"`
}
```

**Step 2: Fix all compilation errors**

The `Function` type changed from `{Name, Receiver string}` to the enhanced struct. Every file that reads `Function.Name` or `Function.Receiver` still works (fields kept). But code that creates `Function` literals needs updating.

Files that create `Function` values:
- `internal/adapters/outbound/parser/go_parser.go` (updated in Task 2)
- Test files that build mock `AnalyzedFile` data

For now, update only the parser-adjacent test helpers. The old scorers still compile because they only read `.Name` and `.Receiver` from functions via `ModuleFile.Functions []string` (which is separate from `AnalyzedFile.Functions`).

**Important:** `ModuleFile.Functions` in `model.go` is `[]string` — this is the module detector's view. `AnalyzedFile.Functions` is `[]Function` — this is the parser's view. They're separate types. The old scorers use `ModuleFile.Functions` (string names), not `AnalyzedFile.Functions`. So this change doesn't break old scorers.

**Step 3: Verify compilation**

```bash
go build ./...
```

---

## Task 2: Enhanced Go Parser

**Files:**
- Modify: `internal/adapters/outbound/parser/go_parser.go`
- Modify: `internal/adapters/outbound/parser/go_parser_test.go`

**Depends on:** Task 1

Enhance the parser to extract all data needed by Phase 3 scorers.

**Step 1: Write tests for new parser capabilities**

Add to `internal/adapters/outbound/parser/go_parser_test.go`:

```go
func TestGoParser_FunctionLineNumbers(t *testing.T) {
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax.go")
	require.NoError(t, err)
	for _, fn := range af.Functions {
		assert.Greater(t, fn.LineStart, 0, "function %s should have line start", fn.Name)
		assert.GreaterOrEqual(t, fn.LineEnd, fn.LineStart, "function %s end >= start", fn.Name)
	}
}

func TestGoParser_FunctionParams(t *testing.T) {
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax.go")
	require.NoError(t, err)
	// Find a function with known params
	for _, fn := range af.Functions {
		if fn.Name == "Validate" {
			// Validate is a method with receiver only, no params
			assert.Empty(t, fn.Params)
		}
	}
}

func TestGoParser_ExportedFlag(t *testing.T) {
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax.go")
	require.NoError(t, err)
	for _, fn := range af.Functions {
		if fn.Name[0] >= 'A' && fn.Name[0] <= 'Z' {
			assert.True(t, fn.Exported, "function %s should be exported", fn.Name)
		}
	}
}

func TestGoParser_PackageDoc(t *testing.T) {
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax.go")
	require.NoError(t, err)
	// We just check the field exists and is a bool
	_ = af.PackageDoc
}

func TestGoParser_TotalLines(t *testing.T) {
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax.go")
	require.NoError(t, err)
	assert.Greater(t, af.TotalLines, 0)
}

func TestGoParser_InitFunctions(t *testing.T) {
	p := parser.New()
	af, err := p.AnalyzeFile("../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax.go")
	require.NoError(t, err)
	// Domain files typically don't have init()
	assert.Equal(t, 0, af.InitFunctions)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/adapters/outbound/parser/... -v -run "TestGoParser_Function|TestGoParser_Package|TestGoParser_Total|TestGoParser_Init"
```

Expected: FAIL (new fields are zero-valued since parser doesn't populate them yet)

**Step 3: Implement enhanced parser**

Rewrite `internal/adapters/outbound/parser/go_parser.go`:

```go
package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

type GoParser struct{}

func New() *GoParser { return &GoParser{} }

func (p *GoParser) AnalyzeFile(filePath string) (*domain.AnalyzedFile, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, filePath, nil, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	result := &domain.AnalyzedFile{
		Path:       filePath,
		Package:    file.Name.Name,
		PackageDoc: file.Doc != nil && len(file.Doc.List) > 0,
	}

	// Total lines
	if fset.File(file.Pos()) != nil {
		result.TotalLines = fset.File(file.Pos()).LineCount()
	}

	// Imports
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, path)
	}

	// Walk AST
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.GenDecl:
			p.processGenDecl(decl, fset, result)
		case *ast.FuncDecl:
			fn := p.processFunc(decl, fset, file)
			result.Functions = append(result.Functions, fn)
			if decl.Name.Name == "init" {
				result.InitFunctions++
			}
		}
		return true
	})

	// Error calls and type assertions (separate walk for clarity)
	result.ErrorCalls = extractErrorCalls(file)
	result.TypeAssertions = extractTypeAssertions(file)

	return result, nil
}

func (p *GoParser) processGenDecl(decl *ast.GenDecl, fset *token.FileSet, result *domain.AnalyzedFile) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			switch s.Type.(type) {
			case *ast.StructType:
				result.Structs = append(result.Structs, s.Name.Name)
			case *ast.InterfaceType:
				result.Interfaces = append(result.Interfaces, s.Name.Name)
			}
		case *ast.ValueSpec:
			// Track package-level var declarations
			if decl.Tok == token.VAR {
				for _, name := range s.Names {
					if name.Name != "_" {
						result.GlobalVars = append(result.GlobalVars, name.Name)
					}
				}
			}
		}
	}
}

func (p *GoParser) processFunc(decl *ast.FuncDecl, fset *token.FileSet, file *ast.File) domain.Function {
	f := domain.Function{
		Name:     decl.Name.Name,
		Exported: decl.Name.IsExported(),
	}

	// Line numbers
	f.LineStart = fset.Position(decl.Pos()).Line
	f.LineEnd = fset.Position(decl.End()).Line

	// Receiver
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		f.Receiver = receiverType(decl.Recv.List[0].Type)
	}

	// Parameters
	if decl.Type.Params != nil {
		for _, field := range decl.Type.Params.List {
			typeName := exprToString(field.Type)
			if len(field.Names) == 0 {
				// Unnamed param (e.g., in interface declarations)
				f.Params = append(f.Params, domain.Param{Type: typeName})
			} else {
				for _, name := range field.Names {
					f.Params = append(f.Params, domain.Param{
						Name: name.Name,
						Type: typeName,
					})
				}
			}
		}
	}

	// Returns
	if decl.Type.Results != nil {
		for _, field := range decl.Type.Results.List {
			f.Returns = append(f.Returns, exprToString(field.Type))
		}
	}

	// Nesting depth and conditional complexity
	if decl.Body != nil {
		f.MaxNesting = maxNestingDepth(decl.Body, 0)
		f.MaxCondOps = maxConditionalOps(decl.Body)
	}

	return f
}

// maxNestingDepth walks a block statement and returns the deepest nesting level.
func maxNestingDepth(block *ast.BlockStmt, currentDepth int) int {
	maxDepth := currentDepth
	for _, stmt := range block.List {
		depth := nestingForStmt(stmt, currentDepth)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

func nestingForStmt(stmt ast.Stmt, depth int) int {
	maxDepth := depth
	switch s := stmt.(type) {
	case *ast.IfStmt:
		newDepth := depth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
		if s.Body != nil {
			d := maxNestingDepth(s.Body, newDepth)
			if d > maxDepth {
				maxDepth = d
			}
		}
		if s.Else != nil {
			d := nestingForStmt(s.Else, depth) // else is same level as if
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.ForStmt:
		newDepth := depth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
		if s.Body != nil {
			d := maxNestingDepth(s.Body, newDepth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.RangeStmt:
		newDepth := depth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
		if s.Body != nil {
			d := maxNestingDepth(s.Body, newDepth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.SwitchStmt:
		newDepth := depth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
		if s.Body != nil {
			d := maxNestingDepth(s.Body, newDepth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.TypeSwitchStmt:
		newDepth := depth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
		if s.Body != nil {
			d := maxNestingDepth(s.Body, newDepth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.SelectStmt:
		newDepth := depth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
		if s.Body != nil {
			d := maxNestingDepth(s.Body, newDepth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.BlockStmt:
		d := maxNestingDepth(s, depth)
		if d > maxDepth {
			maxDepth = d
		}
	case *ast.CaseClause:
		for _, bodyStmt := range s.Body {
			d := nestingForStmt(bodyStmt, depth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	case *ast.CommClause:
		for _, bodyStmt := range s.Body {
			d := nestingForStmt(bodyStmt, depth)
			if d > maxDepth {
				maxDepth = d
			}
		}
	}
	return maxDepth
}

// maxConditionalOps finds the maximum number of &&/|| operators in any single if condition.
func maxConditionalOps(block *ast.BlockStmt) int {
	maxOps := 0
	ast.Inspect(block, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		ops := countBoolOps(ifStmt.Cond)
		if ops > maxOps {
			maxOps = ops
		}
		return true
	})
	return maxOps
}

func countBoolOps(expr ast.Expr) int {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return 0
	}
	count := 0
	if bin.Op == token.LAND || bin.Op == token.LOR {
		count = 1
	}
	return count + countBoolOps(bin.X) + countBoolOps(bin.Y)
}

// extractErrorCalls finds fmt.Errorf and errors.New calls.
func extractErrorCalls(file *ast.File) []domain.ErrorCall {
	var calls []domain.ErrorCall
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		var ec domain.ErrorCall
		switch {
		case pkg.Name == "fmt" && sel.Sel.Name == "Errorf":
			ec.Type = "fmt.Errorf"
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					ec.Format = lit.Value
					ec.HasWrap = strings.Contains(lit.Value, "%w")
					ec.HasContext = strings.ContainsAny(lit.Value, "svdxfgq")
				}
			}
			calls = append(calls, ec)
		case pkg.Name == "errors" && sel.Sel.Name == "New":
			ec.Type = "errors.New"
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					ec.Format = lit.Value
				}
			}
			calls = append(calls, ec)
		}
		return true
	})
	return calls
}

// extractTypeAssertions finds type assertion expressions and checks if they use comma-ok.
func extractTypeAssertions(file *ast.File) []domain.TypeAssert {
	var asserts []domain.TypeAssert
	ast.Inspect(file, func(n ast.Node) bool {
		// Look for assignments containing type assertions
		assign, ok := n.(*ast.AssignStmt)
		if ok {
			for _, rhs := range assign.Rhs {
				if _, isTA := rhs.(*ast.TypeAssertExpr); isTA {
					asserts = append(asserts, domain.TypeAssert{
						Safe: len(assign.Lhs) == 2, // v, ok := x.(T)
					})
				}
			}
			return true
		}
		return true
	})
	return asserts
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + receiverType(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.FuncType:
		return "func"
	case *ast.ChanType:
		return "chan"
	default:
		return "unknown"
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/adapters/outbound/parser/... -v
```

Expected: ALL PASS

**Step 5: Add camelcase dependency**

```bash
go get github.com/fatih/camelcase
```

**Step 6: Verify full build**

```bash
go build ./...
```

---

## Task 3: code_health Scorer

**Files:**
- Create: `internal/domain/scoring/code_health.go`
- Create: `internal/domain/scoring/code_health_test.go`

**Depends on:** Tasks 1, 2

**Step 1: Write tests**

```go
package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreCodeHealth_SubMetricNames(t *testing.T) {
	cat := scoring.ScoreCodeHealth(nil, nil)
	expected := []string{"function_size", "file_size", "nesting_depth", "parameter_count", "complex_conditionals"}
	for i, sm := range cat.SubMetrics {
		assert.Equal(t, expected[i], sm.Name)
	}
}

func TestScoreCodeHealth_Weight(t *testing.T) {
	cat := scoring.ScoreCodeHealth(nil, nil)
	assert.InDelta(t, 0.25, cat.Weight, 0.001)
}

func TestScoreCodeHealth_MaxPoints(t *testing.T) {
	cat := scoring.ScoreCodeHealth(nil, nil)
	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Points
	}
	assert.Equal(t, 100, total)
}

func TestScoreCodeHealth_SmallFunctionsScoreHigh(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go": {
			Functions: []domain.Function{
				{Name: "Foo", LineStart: 1, LineEnd: 20},  // 20 lines = good
				{Name: "Bar", LineStart: 22, LineEnd: 40}, // 19 lines = good
			},
			TotalLines: 50,
		},
	}
	cat := scoring.ScoreCodeHealth(nil, analyzed)
	// function_size should be full points
	assert.Equal(t, 20, cat.SubMetrics[0].Score)
}

func TestScoreCodeHealth_LargeFunctionsScoreLow(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go": {
			Functions: []domain.Function{
				{Name: "Foo", LineStart: 1, LineEnd: 150}, // 150 lines = bad
			},
			TotalLines: 160,
		},
	}
	cat := scoring.ScoreCodeHealth(nil, analyzed)
	assert.Equal(t, 0, cat.SubMetrics[0].Score) // function_size = 0
}

func TestScoreCodeHealth_DeepNestingScoreLow(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"main.go": {
			Functions: []domain.Function{
				{Name: "Foo", LineStart: 1, LineEnd: 30, MaxNesting: 6},
			},
			TotalLines: 30,
		},
	}
	cat := scoring.ScoreCodeHealth(nil, analyzed)
	// nesting_depth is sub-metric index 2
	assert.Equal(t, 0, cat.SubMetrics[2].Score)
}

func TestScoreCodeHealth_PerfectFixture(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	_ = modules
	_ = scan
	cat := scoring.ScoreCodeHealth(nil, analyzed)
	assert.Greater(t, cat.Score, 50, "perfect fixture should score well on code health")
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/domain/scoring/... -v -run "TestScoreCodeHealth"
```

Expected: FAIL (function doesn't exist)

**Step 3: Implement code_health.go**

```go
package scoring

import (
	"fmt"
	"math"

	"github.com/openkraft/openkraft/internal/domain"
)

// ScoreCodeHealth evaluates the 5 code smells that empirically predict
// AI agent success (CodeScene arXiv:2601.02200).
// Weight: 0.25.
func ScoreCodeHealth(scan *domain.ScanResult, analyzed map[string]*domain.AnalyzedFile) domain.CategoryScore {
	cat := domain.CategoryScore{
		Name:   "code_health",
		Weight: 0.25,
	}

	sm1 := scoreFunctionSize(analyzed)
	sm2 := scoreFileSize(analyzed)
	sm3 := scoreNestingDepth(analyzed)
	sm4 := scoreParameterCount(analyzed)
	sm5 := scoreComplexConditionals(analyzed)

	cat.SubMetrics = []domain.SubMetric{sm1, sm2, sm3, sm4, sm5}

	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Score
	}
	cat.Score = total

	cat.Issues = collectCodeHealthIssues(analyzed)
	return cat
}

// scoreFunctionSize (20 pts): <=50 lines full, 51-100 partial, >100 zero.
func scoreFunctionSize(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "function_size", Points: 20}
	if len(analyzed) == 0 {
		sm.Detail = "no files to analyze"
		return sm
	}

	var totalScore float64
	var count int
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines <= 0 {
				continue
			}
			count++
			switch {
			case lines <= 50:
				totalScore += 1.0
			case lines <= 100:
				totalScore += 0.5
			default:
				// 0
			}
		}
	}

	if count == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	avg := totalScore / float64(count)
	sm.Score = int(math.Round(avg * float64(sm.Points)))
	sm.Detail = fmt.Sprintf("%.0f%% functions <=50 lines across %d functions", avg*100, count)
	return sm
}

// scoreFileSize (20 pts): <=300 lines full, 301-500 partial, >500 zero.
func scoreFileSize(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "file_size", Points: 20}
	if len(analyzed) == 0 {
		sm.Detail = "no files to analyze"
		return sm
	}

	var totalScore float64
	var count int
	for _, af := range analyzed {
		if af.TotalLines <= 0 {
			continue
		}
		count++
		switch {
		case af.TotalLines <= 300:
			totalScore += 1.0
		case af.TotalLines <= 500:
			totalScore += 0.5
		default:
			// 0
		}
	}

	if count == 0 {
		sm.Detail = "no files with line counts"
		return sm
	}

	avg := totalScore / float64(count)
	sm.Score = int(math.Round(avg * float64(sm.Points)))
	sm.Detail = fmt.Sprintf("%.0f%% files <=300 lines across %d files", avg*100, count)
	return sm
}

// scoreNestingDepth (20 pts): <=3 full, 4 partial, >=5 zero.
func scoreNestingDepth(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "nesting_depth", Points: 20}
	if len(analyzed) == 0 {
		sm.Detail = "no files to analyze"
		return sm
	}

	var totalScore float64
	var count int
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			count++
			switch {
			case fn.MaxNesting <= 3:
				totalScore += 1.0
			case fn.MaxNesting == 4:
				totalScore += 0.5
			default:
				// 0
			}
		}
	}

	if count == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	avg := totalScore / float64(count)
	sm.Score = int(math.Round(avg * float64(sm.Points)))
	sm.Detail = fmt.Sprintf("%.0f%% functions with nesting <=3 across %d functions", avg*100, count)
	return sm
}

// scoreParameterCount (20 pts): <=4 full, 5-6 partial, >=7 zero.
func scoreParameterCount(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "parameter_count", Points: 20}
	if len(analyzed) == 0 {
		sm.Detail = "no files to analyze"
		return sm
	}

	var totalScore float64
	var count int
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			count++
			switch {
			case len(fn.Params) <= 4:
				totalScore += 1.0
			case len(fn.Params) <= 6:
				totalScore += 0.5
			default:
				// 0
			}
		}
	}

	if count == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	avg := totalScore / float64(count)
	sm.Score = int(math.Round(avg * float64(sm.Points)))
	sm.Detail = fmt.Sprintf("%.0f%% functions with <=4 params across %d functions", avg*100, count)
	return sm
}

// scoreComplexConditionals (20 pts): <=2 &&/|| full, 3 partial, >=4 zero.
func scoreComplexConditionals(analyzed map[string]*domain.AnalyzedFile) domain.SubMetric {
	sm := domain.SubMetric{Name: "complex_conditionals", Points: 20}
	if len(analyzed) == 0 {
		sm.Detail = "no files to analyze"
		return sm
	}

	var totalScore float64
	var count int
	for _, af := range analyzed {
		for _, fn := range af.Functions {
			count++
			switch {
			case fn.MaxCondOps <= 2:
				totalScore += 1.0
			case fn.MaxCondOps == 3:
				totalScore += 0.5
			default:
				// 0
			}
		}
	}

	if count == 0 {
		sm.Detail = "no functions found"
		return sm
	}

	avg := totalScore / float64(count)
	sm.Score = int(math.Round(avg * float64(sm.Points)))
	sm.Detail = fmt.Sprintf("%.0f%% functions with <=2 conditional ops across %d functions", avg*100, count)
	return sm
}

func collectCodeHealthIssues(analyzed map[string]*domain.AnalyzedFile) []domain.Issue {
	var issues []domain.Issue
	for path, af := range analyzed {
		if af.TotalLines > 500 {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityWarning,
				Category: "code_health",
				File:     path,
				Message:  fmt.Sprintf("file has %d lines (recommended <=300)", af.TotalLines),
			})
		}
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines > 100 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has %d lines (recommended <=50)", fn.Name, lines),
				})
			}
			if fn.MaxNesting >= 5 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityError,
					Category: "code_health",
					File:     path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has nesting depth %d (recommended <=3)", fn.Name, fn.MaxNesting),
				})
			}
			if len(fn.Params) >= 7 {
				issues = append(issues, domain.Issue{
					Severity: domain.SeverityWarning,
					Category: "code_health",
					File:     path,
					Line:     fn.LineStart,
					Message:  fmt.Sprintf("function %s has %d parameters (recommended <=4)", fn.Name, len(fn.Params)),
				})
			}
		}
	}
	return issues
}
```

**Step 4: Run tests**

```bash
go test ./internal/domain/scoring/... -v -run "TestScoreCodeHealth"
```

Expected: ALL PASS

---

## Task 4: discoverability Scorer

**Files:**
- Create: `internal/domain/scoring/discoverability.go`
- Create: `internal/domain/scoring/discoverability_test.go`

**Depends on:** Tasks 1, 2

**Step 1: Write tests**

```go
package scoring_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScoreDiscoverability_SubMetricNames(t *testing.T) {
	cat := scoring.ScoreDiscoverability(nil, nil, nil)
	expected := []string{"naming_uniqueness", "file_naming_conventions", "predictable_structure", "dependency_direction"}
	for i, sm := range cat.SubMetrics {
		assert.Equal(t, expected[i], sm.Name)
	}
}

func TestScoreDiscoverability_Weight(t *testing.T) {
	cat := scoring.ScoreDiscoverability(nil, nil, nil)
	assert.InDelta(t, 0.20, cat.Weight, 0.001)
}

func TestScoreDiscoverability_MaxPoints(t *testing.T) {
	cat := scoring.ScoreDiscoverability(nil, nil, nil)
	total := 0
	for _, sm := range cat.SubMetrics {
		total += sm.Points
	}
	assert.Equal(t, 100, total)
}

func TestScoreDiscoverability_UniqueNamesScoreHigh(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"payment.go": {
			Functions: []domain.Function{
				{Name: "CreatePayment", Exported: true},
				{Name: "ValidateOrder", Exported: true},
				{Name: "CalculateTax", Exported: true},
			},
		},
	}
	cat := scoring.ScoreDiscoverability(nil, nil, analyzed)
	assert.Greater(t, cat.SubMetrics[0].Score, 15) // naming_uniqueness high
}

func TestScoreDiscoverability_VagueNamesScoreLow(t *testing.T) {
	analyzed := map[string]*domain.AnalyzedFile{
		"handler.go": {
			Functions: []domain.Function{
				{Name: "Handle", Exported: true},
				{Name: "Process", Exported: true},
				{Name: "Do", Exported: true},
			},
		},
	}
	cat := scoring.ScoreDiscoverability(nil, nil, analyzed)
	assert.Less(t, cat.SubMetrics[0].Score, 15) // naming_uniqueness low
}

func TestScoreDiscoverability_ConventionalFileSuffixes(t *testing.T) {
	scan := &domain.ScanResult{
		GoFiles: []string{
			"payment_handler.go",
			"payment_service.go",
			"payment_test.go",
			"payment_repository.go",
		},
	}
	cat := scoring.ScoreDiscoverability(nil, scan, nil)
	assert.Equal(t, 25, cat.SubMetrics[1].Score) // file_naming_conventions full
}

func TestScoreDiscoverability_PerfectFixture(t *testing.T) {
	modules, scan, analyzed := loadFixture(t)
	cat := scoring.ScoreDiscoverability(modules, scan, analyzed)
	assert.Greater(t, cat.Score, 30, "perfect fixture should have decent discoverability")
}
```

**Step 2: Implement discoverability.go**

The implementation includes:
- `ScoreDiscoverability(modules, scan, analyzed)` — main scorer
- `scoreNamingUniqueness(analyzed)` — composite: word count (40%) + vocabulary specificity (30%) + Shannon entropy (30%)
- `scoreFileNamingConventions(scan)` — ratio of files with conventional suffixes
- `scorePredictableStructure(modules)` — Jaccard similarity of file suffixes across same-level modules
- `scoreDependencyDirection(modules, analyzed)` — migrated from Phase 1 architecture scorer

Uses `github.com/fatih/camelcase` for CamelCase splitting.

Contains `vagueWords` map and `specificPrefixes` map for vocabulary scoring.

**Step 3: Run tests**

```bash
go test ./internal/domain/scoring/... -v -run "TestScoreDiscoverability"
```

---

## Task 5: structure Scorer

**Files:**
- Create: `internal/domain/scoring/structure.go`
- Create: `internal/domain/scoring/structure_test.go`

**Depends on:** Tasks 1, 2

**Step 1: Write tests**

Tests for: sub-metric names, weight (0.15), max points (100), perfect fixture scores well, empty modules get zero.

**Step 2: Implement structure.go**

- `ScoreStructure(modules, scan, analyzed)` — main scorer, weight 0.15
- `scoreExpectedLayers(modules, scan)` — presence of expected directories per project type. Checks for `internal/`, `cmd/`, `domain/`, `application/`, `adapters/` paths in scan.AllFiles
- `scoreExpectedFiles(modules, analyzed)` — per module, ratio of files matching conventional suffixes (model, service, handler, repository, ports)
- `scoreInterfaceContracts(modules, analyzed)` — count interfaces in domain/ports vs. concrete cross-package dependencies
- `scoreModuleCompleteness(modules, analyzed)` — golden module comparison (migrated from Phase 1 completeness scorer)

**Step 3: Run tests**

```bash
go test ./internal/domain/scoring/... -v -run "TestScoreStructure"
```

---

## Task 6: verifiability Scorer

**Files:**
- Create: `internal/domain/scoring/verifiability.go`
- Create: `internal/domain/scoring/verifiability_test.go`

**Depends on:** Tasks 1, 2

**Step 1: Write tests**

Tests for: sub-metric names, weight (0.15), max points (100), project with tests scores well, project without tests scores low.

**Step 2: Implement verifiability.go**

- `ScoreVerifiability(scan, analyzed)` — main scorer, weight 0.15
- `scoreTestPresence(scan)` — ratio of `.go` files with corresponding `_test.go`. Migrated from Phase 1 tests.unit_test_presence
- `scoreTestNaming(analyzed)` — parse test functions, check `Test<Func>_<Scenario>` pattern, check for `t.Run` subtests
- `scoreBuildReproducibility(scan)` — go.sum (10pts), Makefile/Taskfile (8pts), CI config (7pts)
- `scoreTypeSafetySignals(scan, analyzed)` — .golangci.yml presence (10pts), low `interface{}`/`any` usage (10pts), safe type assertions (5pts)

**Step 3: Run tests**

```bash
go test ./internal/domain/scoring/... -v -run "TestScoreVerifiability"
```

---

## Task 7: context_quality Scorer

**Files:**
- Create: `internal/domain/scoring/context_quality.go`
- Create: `internal/domain/scoring/context_quality_test.go`

**Depends on:** Tasks 1, 2

**Step 1: Write tests**

Tests for: sub-metric names, weight (0.15), max points (100), project with CLAUDE.md scores well.

**Step 2: Implement context_quality.go**

- `ScoreContextQuality(scan, analyzed)` — main scorer, weight 0.15
- `scoreAIContextFiles(scan)` — CLAUDE.md, AGENTS.md, .cursorrules, copilot-instructions.md. Presence + min size. Migrated and enhanced from Phase 1 ai_context
- `scorePackageDocumentation(analyzed)` — ratio of packages with `// Package ...` doc comment
- `scoreArchitectureDocs(scan)` — README.md size, docs/ directory, ADR files
- `scoreCanonicalExamples(scan, analyzed)` — example_test.go files + CLAUDE.md pattern references

**Step 3: Run tests**

```bash
go test ./internal/domain/scoring/... -v -run "TestScoreContextQuality"
```

---

## Task 8: predictability Scorer

**Files:**
- Create: `internal/domain/scoring/predictability.go`
- Create: `internal/domain/scoring/predictability_test.go`

**Depends on:** Tasks 1, 2

**Step 1: Write tests**

Tests for: sub-metric names, weight (0.10), max points (100), descriptive names score high, vague names score low, wrapped errors score high.

**Step 2: Implement predictability.go**

- `ScorePredictability(modules, scan, analyzed)` — main scorer, weight 0.10
- `scoreSelfDescribingNames(analyzed)` — exported functions with verb+noun pattern. Uses camelcase split + `specificPrefixes` from discoverability (extract to shared helper)
- `scoreExplicitDependencies(analyzed)` — count mutable package-level vars + init() functions. Zero = full score
- `scoreErrorMessageQuality(analyzed)` — composite: wrapping ratio (40%) + context richness (30%) + convention compliance (20%) + sentinel presence (10%)
- `scoreConsistentPatterns(modules, analyzed)` — group functions by role (file suffix), extract normalized signatures, measure modal consistency

**Step 3: Run tests**

```bash
go test ./internal/domain/scoring/... -v -run "TestScorePredictability"
```

---

## Task 9: Update Config Registries

**Files:**
- Modify: `internal/domain/config.go`
- Modify: `internal/domain/config_test.go`

**Depends on:** Tasks 3-8

**Step 1: Update ValidCategories and ValidSubMetrics**

In `internal/domain/config.go`, replace:

```go
var ValidCategories = []string{
	"code_health", "discoverability", "structure",
	"verifiability", "context_quality", "predictability",
}

var ValidSubMetrics = []string{
	// code_health
	"function_size", "file_size", "nesting_depth",
	"parameter_count", "complex_conditionals",
	// discoverability
	"naming_uniqueness", "file_naming_conventions",
	"predictable_structure", "dependency_direction",
	// structure
	"expected_layers", "expected_files",
	"interface_contracts", "module_completeness",
	// verifiability
	"test_presence", "test_naming",
	"build_reproducibility", "type_safety_signals",
	// context_quality
	"ai_context_files", "package_documentation",
	"architecture_docs", "canonical_examples",
	// predictability
	"self_describing_names", "explicit_dependencies",
	"error_message_quality", "consistent_patterns",
}
```

**Step 2: Update DefaultConfigForType**

Update all project type presets with new category names and weights:

```go
func DefaultConfigForType(pt ProjectType) ProjectConfig {
	cfg := ProjectConfig{ProjectType: pt}

	switch pt {
	case ProjectTypeCLI:
		cfg.Weights = map[string]float64{
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.10,
			"verifiability": 0.20, "context_quality": 0.15, "predictability": 0.10,
		}
	case ProjectTypeLibrary:
		cfg.Weights = map[string]float64{
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.10,
			"verifiability": 0.25, "context_quality": 0.10, "predictability": 0.10,
		}
	case ProjectTypeMicroservice:
		cfg.Weights = map[string]float64{
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.20,
			"verifiability": 0.15, "context_quality": 0.10, "predictability": 0.10,
		}
	default: // api
		cfg.Weights = map[string]float64{
			"code_health": 0.25, "discoverability": 0.20, "structure": 0.15,
			"verifiability": 0.15, "context_quality": 0.15, "predictability": 0.10,
		}
	}

	return cfg
}
```

**Step 3: Update config_test.go**

Update all tests that reference old category names (`architecture`, `conventions`, `patterns`, `tests`, `ai_context`, `completeness`) to use new names.

**Step 4: Verify**

```bash
go test ./internal/domain/... -v
```

---

## Task 10: Wire New Scorers into ScoreService

**Files:**
- Modify: `internal/application/score_service.go`
- Modify: `internal/application/score_service_test.go`

**Depends on:** Tasks 3-9

**Step 1: Replace scorer calls**

In `score_service.go`, replace the scorer invocation block:

```go
	// 4. Run all 6 scorers
	categories := []domain.CategoryScore{
		scoring.ScoreCodeHealth(scan, analyzed),
		scoring.ScoreDiscoverability(modules, scan, analyzed),
		scoring.ScoreStructure(modules, scan, analyzed),
		scoring.ScoreVerifiability(scan, analyzed),
		scoring.ScoreContextQuality(scan, analyzed),
		scoring.ScorePredictability(modules, scan, analyzed),
	}
```

**Step 2: Update score_service_test.go**

- Change expected category count from 6 (same, but different names)
- Update weight assertions to new values
- Update category name assertions
- Update config-aware tests to use new category/sub-metric names

```go
func TestScoreService_CategoriesHaveCorrectWeights(t *testing.T) {
	// ... setup ...
	expectedWeights := map[string]float64{
		"code_health":    0.25,
		"discoverability": 0.20,
		"structure":       0.15,
		"verifiability":   0.15,
		"context_quality": 0.15,
		"predictability":  0.10,
	}
	for _, cat := range score.Categories {
		expected, ok := expectedWeights[cat.Name]
		assert.True(t, ok, "unexpected category: %s", cat.Name)
		assert.InDelta(t, expected, cat.Weight, 0.001, "weight mismatch for %s", cat.Name)
	}
}
```

**Step 3: Verify**

```bash
go test ./internal/application/... -v
```

---

## Task 11: Update TUI Renderer

**Files:**
- Modify: `internal/adapters/outbound/tui/renderer.go`
- Modify: `internal/adapters/outbound/tui/renderer_test.go`

**Depends on:** Task 10

The TUI renderer is category-agnostic — it iterates `score.Categories` and renders whatever it finds. The main changes are:
1. Update any hardcoded category name references
2. Verify tests pass with new category names

**Step 1: Check for hardcoded category names in renderer.go**

Search for any string literals matching old category names. If found, update them. The renderer should be generic enough that it just works.

**Step 2: Update renderer_test.go**

Update test assertions that check for specific category names:

```go
// Replace old names with new ones in assertions
assert.Contains(t, output, "code_health")
assert.Contains(t, output, "discoverability")
```

**Step 3: Verify**

```bash
go test ./internal/adapters/outbound/tui/... -v
```

---

## Task 12: Delete Old Scorers

**Files:**
- Delete: `internal/domain/scoring/architecture.go`
- Delete: `internal/domain/scoring/architecture_test.go`
- Delete: `internal/domain/scoring/conventions.go`
- Delete: `internal/domain/scoring/conventions_test.go`
- Delete: `internal/domain/scoring/patterns.go`
- Delete: `internal/domain/scoring/patterns_test.go`
- Delete: `internal/domain/scoring/tests.go`
- Delete: `internal/domain/scoring/tests_test.go`
- Delete: `internal/domain/scoring/ai_context.go`
- Delete: `internal/domain/scoring/ai_context_test.go`
- Delete: `internal/domain/scoring/completeness.go`
- Delete: `internal/domain/scoring/completeness_test.go`

**Depends on:** Task 10

**Step 1: Delete all 12 files**

```bash
rm internal/domain/scoring/architecture.go internal/domain/scoring/architecture_test.go
rm internal/domain/scoring/conventions.go internal/domain/scoring/conventions_test.go
rm internal/domain/scoring/patterns.go internal/domain/scoring/patterns_test.go
rm internal/domain/scoring/tests.go internal/domain/scoring/tests_test.go
rm internal/domain/scoring/ai_context.go internal/domain/scoring/ai_context_test.go
rm internal/domain/scoring/completeness.go internal/domain/scoring/completeness_test.go
```

**Step 2: Check for any remaining references**

```bash
grep -r "ScoreArchitecture\|ScoreConventions\|ScorePatterns\|ScoreTests\|ScoreAIContext\|ScoreCompleteness" internal/
```

If any references remain (besides score_service.go which was already updated), fix them.

**Step 3: Verify compilation and tests**

```bash
go build ./...
go test ./internal/domain/scoring/... -v
```

---

## Task 13: Update All Tests

**Files:**
- Modify: `internal/adapters/inbound/cli/score_test.go`
- Modify: `internal/adapters/outbound/tui/check_renderer_test.go` (if needed)
- Modify: `tests/e2e/e2e_test.go`
- Modify: `internal/adapters/inbound/cli/init.go` (update generated YAML template)
- Modify: `internal/adapters/outbound/config/yaml_loader_test.go` (update category names)

**Depends on:** Tasks 10, 11, 12

**Step 1: Update CLI score_test.go**

The score tests use `NewRootCmdForTest()` and run the full pipeline. Update assertions:

```go
func TestScoreCommand_DefaultTUI(t *testing.T) {
	// ... existing setup ...
	assert.Contains(t, buf.String(), "openkraft")
	// Score may change from 100 since scoring logic changed
	// Just verify it outputs something reasonable
	assert.Contains(t, buf.String(), "/100")
}
```

**Step 2: Update E2E tests**

In `tests/e2e/e2e_test.go`:

- `TestE2E_ScoreJSON`: Update expected category count (still 6) and category names
- `TestE2E_ScoreWithCLIConfig`: Update assertions for new category names and weights
- `TestE2E_ScoreSkippedCategory`: Use new category name (e.g., `predictability` instead of `ai_context`)
- `TestE2E_ScoreOrdering`: Still works (perfect > incomplete > empty should hold)

```go
func TestE2E_ScoreJSON(t *testing.T) {
	// ... existing ...
	assert.Len(t, score.Categories, 6, "should have 6 categories")
	// Verify new category names exist
	names := make(map[string]bool)
	for _, cat := range score.Categories {
		names[cat.Name] = true
	}
	assert.True(t, names["code_health"])
	assert.True(t, names["discoverability"])
	assert.True(t, names["structure"])
	assert.True(t, names["verifiability"])
	assert.True(t, names["context_quality"])
	assert.True(t, names["predictability"])
}
```

**Step 3: Update init.go template**

The `generateConfig()` function in `init.go` outputs a YAML template with category names. Update to new names:

```go
func generateConfig(pt domain.ProjectType) string {
	// ... update weights section to use new category names:
	// code_health, discoverability, structure, verifiability, context_quality, predictability
}
```

**Step 4: Update YAML loader tests**

Any tests in `yaml_loader_test.go` that use old category names in YAML strings must be updated.

**Step 5: Run full test suite**

```bash
go test ./... -count=1
```

Fix any remaining failures.

---

## Task 14: Final Verification

**Depends on:** All previous tasks

**Step 1: Full test suite**

```bash
make test
```

All packages must pass.

**Step 2: Build binary**

```bash
make build
```

Or:

```bash
go build -o openkraft ./cmd/openkraft
```

**Step 3: Score the test fixture**

```bash
./openkraft score testdata/go-hexagonal/perfect --json
```

Verify:
- 6 categories with new names
- Each category has correct sub-metrics
- Score is 0-100
- No crashes

**Step 4: Score OpenKraft itself**

```bash
./openkraft score . --json
```

Verify:
- Produces a score across all 6 new categories
- code_health reflects actual function sizes and nesting
- discoverability reflects naming patterns
- context_quality detects CLAUDE.md

**Step 5: Score with CLI config**

```bash
./openkraft init --type cli-tool --force
./openkraft score .
```

Verify:
- Config applies correctly with new category weights
- TUI shows categories with correct weights

**Step 6: Clean up**

```bash
rm -f .openkraft.yaml
rm -f openkraft
```

---

## Files Summary

### New Files (12)
- `internal/domain/scoring/code_health.go`
- `internal/domain/scoring/code_health_test.go`
- `internal/domain/scoring/discoverability.go`
- `internal/domain/scoring/discoverability_test.go`
- `internal/domain/scoring/structure.go`
- `internal/domain/scoring/structure_test.go`
- `internal/domain/scoring/verifiability.go`
- `internal/domain/scoring/verifiability_test.go`
- `internal/domain/scoring/context_quality.go`
- `internal/domain/scoring/context_quality_test.go`
- `internal/domain/scoring/predictability.go`
- `internal/domain/scoring/predictability_test.go`

### Modified Files (10)
- `internal/domain/ports.go` — enhanced Function, Param, ErrorCall, TypeAssert, AnalyzedFile
- `internal/adapters/outbound/parser/go_parser.go` — full rewrite to extract all new AST data
- `internal/adapters/outbound/parser/go_parser_test.go` — new parser tests
- `internal/domain/config.go` — new ValidCategories, ValidSubMetrics, DefaultConfigForType
- `internal/domain/config_test.go` — updated category name assertions
- `internal/application/score_service.go` — call new scorers
- `internal/application/score_service_test.go` — updated assertions
- `internal/adapters/outbound/tui/renderer_test.go` — updated category name assertions
- `internal/adapters/inbound/cli/init.go` — updated config template
- `tests/e2e/e2e_test.go` — updated assertions for new categories

### Deleted Files (12)
- `internal/domain/scoring/architecture.go` + test
- `internal/domain/scoring/conventions.go` + test
- `internal/domain/scoring/patterns.go` + test
- `internal/domain/scoring/tests.go` + test
- `internal/domain/scoring/ai_context.go` + test
- `internal/domain/scoring/completeness.go` + test

### New Dependencies (1)
- `github.com/fatih/camelcase` — CamelCase word splitting

### Shared Helpers

Some scoring logic is reused across scorers (e.g., `vagueWords`, `specificPrefixes`, CamelCase analysis). Create a shared file:

- `internal/domain/scoring/naming.go` — naming analysis helpers used by both discoverability and predictability scorers. Contains `vagueWords`, `specificPrefixes`, `wordCountScore()`, `vocabularySpecificity()`, `ShannonEntropy()`.
