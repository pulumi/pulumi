package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// Block represents an HCL2 block.
type Block = model.Block

// BindBlock binds an HCL2 block using the given scopes and token map.
func BindBlock(block *hclsyntax.Block, scopes Scopes, tokens syntax.TokenMap, opts ...BindOption) (*Block, hcl.Diagnostics) {
	return model.BindBlock(block, scopes, tokens, opts...)
}

