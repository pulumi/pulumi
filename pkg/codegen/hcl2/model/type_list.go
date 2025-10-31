package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// ListType represents lists of particular element types.
type ListType = model.ListType

// NewListType creates a new list type with the given element type.
func NewListType(elementType Type) *ListType {
	return model.NewListType(elementType)
}

