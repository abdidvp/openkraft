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

func New() *GoParser {
	return &GoParser{}
}

func (p *GoParser) AnalyzeFile(filePath string) (*domain.AnalyzedFile, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, filePath, nil, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	result := &domain.AnalyzedFile{
		Path:    filePath,
		Package: file.Name.Name,
	}

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, path)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Type.(type) {
				case *ast.StructType:
					result.Structs = append(result.Structs, ts.Name.Name)
				case *ast.InterfaceType:
					result.Interfaces = append(result.Interfaces, ts.Name.Name)
				}
			}
		case *ast.FuncDecl:
			f := domain.Function{Name: decl.Name.Name}
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				f.Receiver = receiverType(decl.Recv.List[0].Type)
			}
			result.Functions = append(result.Functions, f)
		}
		return true
	})

	return result, nil
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
