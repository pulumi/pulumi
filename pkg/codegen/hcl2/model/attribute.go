package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// Attribute represents an HCL2 attribute.
type Attribute = model.Attribute

// BindAttribute binds an HCL2 attribute using the given scope and token map.
func BindAttribute(attribute *hclsyntax.Attribute, scope *Scope, tokens syntax.TokenMap, opts ...BindOption) (*Attribute, hcl.Diagnostics) {
	return model.BindAttribute(attribute, scope, tokens, opts...)
}

