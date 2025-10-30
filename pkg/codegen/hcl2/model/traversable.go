package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// Traversable represents an entity that can be traversed by an HCL2 traverser.
type Traversable = model.Traversable

// TypedTraversable is a Traversable that has an associated type.
type TypedTraversable = model.TypedTraversable

// ValueTraversable is a Traversable that has an associated value.
type ValueTraversable = model.ValueTraversable

// GetTraversableType returns the type of the given Traversable:
// - If the Traversable is a TypedTraversable, this returns t.Type()
// - If the Traversable is a Type, this returns t
// - Otherwise, this returns DynamicType
func GetTraversableType(t Traversable) Type {
	return model.GetTraversableType(t)
}

// GetTraverserKey extracts the value and type of the key associated with the given traverser.
func GetTraverserKey(t hcl.Traverser) (cty.Value, Type) {
	return model.GetTraverserKey(t)
}

