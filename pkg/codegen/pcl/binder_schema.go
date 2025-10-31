package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

type PackageInfo = pcl.PackageInfo

type PackageCache = pcl.PackageCache

// Lookup a PCL invoke token in a schema.
func LookupFunction(pkg schema.PackageReference, token string) (*schema.Function, bool, error) {
	return pcl.LookupFunction(pkg, token)
}

// Lookup a PCL resource token in a schema.
func LookupResource(pkg schema.PackageReference, token string) (*schema.Resource, bool, error) {
	return pcl.LookupResource(pkg, token)
}

func NewPackageCache() *PackageCache {
	return pcl.NewPackageCache()
}

// GetSchemaForType extracts the schema.Type associated with a model.Type, if any.
// 
// The result may be a *schema.UnionType if multiple schema types are associated with the input type.
func GetSchemaForType(t model.Type) (schema.Type, bool) {
	return pcl.GetSchemaForType(t)
}

// GetDiscriminatedUnionObjectMapping calculates a map of type names to object types for a given
// union type.
func GetDiscriminatedUnionObjectMapping(t *model.UnionType) map[string]model.Type {
	return pcl.GetDiscriminatedUnionObjectMapping(t)
}

// EnumMember returns the name of the member that matches the given `value`. If
// no member if found, (nil, true) returned. If the query is nonsensical, either
// because no schema is associated with the EnumMember or if the type of value
// mismatches the type of the schema, (nil, false) is returned.
func EnumMember(t *model.EnumType, value cty.Value) (*schema.Enum, bool) {
	return pcl.EnumMember(t, value)
}

// GenEnum is a helper function when generating an enum.
// Given an enum, and instructions on what to do when you find a known value,
// and an unknown value, return a function that will generate an the given enum
// from the given expression.
// 
// This function should probably live in the `codegen` namespace, but cannot
// because of import cycles.
func GenEnum(t *model.EnumType, from model.Expression, safeEnum func(*schema.Enum), unsafeEnum func(model.Expression)) *hcl.Diagnostic {
	return pcl.GenEnum(t, from, safeEnum, unsafeEnum)
}

