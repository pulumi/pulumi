package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// Options that affect the validation of the packgae schema.
type ValidationOptions = schema.ValidationOptions

var MetaSchema = schema.MetaSchema

// BindSpec converts a serializable PackageSpec into a Package. Any semantic errors encountered during binding are
// contained in the returned diagnostics. The returned error is only non-nil if a fatal error was encountered.
func BindSpec(spec PackageSpec, loader Loader, options ValidationOptions) (*Package, hcl.Diagnostics, error) {
	return schema.BindSpec(spec, loader, options)
}

// ImportSpec converts a serializable PackageSpec into a Package. Unlike BindSpec, ImportSpec does not validate its
// input against the Pulumi package metaschema. ImportSpec should only be used to load packages that are assumed to be
// well-formed (e.g. packages referenced for program code generation or by a root package being used for SDK
// generation). BindSpec should be used to load and validate a package spec prior to generating its SDKs.
func ImportSpec(spec PackageSpec, languages map[string]Language, options ValidationOptions) (*Package, error) {
	return schema.ImportSpec(spec, languages, options)
}

// ImportPartialSpec converts a serializable PartialPackageSpec into a PartialPackage. Unlike a typical Package, a
// PartialPackage loads and binds its members on-demand rather than at import time. This is useful when the entire
// contents of a package are not needed (e.g. for referenced packages).
func ImportPartialSpec(spec PartialPackageSpec, languages map[string]Language, loader Loader) (*PartialPackage, error) {
	return schema.ImportPartialSpec(spec, languages, loader)
}

