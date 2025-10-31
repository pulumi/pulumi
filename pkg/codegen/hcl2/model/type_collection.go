package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// GetCollectionTypes returns the key and value types of the given type if it is a collection.
func GetCollectionTypes(collectionType Type, rng hcl.Range, strict bool) (Type, Type, hcl.Diagnostics) {
	return model.GetCollectionTypes(collectionType, rng, strict)
}

