package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// A ResourceSpec specifies the subset of a resource's state that is relevant to fuzzing snapshot integrity issues.
// Generally this encompasses enough to identify a resource (URN, ID, and so on) and any dependencies it may have on
// others.
type ResourceSpec = fuzzing.ResourceSpec

// A set of options for configuring the generation of a ResourceSpec.
type ResourceSpecOptions = fuzzing.ResourceSpecOptions

// A rapid.Generator that yields random provider types.
// 
// 	GeneratedProviderType.Draw(t, "ProviderType") = "pulumi:providers:<pkg>"
var GeneratedProviderType = fuzzing.GeneratedProviderType

// A rapid.Generator that yields random resource names.
// 
// 	GeneratedResourceName.Draw(t, "ResourceName") = "res-<random>"
var GeneratedResourceName = fuzzing.GeneratedResourceName

// A rapid.Generator that yields random resource IDs.
// 
// 	GeneratedResourceID.Draw(t, "ResourceID") = "id-<random>"
var GeneratedResourceID = fuzzing.GeneratedResourceID

// Creates a ResourceSpec from the given resource.State.
func FromResource(r *resource.State) *ResourceSpec {
	return fuzzing.FromResource(r)
}

// Creates a ResourceSpec from the given ResourceV3.
func FromResourceV3(r apitype.ResourceV3) *ResourceSpec {
	return fuzzing.FromResourceV3(r)
}

// AddTag adds the given tag to the given ResourceSpec. Ideally this would be a generic method on ResourceSpec itself,
// but Go doesn't support generic methods yet.
func AddTag(r *ResourceSpec, tag T) {
	fuzzing.AddTag(r, tag)
}

// Given a package name, returns a rapid.Generator that yields random resource types within that package.
// 
// 	GeneratedResourceType("pkg-xyz").Draw(t, "ResourceType") = "pkg-xyz:<mod>:<type>"
func GeneratedResourceType(pkg tokens.Package) *interface{} {
	return fuzzing.GeneratedResourceType(pkg)
}

// Given a set of StackSpecOptions, returns a rapid.Generator that yields random provider ResourceSpecs with no
// dependencies. Provider resources are always custom and never deleted.
func GeneratedProviderResourceSpec(sso StackSpecOptions) *interface{} {
	return fuzzing.GeneratedProviderResourceSpec(sso)
}

// Given a set of StackSpecOptions, ResourceSpecOptions, and a map of package names to provider resources, returns a
// rapid.Generator that yields random ResourceSpecs with no dependencies.
func GeneratedResourceSpec(sso StackSpecOptions, rso ResourceSpecOptions, provs map[tokens.Package]*ResourceSpec) *interface{} {
	return fuzzing.GeneratedResourceSpec(sso, rso, provs)
}

