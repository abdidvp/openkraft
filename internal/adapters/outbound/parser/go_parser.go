package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// GoParser implements domain.CodeAnalyzer using go/ast.
type GoParser struct{}

func New() *GoParser { return &GoParser{} }

func (p *GoParser) AnalyzeFile(filePath string) (*domain.AnalyzedFile, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, filePath, src, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	result := &domain.AnalyzedFile{
		Path:       filePath,
		Package:    file.Name.Name,
		PackageDoc: file.Doc != nil && len(file.Doc.List) > 0,
	}

	// Total lines in the file.
	if f := fset.File(file.Pos()); f != nil {
		result.TotalLines = f.LineCount()
	}

	// Detect generated code via comment markers or filename conventions.
	result.IsGenerated = isGeneratedFile(file) || isGeneratedFilename(filePath)

	// Imports.
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, path)
		if path == "C" {
			result.HasCGoImport = true
		}
	}

	// Walk top-level declarations.
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			p.processGenDecl(d, result)
		case *ast.FuncDecl:
			fn := p.processFunc(d, fset)
			result.Functions = append(result.Functions, fn)
			if d.Name.Name == "init" {
				result.InitFunctions++
			}
		}
	}

	// Error calls and type assertions require a deep walk.
	result.ErrorCalls = extractErrorCalls(file)
	result.TypeAssertions = extractTypeAssertions(file)

	// Normalized tokens for duplication detection.
	result.NormalizedTokens = normalizeTokens(src)

	return result, nil
}

// processGenDecl extracts struct/interface declarations and package-level variables.
func (p *GoParser) processGenDecl(decl *ast.GenDecl, result *domain.AnalyzedFile) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			switch itype := s.Type.(type) {
			case *ast.StructType:
				result.Structs = append(result.Structs, s.Name.Name)
			case *ast.InterfaceType:
				result.Interfaces = append(result.Interfaces, s.Name.Name)
				idef := domain.InterfaceDef{Name: s.Name.Name}
				if itype.Methods != nil {
					for _, method := range itype.Methods.List {
						if len(method.Names) > 0 {
							idef.Methods = append(idef.Methods, method.Names[0].Name)
						}
					}
				}
				result.InterfaceDefs = append(result.InterfaceDefs, idef)
			}
		case *ast.ValueSpec:
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

// processFunc extracts a rich Function representation from a function declaration.
func (p *GoParser) processFunc(decl *ast.FuncDecl, fset *token.FileSet) domain.Function {
	f := domain.Function{
		Name:     decl.Name.Name,
		Exported: decl.Name.IsExported(),
	}

	// Line numbers.
	f.LineStart = fset.Position(decl.Pos()).Line
	f.LineEnd = fset.Position(decl.End()).Line

	// Receiver.
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		f.Receiver = receiverType(decl.Recv.List[0].Type)
	}

	// Parameters.
	if decl.Type.Params != nil {
		for _, field := range decl.Type.Params.List {
			typeName := exprToString(field.Type)
			if len(field.Names) == 0 {
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

	// Return types.
	if decl.Type.Results != nil {
		for _, field := range decl.Type.Results.List {
			f.Returns = append(f.Returns, exprToString(field.Type))
		}
	}

	// Nesting depth, conditional complexity, and cognitive complexity.
	if decl.Body != nil {
		f.MaxNesting = maxNestingDepth(decl.Body, 0)
		f.MaxCondOps = maxConditionalOps(decl.Body)
		f.CognitiveComplexity = cognitiveComplexity(decl.Body)
		lines := f.LineEnd - f.LineStart + 1
		f.StringLiteralRatio = stringLiteralRatio(fset, decl.Body, lines)
		f.MaxCaseArms, f.AvgCaseLines = switchDispatchMetrics(fset, decl.Body)
	}

	return f
}

// --- Nesting depth ---

// maxNestingDepth returns the deepest nesting level within a block.
func maxNestingDepth(block *ast.BlockStmt, depth int) int {
	max := depth
	for _, stmt := range block.List {
		if d := nestingForStmt(stmt, depth); d > max {
			max = d
		}
	}
	return max
}

func nestingForStmt(stmt ast.Stmt, depth int) int {
	max := depth
	switch s := stmt.(type) {
	case *ast.IfStmt:
		inner := depth + 1
		if inner > max {
			max = inner
		}
		if s.Body != nil {
			if d := maxNestingDepth(s.Body, inner); d > max {
				max = d
			}
		}
		if s.Else != nil {
			if d := nestingForStmt(s.Else, depth); d > max {
				max = d
			}
		}
	case *ast.ForStmt:
		inner := depth + 1
		if inner > max {
			max = inner
		}
		if s.Body != nil {
			if d := maxNestingDepth(s.Body, inner); d > max {
				max = d
			}
		}
	case *ast.RangeStmt:
		inner := depth + 1
		if inner > max {
			max = inner
		}
		if s.Body != nil {
			if d := maxNestingDepth(s.Body, inner); d > max {
				max = d
			}
		}
	case *ast.SwitchStmt:
		inner := depth + 1
		if inner > max {
			max = inner
		}
		if s.Body != nil {
			if d := maxNestingDepth(s.Body, inner); d > max {
				max = d
			}
		}
	case *ast.TypeSwitchStmt:
		inner := depth + 1
		if inner > max {
			max = inner
		}
		if s.Body != nil {
			if d := maxNestingDepth(s.Body, inner); d > max {
				max = d
			}
		}
	case *ast.SelectStmt:
		inner := depth + 1
		if inner > max {
			max = inner
		}
		if s.Body != nil {
			if d := maxNestingDepth(s.Body, inner); d > max {
				max = d
			}
		}
	case *ast.BlockStmt:
		if d := maxNestingDepth(s, depth); d > max {
			max = d
		}
	case *ast.CaseClause:
		for _, body := range s.Body {
			if d := nestingForStmt(body, depth); d > max {
				max = d
			}
		}
	case *ast.CommClause:
		for _, body := range s.Body {
			if d := nestingForStmt(body, depth); d > max {
				max = d
			}
		}
	}
	return max
}

// --- Conditional complexity ---

// maxConditionalOps returns the highest number of &&/|| operators in any
// single if-condition within the block.
func maxConditionalOps(block *ast.BlockStmt) int {
	max := 0
	ast.Inspect(block, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		if ops := countBoolOps(ifStmt.Cond); ops > max {
			max = ops
		}
		return true
	})
	return max
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

// --- Cognitive complexity (Sonar algorithm) ---

// cognitiveComplexity computes the cognitive complexity of a function body
// following the SonarQube specification adapted for Go:
//
//   - +1 with nesting increment for: if (not else-if), for, range, switch, typeswitch, select
//   - +1 without nesting increment for: else if, else, goto, labeled break/continue
//   - Nesting level increases inside: if, else if, else, for, range, switch, typeswitch, select, func literals
//   - Boolean operator sequences: +1 per sequence of identical operators; +1 per operator type transition
func cognitiveComplexity(body *ast.BlockStmt) int {
	s := &cogState{}
	s.walkBlock(body)
	return s.score
}

type cogState struct {
	score    int
	nesting  int
}

func (s *cogState) walkBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}
	for _, stmt := range block.List {
		s.walkStmt(stmt)
	}
}

func (s *cogState) walkStmt(stmt ast.Stmt) {
	switch st := stmt.(type) {
	case *ast.IfStmt:
		s.walkIf(st, true)

	case *ast.ForStmt:
		s.score += 1 + s.nesting // +1 with nesting penalty
		if st.Cond != nil {
			s.walkBoolOps(st.Cond) // boolean operators in for-condition
		}
		s.nesting++
		s.walkBlock(st.Body)
		s.nesting--

	case *ast.RangeStmt:
		s.score += 1 + s.nesting
		s.nesting++
		s.walkBlock(st.Body)
		s.nesting--

	case *ast.SwitchStmt:
		s.score += 1 + s.nesting
		// Note: st.Tag is not a boolean expression (it's the switch value),
		// so walkBoolOps is not called here. Boolean ops in case clauses
		// are not penalized per the SonarQube spec.
		s.nesting++
		s.walkBlock(st.Body)
		s.nesting--

	case *ast.TypeSwitchStmt:
		s.score += 1 + s.nesting
		s.nesting++
		s.walkBlock(st.Body)
		s.nesting--

	case *ast.SelectStmt:
		s.score += 1 + s.nesting
		s.nesting++
		s.walkBlock(st.Body)
		s.nesting--

	case *ast.BranchStmt:
		if st.Tok == token.GOTO {
			s.score++ // +1, no nesting
		} else if st.Label != nil && (st.Tok == token.BREAK || st.Tok == token.CONTINUE) {
			s.score++ // labeled break/continue +1, no nesting
		}

	case *ast.BlockStmt:
		s.walkBlock(st)

	case *ast.CaseClause:
		for _, body := range st.Body {
			s.walkStmt(body)
		}

	case *ast.CommClause:
		for _, body := range st.Body {
			s.walkStmt(body)
		}

	case *ast.LabeledStmt:
		s.walkStmt(st.Stmt)

	case *ast.ExprStmt:
		s.walkExprForFuncLiterals(st.X)

	case *ast.AssignStmt:
		for _, rhs := range st.Rhs {
			s.walkExprForFuncLiterals(rhs)
		}

	case *ast.ReturnStmt:
		for _, r := range st.Results {
			s.walkExprForFuncLiterals(r)
		}

	case *ast.DeferStmt:
		s.walkExprForFuncLiterals(st.Call)

	case *ast.GoStmt:
		s.walkExprForFuncLiterals(st.Call)

	case *ast.SendStmt:
		// no control flow

	case *ast.DeclStmt:
		// variable declarations may contain func literals
		if gd, ok := st.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, v := range vs.Values {
						s.walkExprForFuncLiterals(v)
					}
				}
			}
		}
	}
}

// walkIf handles if/else-if/else chains. The first if in a chain gets +1
// with nesting penalty. Subsequent else-if and else get +1 without nesting penalty.
func (s *cogState) walkIf(stmt *ast.IfStmt, isFirst bool) {
	if isFirst {
		s.score += 1 + s.nesting // +1 with nesting
	} else {
		s.score++ // else-if: +1, no nesting penalty
	}

	// Count boolean operator sequences in condition
	if stmt.Cond != nil {
		s.walkBoolOps(stmt.Cond)
	}

	s.nesting++
	s.walkBlock(stmt.Body)
	s.nesting--

	// Handle else/else-if
	if stmt.Else != nil {
		switch elseStmt := stmt.Else.(type) {
		case *ast.IfStmt:
			s.walkIf(elseStmt, false) // else-if continues the chain
		case *ast.BlockStmt:
			s.score++ // else: +1, no nesting penalty
			s.nesting++
			s.walkBlock(elseStmt)
			s.nesting--
		}
	}
}

// walkBoolOps counts +1 per sequence of identical boolean operators,
// and +1 per transition between && and ||.
// "a && b && c" = 1 sequence. "a && b || c" = 2 sequences.
func (s *cogState) walkBoolOps(expr ast.Expr) {
	ops := flattenBoolOps(expr)
	if len(ops) == 0 {
		return
	}
	s.score++ // first sequence
	for i := 1; i < len(ops); i++ {
		if ops[i] != ops[i-1] {
			s.score++ // operator transition
		}
	}
}

// flattenBoolOps collects the sequence of &&/|| operators in left-to-right order.
func flattenBoolOps(expr ast.Expr) []token.Token {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	if bin.Op != token.LAND && bin.Op != token.LOR {
		return nil
	}
	var ops []token.Token
	ops = append(ops, flattenBoolOps(bin.X)...)
	ops = append(ops, bin.Op)
	ops = append(ops, flattenBoolOps(bin.Y)...)
	return ops
}

// walkExprForFuncLiterals walks an expression tree to find func literals,
// which increase nesting level for cognitive complexity.
func (s *cogState) walkExprForFuncLiterals(expr ast.Expr) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			s.nesting++
			s.walkBlock(fl.Body)
			s.nesting--
			return false // don't recurse into children again
		}
		return true
	})
}

// --- Error calls ---

// extractErrorCalls finds fmt.Errorf and errors.New invocations.
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

// --- Type assertions ---

// extractTypeAssertions finds type assertion expressions and checks safety.
func extractTypeAssertions(file *ast.File) []domain.TypeAssert {
	var asserts []domain.TypeAssert
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			if _, isTA := rhs.(*ast.TypeAssertExpr); isTA {
				asserts = append(asserts, domain.TypeAssert{
					Safe: len(assign.Lhs) == 2,
				})
			}
		}
		return true
	})
	return asserts
}

// --- Generated code detection ---

// isGeneratedFile checks whether any comment group contains a "Code generated ... DO NOT EDIT"
// marker, following the Go convention established by go generate.
// Checks all comment groups, not just the first, to handle files where
// a copyright header precedes the generated-code marker.
func isGeneratedFile(file *ast.File) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "Code generated") && strings.Contains(c.Text, "DO NOT EDIT") {
				return true
			}
		}
	}
	return false
}

// isGeneratedFilename detects generated files by filename convention.
// Matches *_gen.go and *.pb.go but NOT *_gen_test.go (hand-written tests).
func isGeneratedFilename(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return false
	}
	return strings.HasSuffix(base, "_gen.go") || strings.HasSuffix(base, ".pb.go")
}

// --- String literal ratio ---

// stringLiteralRatio computes the fraction of function body lines occupied
// by string literal tokens. Functions dominated by string literals (>80%)
// are typically template holders (e.g., shell completion scripts) rather
// than logic, and deserve relaxed size thresholds.
func stringLiteralRatio(fset *token.FileSet, body *ast.BlockStmt, totalLines int) float64 {
	if body == nil || totalLines <= 0 {
		return 0
	}
	var literalLines int
	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if ok && lit.Kind == token.STRING {
			start := fset.Position(lit.Pos()).Line
			end := fset.Position(lit.End()).Line
			literalLines += end - start + 1
		}
		return true
	})
	ratio := float64(literalLines) / float64(totalLines)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return ratio
}

// --- Switch dispatch detection ---

// switchDispatchMetrics finds the switch statement with the most case arms
// in a function body and returns (maxCaseArms, avgLinesPerCase).
// Used to detect type-switch dispatch functions (e.g., zap's Any(), ollama's String())
// that have zero cognitive complexity but many structurally-identical case arms.
func switchDispatchMetrics(fset *token.FileSet, body *ast.BlockStmt) (int, float64) {
	var maxArms int
	var avgLines float64

	ast.Inspect(body, func(n ast.Node) bool {
		var clauses []ast.Stmt
		switch s := n.(type) {
		case *ast.SwitchStmt:
			if s.Body != nil {
				clauses = s.Body.List
			}
		case *ast.TypeSwitchStmt:
			if s.Body != nil {
				clauses = s.Body.List
			}
		default:
			return true
		}

		arms := len(clauses)
		if arms <= maxArms {
			return true
		}

		// Compute average lines per case clause.
		var totalLines int
		for _, clause := range clauses {
			cc, ok := clause.(*ast.CaseClause)
			if !ok {
				continue
			}
			start := fset.Position(cc.Pos()).Line
			end := fset.Position(cc.End()).Line
			totalLines += end - start + 1
		}

		maxArms = arms
		if arms > 0 {
			avgLines = float64(totalLines) / float64(arms)
		}
		return true
	})

	return maxArms, avgLines
}

// --- Normalized tokens for duplication detection ---

// normalizeTokens tokenizes Go source and replaces identifiers and literals
// with canonical placeholder values so that structurally identical code
// fragments produce the same token sequence regardless of naming.
//
// Normalization rules:
//   - IDENT → -1
//   - STRING → -2, INT → -3, FLOAT → -4, IMAG → -5, CHAR → -6
//   - Comments → skipped
//   - Structural tokens (keywords, operators, delimiters) → int(tok)
func normalizeTokens(src []byte) []int {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, src, nil, 0) // mode 0: skip comments

	var tokens []int
	for {
		_, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		switch {
		case tok == token.IDENT:
			tokens = append(tokens, -1)
		case tok == token.STRING:
			tokens = append(tokens, -2)
		case tok == token.INT:
			tokens = append(tokens, -3)
		case tok == token.FLOAT:
			tokens = append(tokens, -4)
		case tok == token.IMAG:
			tokens = append(tokens, -5)
		case tok == token.CHAR:
			tokens = append(tokens, -6)
		default:
			tokens = append(tokens, int(tok))
		}
	}
	return tokens
}

// --- Helpers ---

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
