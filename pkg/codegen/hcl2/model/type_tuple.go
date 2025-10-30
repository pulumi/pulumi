package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// TupleType represents values that are a sequence of independently-typed elements.
type TupleType = model.TupleType

// NewTupleType creates a new tuple type with the given element types.
func NewTupleType(elementTypes ...Type) Type {
	return model.NewTupleType(elementTypes...)
}

