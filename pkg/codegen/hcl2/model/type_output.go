package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// OutputType represents eventual values that carry additional application-specific information.
type OutputType = model.OutputType

// NewOutputType creates a new output type with the given element type after replacing any output or promise types
// within the element type with their respective element types.
func NewOutputType(elementType Type) *OutputType {
	return model.NewOutputType(elementType)
}

