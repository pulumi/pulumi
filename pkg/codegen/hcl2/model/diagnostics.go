package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

func ExprNotConvertible(destType Type, expr Expression) *hcl.Diagnostic {
	return model.ExprNotConvertible(destType, expr)
}

