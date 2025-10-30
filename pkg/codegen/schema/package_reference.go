package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// A PackageReference represents a references Pulumi Package. Applications that do not need access to the entire
// definition of a Pulumi Package should use PackageReference rather than Package, as the former uses memory more
// efficiently than the latter by binding package members on-demand.
type PackageReference = schema.PackageReference

// PackageTypes provides random and sequential access to a package's types.
type PackageTypes = schema.PackageTypes

// TypesIter is an iterator for ranging over a package's types. See PackageTypes.Range.
type TypesIter = schema.TypesIter

// PackageResources provides random and sequential access to a package's resources.
type PackageResources = schema.PackageResources

// ResourcesIter is an iterator for ranging over a package's resources. See PackageResources.Range.
type ResourcesIter = schema.ResourcesIter

// PackageFunctions provides random and sequential access to a package's functions.
type PackageFunctions = schema.PackageFunctions

// FunctionsIter is an iterator for ranging over a package's functions. See PackageFunctions.Range.
type FunctionsIter = schema.FunctionsIter

// PartialPackage is an implementation of PackageReference that loads and binds package members on demand. A
// PartialPackage is backed by a PartialPackageSpec, which leaves package members in their JSON-encoded form until
// they are required. PartialPackages are created using ImportPartialSpec.
type PartialPackage = schema.PartialPackage

