package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// UnionType represents values that may be any one of a specified set of types.
type UnionType = model.UnionType

// NewUnionTypeAnnotated creates a new union type with the given element types and annotations.
// NewUnionTypeAnnotated enforces 3 properties on the returned type:
// 1. Any element types that are union types are replaced with their element types.
// 2. Any duplicate types are removed.
// 3. Unions have have more then 1 type. If only a single type is left after (1) and (2),
// it is returned as is.
func NewUnionTypeAnnotated(types []Type, annotations ...any) Type {
	return model.NewUnionTypeAnnotated(types, annotations...)
}

// NewUnionType creates a new union type with the given element types. Any element types that are union types are
// replaced with their element types.
func NewUnionType(types ...Type) Type {
	return model.NewUnionType(types...)
}

// NewOptionalType returns a new union(T, None).
func NewOptionalType(t Type) Type {
	return model.NewOptionalType(t)
}

// IsOptionalType returns true if t is an optional type.
func IsOptionalType(t Type) bool {
	return model.IsOptionalType(t)
}

