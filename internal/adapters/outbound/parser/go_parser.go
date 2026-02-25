package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

// GoParser implements domain.CodeAnalyzer using go/ast.
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

	// Total lines in the file.
	if f := fset.File(file.Pos()); f != nil {
		result.TotalLines = f.LineCount()
	}

	// Imports.
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, path)
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

	// Nesting depth and conditional complexity.
	if decl.Body != nil {
		f.MaxNesting = maxNestingDepth(decl.Body, 0)
		f.MaxCondOps = maxConditionalOps(decl.Body)
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
