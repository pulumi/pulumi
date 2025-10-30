package autonaming

import autonaming "github.com/pulumi/pulumi/sdk/v3/pkg/resource/autonaming"

// Autonamer resolves custom autonaming options for a given resource URN.
type Autonamer = autonaming.Autonamer

// Pattern is an autonaming config that uses a pattern to generate a name.
type Pattern = autonaming.Pattern

// Provider represents the configuration for a provider
type Provider = autonaming.Provider

// Global represents the root configuration object for Pulumi autonaming
type Global = autonaming.Global

// Default is the default autonaming strategy, which is equivalent to
// no custom autonaming.
func Default() Autonamer {
	return autonaming.Default()
}

// Verbatim is an autonaming config that enforces the use of a
// logical resource name as the physical resource name literally, with no transformations.
func Verbatim() Autonamer {
	return autonaming.Verbatim()
}

// Disabled is an autonaming config that disables autonaming altogether.
func Disabled() Autonamer {
	return autonaming.Disabled()
}

