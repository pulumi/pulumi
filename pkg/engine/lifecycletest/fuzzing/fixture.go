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
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/snapshot"
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

func generateAndWriteRepro(
	t *rapid.T,
	stackSpec StackSpecOptions,
	snapSpec *SnapshotSpec,
	programSpec *ProgramSpec,
	providerSpec *ProviderSpec,
	planSpec *PlanSpec,
) string {
	reproTest := GenerateReproTest(t, stackSpec, snapSpec, programSpec, providerSpec, planSpec)

	var reproMessage string
	if os.Getenv("CI") != "" {
		reproMessage = "Full test case:\n\n" + reproTest + "\n\n"
	} else {
		reproFile, reproErr := writeReproTest(reproTest)
		if reproErr != nil {
			reproMessage = fmt.Sprintf("Error writing reproduction test case:\n\n%v", reproErr)
		} else {
			reproMessage = "Reproduction test case was written to " + reproFile
		}
	}

	return reproMessage
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

		if fo.SnapshotSpecOptions.ExclusionRules.ShouldExclude(snapSpec, progSpec, provSpec, planSpec) {
			// If the generated snapshot matches an exclusion rule, we skip this test case.
			t.Skip("snapshot matches exclusion rule")
			return
		}

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

		failWithError := func(err error) {
			// Try to generate code for a reproducing test. If we fail, we'll still report the snapshot integrity error and
			// just inform the user that we couldn't write a reproduction for some reason.
			reproMessage := generateAndWriteRepro(t, fo.StackSpecOptions, snapSpec, progSpec, provSpec, planSpec)

			// To aid in debugging further, we'll color any URNs in the snapshot integrity error message using a regular
			// expression. This is probably not perfect, but in practice it's really helpful.
			coloredErr := urnPattern.ReplaceAllStringFunc(err.Error(), Colored)

			assert.Failf(
				t,
				"Encountered a snapshot integrity error",
				`Error: %s
%s
%s
%s
%s

%s
	`,
				coloredErr,
				snapSpec.Pretty(""),
				progSpec.Pretty(""),
				provSpec.Pretty(""),
				planSpec.Pretty(""),
				reproMessage,
			)
		}
		done := make(chan struct{})
		// Cancellable context so the 20s cap below can signal the engine to shut down.
		// This is best-effort: a genuinely hung engine may not honor cancellation, in
		// which case the post-cancel drain below escalates to a hard failure.
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() {
			defer close(done)
			// Operations may fail for legitimate reasons -- e.g. we have configured a provider operation to fail, aborting
			// execution. We thus only fail if we encounter an actual snapshot integrity error.
			outSnap, err := op.RunWithContextStep(
				ctx, project, p.GetTarget(t, inSnap), opOpts, false, p.BackendClient, nil, "0")
			// If the outer select cancelled us after the 20s cap, skip all post-op checks — the
			// outer goroutine is responsible for reporting the timeout, and any error we observe
			// here is cancellation noise, not a bug.
			if ctx.Err() != nil {
				return
			}
			if _, isSIE := snapshot.AsSnapshotIntegrityError(err); isSIE {
				failWithError(err)
			}

			// If for some reason the operation does not return an error, but the resulting snapshot is invalid, we'll fail in
			// the same manner.
			outSnapErr := outSnap.VerifyIntegrity()
			if _, isSIE := snapshot.AsSnapshotIntegrityError(outSnapErr); isSIE {
				failWithError(outSnapErr)
			}

			if err != nil && strings.Contains(err.Error(), "actual and expected resources differ") {
				failWithError(err)
			}

			if err != nil && strings.Contains(err.Error(), "panic") {
				failWithError(err)
			}

			// In all other cases, we expect errors to be "expected", or "bails" in our terminology.
			assert.True(t, err == nil || result.IsBail(err), "unexpected error: %v", err)
		}()
		select {
		case <-done:
			// Test completed within the timeout.
		case <-time.After(20 * time.Second):
			cancel()
			// Give the engine a brief window to observe cancellation and drain. A
			// genuinely hung engine likely won't respond at all, so keep this short and
			// fail loudly below if it doesn't drain in time.
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				reproMessage := generateAndWriteRepro(t, fo.StackSpecOptions, snapSpec, progSpec, provSpec, planSpec)
				assert.Failf(
					t,
					"test timed out after 20 seconds and did not respond to cancellation within 5 more seconds",
					reproMessage,
				)
				return
			}
			reproMessage := generateAndWriteRepro(t, fo.StackSpecOptions, snapSpec, progSpec, provSpec, planSpec)
			assert.Failf(t, "test timed out after 20 seconds", reproMessage)
		}
	}
}

// A regular expression pattern for matching URNs in error messages without slurping characters that might appear after
// them such as commas, periods, or quotation marks.
var urnPattern = regexp.MustCompile(`urn:pulumi:[^:]+::[^:]+::[^\s,.'"]+`)

// writeReproTest writes the given string to a file in the directory specified by
// PULUMI_LIFECYCLE_TEST_FUZZING_REPRO_DIR (which will be created if it does not exist), or to a temporary directory if
// that environment variable is not set. Returns the path to the file written, or an error if one occurred at any point
// during directory creation or the write.
func writeReproTest(reproTest string) (string, error) {
	reproDir := os.Getenv("PULUMI_LIFECYCLE_TEST_FUZZING_REPRO_DIR")
	if reproDir != "" {
		mkdirErr := os.MkdirAll(reproDir, 0o700)
		if mkdirErr != nil {
			return "", mkdirErr
		}
	}

	reproFile, reproFileErr := os.CreateTemp(reproDir, "fuzzing_repro_test_*.go")
	if reproFileErr != nil {
		return "", reproFileErr
	}

	n, writeErr := reproFile.WriteString(reproTest)
	if writeErr != nil {
		return "", writeErr
	}

	if n != len(reproTest) {
		return "", fmt.Errorf("wrote %d bytes to %s, expected %d", n, reproFile.Name(), len(reproTest))
	}

	return reproFile.Name(), nil
}
