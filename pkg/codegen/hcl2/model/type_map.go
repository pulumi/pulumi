package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// MapType represents maps from strings to particular element types.
type MapType = model.MapType

// NewMapType creates a new map type with the given element type.
func NewMapType(elementType Type) *MapType {
	return model.NewMapType(elementType)
}

