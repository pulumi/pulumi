package providers

import providers "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/providers"

// Reference represents a reference to a particular provider.
type Reference = providers.Reference

// UnknownID is a distinguished token used to indicate that a provider's ID is not known (e.g. because we are
// performing a preview).
const UnknownID = providers.UnknownID

// UnconfiguredID is a distinguished token used to indicate that a provider doesn't yet have an ID because it hasn't
// been configured yet. This should never be returned back to SDKs by the engine but is used for internal tracking so we
// maximally reuse provider instances but only configure them once.
const UnconfiguredID = providers.UnconfiguredID

// IsProviderType returns true if the supplied type token refers to a Pulumi provider.
func IsProviderType(typ tokens.Type) bool {
	return providers.IsProviderType(typ)
}

// IsDefaultProvider returns true if this URN refers to a default Pulumi provider.
func IsDefaultProvider(urn resource.URN) bool {
	return providers.IsDefaultProvider(urn)
}

// MakeProviderType returns the provider type token for the given package.
func MakeProviderType(pkg tokens.Package) tokens.Type {
	return providers.MakeProviderType(pkg)
}

// GetProviderPackage returns the provider package for the given type token.
func GetProviderPackage(typ tokens.Type) tokens.Package {
	return providers.GetProviderPackage(typ)
}

// DenyDefaultProvider represent a default provider that cannot be created.
func NewDenyDefaultProvider(name string) Reference {
	return providers.NewDenyDefaultProvider(name)
}

// Retrieves the package of the denied provider.
// 
// For example, if a reference to:
// "urn:pulumi:stack::project::pulumi:providers:aws::default_4_35_0"
// was denied, then GetDeniedDefaultProviderPkg would return "aws".
// 
// Panics if called on a provider that is not a DenyDefaultProvider.
func GetDeniedDefaultProviderPkg(ref Reference) string {
	return providers.GetDeniedDefaultProviderPkg(ref)
}

func IsDenyDefaultsProvider(ref Reference) bool {
	return providers.IsDenyDefaultsProvider(ref)
}

// NewReference creates a new reference for the given URN and ID.
func NewReference(urn resource.URN, id resource.ID) (Reference, error) {
	return providers.NewReference(urn, id)
}

// ParseReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns false.
func ParseReference(s string) (Reference, error) {
	return providers.ParseReference(s)
}

