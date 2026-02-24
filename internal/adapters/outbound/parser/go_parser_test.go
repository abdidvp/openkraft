package parser_test

import (
	"testing"

	"github.com/openkraft/openkraft/internal/adapters/outbound/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const taxRulePath = "../../../../testdata/go-hexagonal/perfect/internal/tax/domain/tax_rule.go"
const taxPortsPath = "../../../../testdata/go-hexagonal/perfect/internal/tax/application/tax_ports.go"

func TestGoParser_FindsStructs(t *testing.T) {
	p := parser.New()
	result, err := p.AnalyzeFile(taxRulePath)
	require.NoError(t, err)

	assert.Contains(t, result.Structs, "TaxRule")
}

func TestGoParser_FindsFunctions(t *testing.T) {
	p := parser.New()
	result, err := p.AnalyzeFile(taxRulePath)
	require.NoError(t, err)

	funcNames := make([]string, len(result.Functions))
	for i, f := range result.Functions {
		funcNames[i] = f.Name
	}
	assert.Contains(t, funcNames, "NewTaxRule")
}

func TestGoParser_FindsMethods(t *testing.T) {
	p := parser.New()
	result, err := p.AnalyzeFile(taxRulePath)
	require.NoError(t, err)

	var validateFunc *struct{ name, receiver string }
	for _, f := range result.Functions {
		if f.Name == "Validate" {
			validateFunc = &struct{ name, receiver string }{f.Name, f.Receiver}
			break
		}
	}
	require.NotNil(t, validateFunc, "should find Validate method")
	assert.Equal(t, "*TaxRule", validateFunc.receiver)
}

func TestGoParser_FindsInterfaces(t *testing.T) {
	p := parser.New()
	result, err := p.AnalyzeFile(taxPortsPath)
	require.NoError(t, err)

	assert.Contains(t, result.Interfaces, "TaxRuleRepository")
}

func TestGoParser_FindsImports(t *testing.T) {
	p := parser.New()
	result, err := p.AnalyzeFile(taxRulePath)
	require.NoError(t, err)

	assert.Contains(t, result.Imports, "errors")
	assert.Contains(t, result.Imports, "time")
}

func TestGoParser_PackageName(t *testing.T) {
	p := parser.New()
	result, err := p.AnalyzeFile(taxRulePath)
	require.NoError(t, err)

	assert.Equal(t, "domain", result.Package)
}
