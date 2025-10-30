package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// A set of options for configuring the generation of a fuzzing lifecycle test fixture. A fixture comprises a stack, an
// initial snapshot, a program to execute against that snapshot, a set of providers to use when executing the program,
// and a plan to execute and observe the results of.
type FixtureOptions = fuzzing.FixtureOptions

// Given a set of options, returns a Rapid property test function that generates and tests fixtures according to that
// configuration.
func GeneratedFixture(fo FixtureOptions) func(*rapid.T) {
	return fuzzing.GeneratedFixture(fo)
}

