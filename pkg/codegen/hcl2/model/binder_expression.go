package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

type BindOption = model.BindOption

func AllowMissingVariables(options *bindOptions) {
	model.AllowMissingVariables(options)
}

func SkipRangeTypechecking(options *bindOptions) {
	model.SkipRangeTypechecking(options)
}

// BindExpression binds an HCL2 expression using the given scope and token map.
func BindExpression(syntax hclsyntax.Node, scope *Scope, tokens _syntax.TokenMap, opts ...BindOption) (Expression, hcl.Diagnostics) {
	return model.BindExpression(syntax, scope, tokens, opts...)
}

// BindExpressionText parses and binds an HCL2 expression using the given scope.
func BindExpressionText(source string, scope *Scope, initialPos hcl.Pos, opts ...BindOption) (Expression, hcl.Diagnostics) {
	return model.BindExpressionText(source, scope, initialPos, opts...)
}

// MakeTraverser returns an hcl.TraverseIndex with a key of the appropriate type.
func MakeTraverser(t Type) hcl.TraverseIndex {
	return model.MakeTraverser(t)
}

