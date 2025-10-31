package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// EnumType represents values of a single type, and a closed set of possible values.
type EnumType = model.EnumType

func NewEnumType(token string, typ Type, elements []cty.Value, annotations ...any) *EnumType {
	return model.NewEnumType(token, typ, elements, annotations...)
}

