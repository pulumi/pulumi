package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// ObjectType represents schematized maps from strings to particular types.
type ObjectType = model.ObjectType

// NewObjectType creates a new object type with the given properties and annotations.
func NewObjectType(properties map[string]Type, annotations ...any) *ObjectType {
	return model.NewObjectType(properties, annotations...)
}

// GetObjectTypeAnnotation retrieves an annotation of the given type from the object type, if one exists.
func GetObjectTypeAnnotation(t *ObjectType) (T, bool) {
	return model.GetObjectTypeAnnotation(t)
}

