// Copyright 2026, Pulumi Corporation.
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

package ints

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// testproviderAbsPath returns the absolute path to the local testprovider directory next to this
// test file. The Pulumi.yaml plugins.providers entry needs an absolute path because the project
// lives in a temp directory created by ptesting.Environment.
func testproviderAbsPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not determine path of preview_changes_only_test.go")
	abs, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", "testprovider"))
	require.NoError(t, err)
	return abs
}

// previewChangesOnlyYAML returns a YAML pulumi program that uses three Updatable resources from the
// testprovider. The `phase` arg controls the second deployment: keep stays the same, update has its
// `foo` property modified, remove disappears, and added is introduced.
func previewChangesOnlyYAML(testproviderPath, phase string) string {
	header := `name: preview-changes-only-test
runtime: yaml
description: Integration test for ` + "`pulumi preview --json --changes-only`" + `
plugins:
  providers:
    - name: testprovider
      path: ` + testproviderPath + `
resources:
  keep:
    type: testprovider:index:Updatable
    properties:
      foo: initial
      bar: stable
`
	switch phase {
	case "before":
		return header + `  update:
    type: testprovider:index:Updatable
    properties:
      foo: initial
      bar: stable
  remove:
    type: testprovider:index:Updatable
    properties:
      foo: initial
      bar: stable
`
	case "after":
		return header + `  update:
    type: testprovider:index:Updatable
    properties:
      foo: changed
      bar: stable
  added:
    type: testprovider:index:Updatable
    properties:
      foo: brand-new
      bar: stable
`
	default:
		panic("unknown phase: " + phase)
	}
}

// setupPreviewChangesOnlyStack bootstraps a yaml pulumi project that uses the local testprovider's
// Updatable resource, deploys it, then rewrites the program to produce an update/create/delete mix.
func setupPreviewChangesOnlyStack(t *testing.T, stack string) *ptesting.Environment {
	t.Helper()

	e := ptesting.NewEnvironment(t)
	// The file backend lives entirely inside the temp dir, so dropping the env is enough; we
	// don't run `pulumi destroy` here because it can race with the test's cancelled context.
	t.Cleanup(e.DeleteIfNotFailed)

	providerPath := testproviderAbsPath(t)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.WriteTestFile("Pulumi.yaml", previewChangesOnlyYAML(providerPath, "before"))
	e.RunCommand("pulumi", "stack", "init", stack)
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Swap the program for one that triggers an update, a create and a delete.
	e.WriteTestFile("Pulumi.yaml", previewChangesOnlyYAML(providerPath, "after"))

	return e
}

// findStep returns the first step whose URN ends with the given resource name. Returns nil if
// no such step exists (which most tests treat as a hard failure).
func findStep(digest *display.PreviewDigest, name string) *display.PreviewStep {
	for _, step := range digest.Steps {
		if strings.HasSuffix(string(step.URN), "::"+name) {
			return step
		}
	}
	return nil
}

// TestPreviewChangesOnlyRequiresJSON verifies that --changes-only is rejected without --json
// at flag-parse time, so it fails fast and doesn't require a project on disk.
func TestPreviewChangesOnlyRequiresJSON(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	_, stderr := e.RunCommandExpectError("pulumi", "preview", "--changes-only")
	assert.Equal(t,
		"error: --changes-only can only be used with --json",
		strings.Trim(stderr, "\r\n"))
}

// TestPreviewChangesOnlyFiltersDigest runs a full preview through the CLI with --json
// --changes-only and verifies that (a) only changed resources appear, (b) for the updated
// resource only the changed top-level inputs are serialised, and (c) outputs are dropped.
func TestPreviewChangesOnlyFiltersDigest(t *testing.T) {
	t.Parallel()

	e := setupPreviewChangesOnlyStack(t, "changes-only-filter")

	stdout, _ := e.RunCommand("pulumi", "preview", "--json", "--changes-only")

	var digest display.PreviewDigest
	require.NoError(t, json.Unmarshal([]byte(stdout), &digest),
		"preview --json --changes-only must emit a valid PreviewDigest; got: %s", stdout)

	// --- Step filtering ---------------------------------------------------------------------
	// "keep" is unchanged so it must not appear at all. The other three resources (update,
	// remove, added) should be the only Updatable steps.
	for _, step := range digest.Steps {
		// The implicit root stack same-op step is allowed through shouldShow via the
		// isRootStack() override; skip it here so we can make assertions on the resources.
		if strings.Contains(string(step.URN), "::pulumi:pulumi:Stack::") {
			continue
		}
		// No OpSame should ever be emitted under --changes-only for real resources.
		assert.NotEqual(t, deploy.OpSame, step.Op,
			"unchanged resources must be filtered out: %s", step.URN)
		// Reads/refreshes are also suppressed.
		assert.NotEqual(t, deploy.OpRead, step.Op, "reads must be filtered out")
		assert.NotEqual(t, deploy.OpRefresh, step.Op, "refreshes must be filtered out")
	}

	assert.Nil(t, findStep(&digest, "keep"),
		"unchanged resource 'keep' must not appear in --changes-only output")

	// --- Update step: only changed inputs, no outputs ---------------------------------------
	updateStep := findStep(&digest, "update")
	require.NotNil(t, updateStep, "expected an update step for resource 'update'")
	assert.Equal(t, deploy.OpUpdate, updateStep.Op)
	// The provider reported `foo` as the changed key; `bar` should be filtered out of NewState
	// and the full Outputs map should be empty.
	require.NotNil(t, updateStep.NewState, "update step must include NewState")
	newInputs := updateStep.NewState.Inputs
	assert.Contains(t, newInputs, "foo", "changed key 'foo' must remain in NewState.Inputs")
	assert.NotContains(t, newInputs, "bar",
		"unchanged key 'bar' must be stripped from NewState.Inputs under --changes-only")
	assert.Empty(t, updateStep.NewState.Outputs,
		"outputs must be dropped from NewState under --changes-only")
	// DiffReasons still carry the engine's change list, regardless of the filter.
	assert.Contains(t, updateStep.DiffReasons, resource.PropertyKey("foo"))

	// --- Delete step: OldState is still present (so consumers know what's gone) -------------
	removeStep := findStep(&digest, "remove")
	require.NotNil(t, removeStep, "expected a delete step for resource 'remove'")
	assert.Equal(t, deploy.OpDelete, removeStep.Op)
	require.NotNil(t, removeStep.OldState, "delete step must include OldState")
	// For deletes all old inputs are preserved (there's no "changed subset" for a delete).
	assert.Contains(t, removeStep.OldState.Inputs, "foo")
	assert.Contains(t, removeStep.OldState.Inputs, "bar")
	assert.Empty(t, removeStep.OldState.Outputs, "delete step Outputs must also be stripped")

	// --- Create step: NewState has full inputs, no outputs ----------------------------------
	addedStep := findStep(&digest, "added")
	require.NotNil(t, addedStep, "expected a create step for resource 'added'")
	assert.Equal(t, deploy.OpCreate, addedStep.Op)
	require.NotNil(t, addedStep.NewState, "create step must include NewState")
	assert.Contains(t, addedStep.NewState.Inputs, "foo")
	assert.Contains(t, addedStep.NewState.Inputs, "bar")
	assert.Empty(t, addedStep.NewState.Outputs, "create step Outputs must be stripped")

	// --- ChangeSummary is untouched: still reports the full counts --------------------------
	assert.Equal(t, 1, digest.ChangeSummary[deploy.OpCreate])
	assert.Equal(t, 1, digest.ChangeSummary[deploy.OpUpdate])
	assert.Equal(t, 1, digest.ChangeSummary[deploy.OpDelete])
}

// TestPreviewChangesOnlyVsPlain compares --json and --json --changes-only on the exact same
// stack state. This nails down that --changes-only is a filter over the existing --json output:
// for an update step, full inputs are present in the plain digest but absent in the filtered one.
func TestPreviewChangesOnlyVsPlain(t *testing.T) {
	t.Parallel()

	e := setupPreviewChangesOnlyStack(t, "changes-only-compare")

	plainStdout, _ := e.RunCommand("pulumi", "preview", "--json")
	var plain display.PreviewDigest
	require.NoError(t, json.Unmarshal([]byte(plainStdout), &plain))

	filteredStdout, _ := e.RunCommand("pulumi", "preview", "--json", "--changes-only")
	var filtered display.PreviewDigest
	require.NoError(t, json.Unmarshal([]byte(filteredStdout), &filtered))

	// Both digests agree on the aggregate change summary.
	assert.Equal(t, plain.ChangeSummary, filtered.ChangeSummary)

	// The plain output includes the full input map for the updated resource...
	plainUpdate := findStep(&plain, "update")
	require.NotNil(t, plainUpdate)
	require.NotNil(t, plainUpdate.NewState)
	assert.Contains(t, plainUpdate.NewState.Inputs, "bar",
		"plain --json output should retain all inputs, including unchanged ones")

	// ...while the filtered output strips unchanged inputs.
	filteredUpdate := findStep(&filtered, "update")
	require.NotNil(t, filteredUpdate)
	require.NotNil(t, filteredUpdate.NewState)
	assert.NotContains(t, filteredUpdate.NewState.Inputs, "bar")
}
