package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// PromiseType represents eventual values that do not carry additional information.
type PromiseType = model.PromiseType

// NewPromiseType creates a new promise type with the given element type after replacing any promise types within
// the element type with their respective element types.
func NewPromiseType(elementType Type) *PromiseType {
	return model.NewPromiseType(elementType)
}

