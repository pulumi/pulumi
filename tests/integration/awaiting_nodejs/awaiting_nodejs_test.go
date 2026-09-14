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

package awaiting_nodejs

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestAwaitingLifecycleNodejs(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "provider.trace")
	exits := filepath.Join(t.TempDir(), "up.exits")
	wrapper := newCLIWrapper(t, exits)

	validate := func(wantCreates, wantChecks, wantDeletes, wantActive, wantDeferred int, wantExits string) func(*testing.T, integration.RuntimeValidationStackInfo) {
		return func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			contents, err := os.ReadFile(trace)
			require.NoError(t, err)
			lines := string(contents)
			require.Equal(t, wantCreates, strings.Count(lines, "create "))
			require.Equal(t, wantChecks, strings.Count(lines, "dependent-check"))
			require.Equal(t, wantDeletes, strings.Count(lines, "delete "))
			require.Len(t, stack.Deployment.Resources, wantActive)
			require.Len(t, stack.Deployment.DeferredResources, wantDeferred)
			exitCodes, err := os.ReadFile(exits)
			require.NoError(t, err)
			require.Equal(t, wantExits, string(exitCodes))
		}
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("step1"),
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Env:                    []string{"PULUMI_TEST_AWAITING_TRACE=" + trace},
		Bin:                    wrapper,
		Quick:                  true,
		ExpectFailure:          true,
		ExtraRuntimeValidation: validate(1, 0, 0, 2, 2, "10\n"),
		EditDirs: []integration.EditDir{
			{Dir: filepath.Join("step2"), Additive: true, ExtraRuntimeValidation: validate(2, 1, 0, 4, 0, "10\n0\n")},
			{Dir: filepath.Join("step3"), Additive: true, ExpectFailure: true, ExtraRuntimeValidation: validate(3, 1, 0, 4, 2, "10\n0\n10\n")},
			{Dir: filepath.Join("step4"), Additive: true, ExtraRuntimeValidation: validate(4, 2, 1, 4, 0, "10\n0\n10\n0\n")},
		},
	})
}

func TestAwaitingFreshGraphNodejs(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "provider.trace")
	exits := filepath.Join(t.TempDir(), "up.exits")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "fresh",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Env:           []string{"PULUMI_TEST_AWAITING_TRACE=" + trace},
		Bin:           newCLIWrapper(t, exits),
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			contents, err := os.ReadFile(trace)
			require.NoError(t, err)
			require.Equal(t, 1, strings.Count(string(contents), "create "))
			require.NotContains(t, string(contents), "dependent-check")
			require.NotContains(t, string(contents), "delete ")
			require.Len(t, stack.Deployment.Resources, 2)
			require.Len(t, stack.Deployment.DeferredResources, 4)
			exitCodes, err := os.ReadFile(exits)
			require.NoError(t, err)
			require.Equal(t, "10\n", string(exitCodes))
		},
		EditDirs: []integration.EditDir{{
			Dir:      "fresh-ready",
			Additive: true,
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				contents, err := os.ReadFile(trace)
				require.NoError(t, err)
				require.Equal(t, 3, strings.Count(string(contents), "create "))
				require.Equal(t, 2, strings.Count(string(contents), "dependent-check"))
				require.NotContains(t, string(contents), "delete ")
				require.Len(t, stack.Deployment.Resources, 6)
				require.Empty(t, stack.Deployment.DeferredResources)
				exitCodes, err := os.ReadFile(exits)
				require.NoError(t, err)
				require.Equal(t, "10\n0\n", string(exitCodes))
			},
		}},
	})
}

func TestAwaitingTargetDependentsNodejs(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "provider.trace")
	exits := filepath.Join(t.TempDir(), "up.exits")
	wrapper := newCLIWrapper(t, exits)

	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:          "target",
		StackName:    "dev",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Env:        []string{"PULUMI_TEST_AWAITING_TRACE=" + trace},
		Bin:        wrapper,
		Quick:      true,
		NoParallel: true,
	})
	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})
	require.NoError(t, pt.TestLifeCyclePrepare())
	require.NoError(t, pt.TestLifeCycleInitialize())
	require.NoError(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))
	assertTargetState(t, pt, trace, exits, 2, 2, 0, 6, 0, "0\n", "v1")

	replaceProgram(t, pt.GetTmpDir(), "awaiting.ts")
	testURN := "urn:pulumi:dev::awaiting-nodejs-target::testprovider:index:Awaiting::test"
	productionURN := "urn:pulumi:dev::awaiting-nodejs-target::testprovider:index:Awaiting::production"
	err := pt.RunPulumiCommand("up", "--yes", "--skip-preview", "--replace", testURN,
		"--replace", productionURN, "--target-dependents")
	require.Error(t, err)
	assertTargetState(t, pt, trace, exits, 3, 2, 0, 6, 4, "0\n10\n", "v1")

	replaceProgram(t, pt.GetTmpDir(), "ready.ts")
	require.NoError(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview", "--replace", testURN,
		"--replace", productionURN, "--target-dependents"))
	assertTargetState(t, pt, trace, exits, 5, 4, 2, 6, 0, "0\n10\n0\n", "v2")
}

func TestDeliveryControllerPassRefreshesUnchangedOutputsNodejs(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "provider.trace")
	ready := filepath.Join(t.TempDir(), "controller.ready")
	exits := filepath.Join(t.TempDir(), "up.exits")
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:          "controller",
		StackName:    "dev",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Env: []string{
			"PULUMI_TEST_AWAITING_TRACE=" + trace,
			"PULUMI_TEST_CONTROLLER_READY=" + ready,
		},
		Bin:        newCLIWrapper(t, exits),
		Quick:      true,
		NoParallel: true,
	})
	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})
	require.NoError(t, pt.TestLifeCyclePrepare())
	require.NoError(t, pt.TestLifeCycleInitialize())
	require.NoError(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))
	assertControllerState(t, pt, trace, exits, 2, 0, 0, 4, "0\n", false)

	require.NoError(t, os.WriteFile(ready, []byte("2\n"), 0o600))
	replaceProgram(t, pt.GetTmpDir(), "ready.ts")
	require.Error(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))
	assertControllerState(t, pt, trace, exits, 4, 1, 0, 5, "0\n10\n", true)

	require.NoError(t, os.WriteFile(ready, []byte("3\n"), 0o600))
	replaceProgram(t, pt.GetTmpDir(), "resolved.ts")
	require.NoError(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))
	assertControllerState(t, pt, trace, exits, 6, 3, 1, 6, "0\n10\n0\n", true, 3)

	require.NoError(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))
	assertControllerState(t, pt, trace, exits, 6, 5, 1, 6, "0\n10\n0\n0\n", true, 3)
}

func TestRegistrationPassPersistsMigratedDependentInputsNodejs(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "selector.ready")
	exits := filepath.Join(t.TempDir(), "up.exits")
	require.NoError(t, os.WriteFile(ready, []byte("1\n"), 0o600))
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:          "selector",
		StackName:    "dev",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Env: []string{
			"PULUMI_TEST_CONTROLLER_READY=" + ready,
			"PULUMI_TEST_SELECTOR_MIGRATION=1",
		},
		Bin:        newCLIWrapper(t, exits),
		Quick:      true,
		NoParallel: true,
	})
	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})
	require.NoError(t, pt.TestLifeCyclePrepare())
	require.NoError(t, pt.TestLifeCycleInitialize())
	require.NoError(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))

	require.NoError(t, os.WriteFile(ready, []byte("2\n"), 0o600))
	replaceProgram(t, pt.GetTmpDir(), "ready.ts")
	require.Error(t, pt.RunPulumiCommand("up", "--yes", "--skip-preview"))

	export := filepath.Join(t.TempDir(), "stack.json")
	require.NoError(t, pt.RunPulumiCommand("stack", "export", "--file", export))
	raw, err := os.ReadFile(export)
	require.NoError(t, err)
	var untyped apitype.UntypedDeployment
	require.NoError(t, json.Unmarshal(raw, &untyped))
	var deployment apitype.DeploymentV3
	require.NoError(t, json.Unmarshal(untyped.Deployment, &deployment))
	activeSource := ""
	for _, state := range deployment.Resources {
		if strings.HasSuffix(string(state.URN), "::member") {
			activeSource, _ = state.Inputs["source"].(string)
		}
	}
	require.Equal(t, "commit-sha", activeSource)
	deferredSource := ""
	deferredConsumer := false
	for _, state := range deployment.DeferredResources {
		if strings.HasSuffix(string(state.URN), "::member") {
			deferredSource, _ = state.Inputs["source"].(string)
		}
		if strings.HasSuffix(string(state.URN), "::consumer") {
			deferredConsumer = true
		}
	}
	require.Equal(t, "part:/source", deferredSource,
		"the declared member must await with its migrated input while keeping runtime outputs unknown")
	require.True(t, deferredConsumer, "the member's unknown output must defer downstream resources")
	exitCodes, err := os.ReadFile(exits)
	require.NoError(t, err)
	require.Equal(t, "0\n10\n", string(exitCodes))
}

func newCLIWrapper(t *testing.T, exits string) string {
	t.Helper()
	realCLI, err := exec.LookPath("pulumi")
	require.NoError(t, err)
	wrapper := filepath.Join(t.TempDir(), "pulumi")
	require.NoError(t, os.WriteFile(wrapper, []byte("#!/bin/sh\n"+
		strconv.Quote(realCLI)+" \"$@\"\nstatus=$?\n"+
		"if [ \"$1\" = up ]; then echo \"$status\" >> "+strconv.Quote(exits)+"; fi\nexit \"$status\"\n"), 0o700))
	return wrapper
}

func replaceProgram(t *testing.T, dir, source string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(dir, source))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.ts"), contents, 0o600))
}

func assertTargetState(t *testing.T, pt *integration.ProgramTester, trace, exits string,
	wantCreates, wantChecks, wantDeletes, wantActive, wantDeferred int, wantExits, gateID string,
) {
	t.Helper()
	contents, err := os.ReadFile(trace)
	require.NoError(t, err)
	require.Equal(t, wantCreates, strings.Count(string(contents), "create "))
	require.Equal(t, wantChecks, strings.Count(string(contents), "dependent-check"))
	require.Equal(t, wantDeletes, strings.Count(string(contents), "delete "))
	exitCodes, err := os.ReadFile(exits)
	require.NoError(t, err)
	require.Equal(t, wantExits, string(exitCodes))

	export := filepath.Join(t.TempDir(), "stack.json")
	require.NoError(t, pt.RunPulumiCommand("stack", "export", "--file", export))
	raw, err := os.ReadFile(export)
	require.NoError(t, err)
	var untyped apitype.UntypedDeployment
	require.NoError(t, json.Unmarshal(raw, &untyped))
	var deployment apitype.DeploymentV3
	require.NoError(t, json.Unmarshal(untyped.Deployment, &deployment))
	require.Len(t, deployment.Resources, wantActive)
	require.Len(t, deployment.DeferredResources, wantDeferred)
	for _, resource := range deployment.Resources {
		if strings.HasSuffix(string(resource.URN), "::test") {
			require.Equal(t, gateID, resource.ID.String())
			return
		}
	}
	require.Fail(t, "test gate missing from active resources")
}

func assertControllerState(t *testing.T, pt *integration.ProgramTester, trace, exits string,
	wantCreates, wantChecks, wantUpdates, wantActive int, wantExits string, wantResolved bool,
	wantGeneration ...int,
) {
	t.Helper()
	contents, err := os.ReadFile(trace)
	require.NoError(t, err)
	require.Equal(t, wantCreates, strings.Count(string(contents), "create "))
	require.Equal(t, wantChecks, strings.Count(string(contents), "dependent-check"))
	require.Equal(t, wantUpdates, strings.Count(string(contents), "dependent-update"))
	require.NotContains(t, string(contents), "delete ")
	exitCodes, err := os.ReadFile(exits)
	require.NoError(t, err)
	require.Equal(t, wantExits, string(exitCodes))

	export := filepath.Join(t.TempDir(), "stack.json")
	require.NoError(t, pt.RunPulumiCommand("stack", "export", "--file", export))
	raw, err := os.ReadFile(export)
	require.NoError(t, err)
	var untyped apitype.UntypedDeployment
	require.NoError(t, json.Unmarshal(raw, &untyped))
	var deployment apitype.DeploymentV3
	require.NoError(t, json.Unmarshal(untyped.Deployment, &deployment))
	require.Len(t, deployment.Resources, wantActive)
	for _, state := range deployment.Resources {
		if !strings.HasSuffix(string(state.URN), "::stage") {
			continue
		}
		release, ok := state.Outputs["release"].(map[string]any)
		require.True(t, ok)
		parts, ok := release["parts"].(map[string]any)
		require.True(t, ok)
		_, resolved := parts["source"]
		require.Equal(t, wantResolved, resolved)
		if wantResolved {
			require.Equal(t, "release", release["id"])
			generation := 2
			if len(wantGeneration) > 0 {
				generation = wantGeneration[0]
			}
			require.Equal(t, float64(generation), release["generation"])
		}
		return
	}
	require.Fail(t, "controller stage missing from active resources")
}
