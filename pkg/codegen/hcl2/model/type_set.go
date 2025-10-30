package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// SetType represents sets of particular element types.
type SetType = model.SetType

// NewSetType creates a new set type with the given element type.
func NewSetType(elementType Type) *SetType {
	return model.NewSetType(elementType)
}

