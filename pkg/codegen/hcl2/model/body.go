package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// BodyItem represents either an *Attribute or a *Block that is part of an HCL2 Body.
type BodyItem = model.BodyItem

// Body represents an HCL2 body. A Body may be the root of an HCL2 file or the contents of an HCL2 block.
type Body = model.Body

// BindBody binds an HCL2 body using the given scopes and token map.
func BindBody(body *hclsyntax.Body, scopes Scopes, tokens syntax.TokenMap, opts ...BindOption) (*Body, hcl.Diagnostics) {
	return model.BindBody(body, scopes, tokens, opts...)
}

