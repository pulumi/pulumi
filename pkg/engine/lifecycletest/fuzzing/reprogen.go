package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// GenerateReproTest generates a string containing Go code for a set of lifecycle tests that reproduce the scenario
// captured by the given *Specs.
func GenerateReproTest(t lt.TB, sso StackSpecOptions, snapSpec *SnapshotSpec, progSpec *ProgramSpec, provSpec *ProviderSpec, planSpec *PlanSpec) string {
	return fuzzing.GenerateReproTest(t, sso, snapSpec, progSpec, provSpec, planSpec)
}

