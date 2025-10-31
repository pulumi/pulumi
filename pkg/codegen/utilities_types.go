package codegen

import codegen "github.com/pulumi/pulumi/sdk/v3/pkg/codegen"

func VisitType(schemaType schema.Type, visitor func(schema.Type)) {
	codegen.VisitType(schemaType, visitor)
}

func VisitTypeClosure(properties []*schema.Property, visitor func(schema.Type)) {
	codegen.VisitTypeClosure(properties, visitor)
}

func SimplifyInputUnion(t schema.Type) schema.Type {
	return codegen.SimplifyInputUnion(t)
}

// RequiredType unwraps the OptionalType enclosing the Property's type, if any.
func RequiredType(p *schema.Property) schema.Type {
	return codegen.RequiredType(p)
}

// OptionalType wraps the Property's type in an OptionalType if it is not already optional.
func OptionalType(p *schema.Property) schema.Type {
	return codegen.OptionalType(p)
}

// UnwrapType removes any outer OptionalTypes and InputTypes from t.
func UnwrapType(t schema.Type) schema.Type {
	return codegen.UnwrapType(t)
}

// MapInnerType applies f to the first non-wrapper type in t.
// MapInnerType does not mutate it's input, and t should not either.
func MapInnerType(t schema.Type, f func(schema.Type) schema.Type) schema.Type {
	return codegen.MapInnerType(t, f)
}

// Applies f to the first non-optional type in t.
// If t is Optional{v} then returns Optional{f(v)}, otherwise f(t) is returned
func MapOptionalType(t schema.Type, f func(schema.Type) schema.Type) schema.Type {
	return codegen.MapOptionalType(t, f)
}

func IsNOptionalInput(t schema.Type) bool {
	return codegen.IsNOptionalInput(t)
}

// PlainType deeply removes any InputTypes from t, with the exception of argument structs. Use ResolvedType to
// unwrap argument structs as well.
func PlainType(t schema.Type) schema.Type {
	return codegen.PlainType(t)
}

// ResolvedType deeply removes any InputTypes from t.
func ResolvedType(t schema.Type) schema.Type {
	return codegen.ResolvedType(t)
}

// If a helper function needs to be invoked to provide default values for a
// plain type. The provided map cannot be reused.
func IsProvideDefaultsFuncRequired(t schema.Type) bool {
	return codegen.IsProvideDefaultsFuncRequired(t)
}

// PackageReferences returns a list of packages that are referenced by the given package.
func PackageReferences(pkg *schema.Package) []schema.PackageReference {
	return codegen.PackageReferences(pkg)
}

