package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// ParameterizationDescriptor is the serializable description of a dependency's parameterization.
type ParameterizationDescriptor = schema.ParameterizationDescriptor

// PackageDescriptor is a descriptor for a package, this is similar to a plugin spec but also contains parameterization
// info.
type PackageDescriptor = schema.PackageDescriptor

type Loader = schema.Loader

type ReferenceLoader = schema.ReferenceLoader

// PackageReferenceNameMismatchError is the type of errors returned by LoadPackageReferenceV2 when the name of the
// loaded reference does not match the requested name.
type PackageReferenceNameMismatchError = schema.PackageReferenceNameMismatchError

// PackageReferenceVersionMismatchError is the type of errors returned by LoadPackageReferenceV2 when the version of the
// loaded reference does not match the requested version.
type PackageReferenceVersionMismatchError = schema.PackageReferenceVersionMismatchError

var ErrGetSchemaNotImplemented = schema.ErrGetSchemaNotImplemented

func NewPluginLoader(host plugin.Host) ReferenceLoader {
	return schema.NewPluginLoader(host)
}

// LoadPackageReference loads a package reference for the given pkg+version using the
// given loader.
// 
// Deprecated: use LoadPackageReferenceV2
func LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	return schema.LoadPackageReference(pkg, version)
}

// LoadPackageReferenceV2 loads a package reference for the given descriptor using the given loader. When a reference is
// loaded, the name and version of the reference are compared to the requested name and version. If the name or version
// do not match, a PackageReferenceNameMismatchError or PackageReferenceVersionMismatchError is returned, respectively.
// 
// In the event that a mismatch error is returned, the reference is still returned. This is to allow for the caller to
// decide whether or not the mismatch impacts their use of the reference.
func LoadPackageReferenceV2(ctx context.Context, descriptor *PackageDescriptor) (PackageReference, error) {
	return schema.LoadPackageReferenceV2(ctx, descriptor)
}

