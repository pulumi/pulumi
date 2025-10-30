package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// A ProviderSpec specifies the behavior of a set of providers that will be mocked in a lifecycle test.
type ProviderSpec = fuzzing.ProviderSpec

// A ProviderCreateSpec specifies the behavior of a provider's create function. It maps resource URNs to the action that
// should be taken if Create is called on that URN. The absence of a URN in the map indicates that the default behavior
// (a successful create) should be taken.
type ProviderCreateSpec = fuzzing.ProviderCreateSpec

// ProviderCreateSpecAction captures the set of actions that can be taken by a Create implementation for a given
// resource.
type ProviderCreateSpecAction = fuzzing.ProviderCreateSpecAction

// A ProviderDeleteSpec specifies the behavior of a provider's delete function. It maps resource URNs to the action that
// should be taken if Delete is called on that URN. The absence of a URN in the map indicates that the default behavior
// (a successful delete) should be taken.
type ProviderDeleteSpec = fuzzing.ProviderDeleteSpec

// ProviderDeleteSpecAction captures the set of actions that can be taken by a Delete implementation for a given
// resource.
type ProviderDeleteSpecAction = fuzzing.ProviderDeleteSpecAction

// A ProviderDiffSpec specifies the behavior of a provider's diff function. It maps resource URNs to the action that
// should be taken if Diff is called on that URN. The absence of a URN in the map indicates that the default behavior (a
// successful diff that reports no changes) should be taken.
type ProviderDiffSpec = fuzzing.ProviderDiffSpec

// ProviderDiffSpecAction captures the set of actions that can be taken by a Diff implementation for a given resource.
type ProviderDiffSpecAction = fuzzing.ProviderDiffSpecAction

// A ProviderReadSpec specifies the behavior of a provider's read function. It maps resource URNs to the action that
// should be taken if Read is called on that URN. The absence of a URN in the map indicates that the default behavior (a
// successful read that reports that the resource exists) should be taken.
type ProviderReadSpec = fuzzing.ProviderReadSpec

// ProviderReadSpecAction captures the set of actions that can be taken by a Read implementation for a given resource.
type ProviderReadSpecAction = fuzzing.ProviderReadSpecAction

// A ProviderUpdateSpec specifies the behavior of a provider's update function. It maps resource URNs to the action that
// should be taken if Update is called on that URN. The absence of a URN in the map indicates that the default behavior
// (a successful update) should be taken.
type ProviderUpdateSpec = fuzzing.ProviderUpdateSpec

// ProviderUpdateSpecAction captures the set of actions that can be taken by an Update implementation for a given
// resource.
type ProviderUpdateSpecAction = fuzzing.ProviderUpdateSpecAction

// A set of options for configuring the generation of a ProviderSpec.
type ProviderSpecOptions = fuzzing.ProviderSpecOptions

const ProviderCreateFailure = fuzzing.ProviderCreateFailure

const ProviderDeleteFailure = fuzzing.ProviderDeleteFailure

const ProviderDiffDeleteBeforeReplace = fuzzing.ProviderDiffDeleteBeforeReplace

const ProviderDiffDeleteAfterReplace = fuzzing.ProviderDiffDeleteAfterReplace

const ProviderDiffChange = fuzzing.ProviderDiffChange

const ProviderDiffFailure = fuzzing.ProviderDiffFailure

const ProviderReadDeleted = fuzzing.ProviderReadDeleted

const ProviderReadFailure = fuzzing.ProviderReadFailure

const ProviderUpdateFailure = fuzzing.ProviderUpdateFailure

// Given a ProgramSpec and a set of ProviderSpecOptions, returns a rapid.Generator that will produce random
// ProviderSpecs that affect the resources defined in the ProgramSpec.
func GeneratedProviderSpec(progSpec *ProgramSpec, pso ProviderSpecOptions) *interface{} {
	return fuzzing.GeneratedProviderSpec(progSpec, pso)
}

