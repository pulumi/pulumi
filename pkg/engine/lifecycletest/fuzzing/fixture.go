// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fuzzing

import (
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// A set of options for configuring the generation of a fuzzing lifecycle test fixture. A fixture comprises a stack, an
// initial snapshot, a program to execute against that snapshot, a set of providers to use when executing the program,
// and a plan to execute and observe the results of.
type FixtureOptions struct {
	StackSpecOptions    StackSpecOptions
	SnapshotSpecOptions SnapshotSpecOptions
	ProgramSpecOptions  ProgramSpecOptions
	ProviderSpecOptions ProviderSpecOptions
	PlanSpecOptions     PlanSpecOptions
}

// Returns a copy of the FixtureOptions with the given overrides applied.
func (fo FixtureOptions) With(overrides FixtureOptions) FixtureOptions {
	fo.StackSpecOptions = fo.StackSpecOptions.With(overrides.StackSpecOptions)
	fo.SnapshotSpecOptions = fo.SnapshotSpecOptions.With(overrides.SnapshotSpecOptions)
	fo.ProgramSpecOptions = fo.ProgramSpecOptions.With(overrides.ProgramSpecOptions)
	fo.ProviderSpecOptions = fo.ProviderSpecOptions.With(overrides.ProviderSpecOptions)
	fo.PlanSpecOptions = fo.PlanSpecOptions.With(overrides.PlanSpecOptions)

	return fo
}

// A default set of FixtureOptions.
var defaultFixtureOptions = FixtureOptions{
	StackSpecOptions:    defaultStackSpecOptions,
	SnapshotSpecOptions: defaultSnapshotSpecOptions,
	ProgramSpecOptions:  defaultProgramSpecOptions,
	ProviderSpecOptions: defaultProviderSpecOptions,
	PlanSpecOptions:     defaultPlanSpecOptions,
}

// Given a set of options, returns a Rapid property test function that generates and tests fixtures according to that
// configuration.
func GeneratedFixture(fo FixtureOptions) func(t *rapid.T) {
	fo = defaultFixtureOptions.With(fo)

	return func(t *rapid.T) {
		snapSpec := GeneratedSnapshotSpec(fo.StackSpecOptions, fo.SnapshotSpecOptions).Draw(t, "SnapshotSpec")
		progSpec := GeneratedProgramSpec(snapSpec, fo.StackSpecOptions, fo.ProgramSpecOptions).Draw(t, "ProgramSpec")
		provSpec := GeneratedProviderSpec(progSpec, fo.ProviderSpecOptions).Draw(t, "ProviderSpec")
		planSpec := GeneratedPlanSpec(snapSpec, fo.PlanSpecOptions).Draw(t, "PlanSpec")

		inSnap := snapSpec.AsSnapshot()
		require.NoError(t, inSnap.VerifyIntegrity(), "initial snapshot is not valid")

		hostF := deploytest.NewPluginHostF(nil, nil, progSpec.AsLanguageRuntimeF(t), provSpec.AsProviderLoaders()...)

		opOpts, op := planSpec.Executors(t, hostF)
		opOpts.SkipDisplayTests = true

		p := &lt.TestPlan{
			Project: fo.StackSpecOptions.Project,
			Stack:   fo.StackSpecOptions.Stack,
		}
		project := p.GetProject()

		failWithSIE := func(err error) {
			assert.Failf(
				t,
				"Encountered a snapshot integrity error",
				`Error: %v
%s
%s
%s
%s
`,
				err,
				snapSpec.Pretty(""),
				progSpec.Pretty(""),
				provSpec.Pretty(""),
				planSpec.Pretty(""),
			)
		}

		// Operations may fail for legitimate reasons -- e.g. we have configured a provider operation to fail, aborting
		// execution. We thus only fail if we encounter an actual snapshot integrity error.
		outSnap, err := op.RunStep(project, p.GetTarget(t, inSnap), opOpts, false, p.BackendClient, nil, "0")
		if _, isSIE := deploy.AsSnapshotIntegrityError(err); isSIE {
			failWithSIE(err)
		}

		// If for some reason the operation does not return an error, but the resulting snapshot is invalid, we'll fail in
		// the same manner.
		outSnapErr := outSnap.VerifyIntegrity()
		if _, isSIE := deploy.AsSnapshotIntegrityError(outSnapErr); isSIE {
			failWithSIE(outSnapErr)
		}

		// In all other cases, we expect errors to be "expected", or "bails" in our terminology.
		assert.True(t, err == nil || result.IsBail(err), "unexpected error: %v", err)
	}
}
