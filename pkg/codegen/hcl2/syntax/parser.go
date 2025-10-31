package syntax

import syntax "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/syntax"

// File represents a single parsed HCL2 source file.
type File = syntax.File

// Parser is a parser for HCL2 source files.
type Parser = syntax.Parser

// NewParser creates a new HCL2 parser.
func NewParser() *Parser {
	return syntax.NewParser()
}

// NewDiagnosticWriter creates a new diagnostic writer for the given list of HCL2 files.
func NewDiagnosticWriter(w io.Writer, width uint, color bool) hcl.DiagnosticWriter {
	return syntax.NewDiagnosticWriter(w, width, color)
}

// ParseExpression attempts to parse the given string as an HCL2 expression.
func ParseExpression(expression, filename string, start hcl.Pos) (hclsyntax.Expression, TokenMap, hcl.Diagnostics) {
	return syntax.ParseExpression(expression, filename, start)
}

