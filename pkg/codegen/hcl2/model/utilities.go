package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// SourceOrderLess returns true if the first range precedes the second when ordered by source position. Positions are
// ordered first by filename, then by byte offset.
func SourceOrderLess(a, b hcl.Range) bool {
	return model.SourceOrderLess(a, b)
}

// SourceOrderBody sorts the contents of an HCL2 body in source order.
func SourceOrderBody(body *hclsyntax.Body) []hclsyntax.Node {
	return model.SourceOrderBody(body)
}

func VariableReference(v *Variable) *ScopeTraversalExpression {
	return model.VariableReference(v)
}

func ConstantReference(c *Constant) *ScopeTraversalExpression {
	return model.ConstantReference(c)
}

