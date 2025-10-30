package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// ConstType represents a type that is a single constant value.
type ConstType = model.ConstType

// NewConstType creates a new constant type with the given type and value.
func NewConstType(typ Type, value cty.Value) *ConstType {
	return model.NewConstType(typ, value)
}

func IsConstType(t Type) bool {
	return model.IsConstType(t)
}

