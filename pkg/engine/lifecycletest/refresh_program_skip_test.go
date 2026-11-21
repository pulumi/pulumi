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

package lifecycletest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for https://github.com/pulumi/pulumi/issues/25767: with `up --refresh --run-program`, a resource with
// existing state has its refresh run before its registration continues. If that refresh is skipped because a
// resource it depends on failed (or is awaiting), the registration must still complete, and whether the refresh
// is skipped has to agree with whether the step generated after it is skipped.

// watchdog panics if a deployment does not finish in time. It deliberately never touches a *testing.T: the
// lifecycletest framework calls require on the goroutine running the deployment, which must be the test's own.
// A hang panics the whole test binary, so on a regression rerun one subtest at a time with -run to see which hang.
func watchdog(d time.Duration) (stop func()) {
	timer := time.AfterFunc(d, func() { panic("deployment hung waiting on the refresh of a skipped resource") })
	return func() { timer.Stop() }
}

// refreshSkipCase describes a `up --refresh --run-program` over a snapshot where "failing" fails and "dependent"
// (optionally with a transitive dependent "third") is refreshed.
type refreshSkipCase struct {
	// updateErr is returned by the provider when "failing" is updated.
	updateErr       error
	continueOnError bool
	preview         bool
	// oldDepends and newDepends say whether "dependent" depends on "failing" in the old state and in the program.
	oldDepends bool
	newDepends bool
	// alias renames "dependent" from "dependentOld" in the program.
	alias bool
	// component makes "dependent" a component rather than a custom resource.
	component bool
}

// skipped is whether "dependent" must be skipped: its program registration depends on "failing".
func (c refreshSkipCase) skipped() bool { return c.newDepends }

func runRefreshSkipCase(t *testing.T, c refreshSkipCase) {
	t.Helper()

	var mu sync.Mutex
	var reads []resource.URN
	var failingResp, dependentResp, thirdResp *deploytest.RegisterResourceResponse
	var outputsErr error

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					mu.Lock()
					reads = append(reads, req.URN)
					mu.Unlock()
					inputs, outputs := req.Inputs, req.State
					if req.URN.Name() == "dependent" || req.URN.Name() == "dependentOld" {
						outputs = property.NewMap(map[string]property.Value{"in": property.New("drifted")})
					}
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{ID: req.ID, Inputs: &inputs, Outputs: &outputs},
						Status:     resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					if req.URN.Name() == "failing" || req.URN.Name() == "third" {
						return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
					}
					return plugin.DiffResponse{Changes: plugin.DiffNone}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.URN.Name() == "failing" {
						return plugin.UpdateResponse{Status: resource.StatusOK}, c.updateErr
					}
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	oldDependentName := "dependent"
	if c.alias {
		oldDependentName = "dependentOld"
	}
	failingURN := resource.NewURN("test", "test", "", "pkgA:m:typA", "failing")
	oldDependentURN := resource.NewURN("test", "test", "", "pkgA:m:typA", oldDependentName)
	dependentURN := resource.NewURN("test", "test", "", "pkgA:m:typA", "dependent")
	thirdURN := resource.NewURN("test", "test", "", "pkgA:m:typA", "third")

	v1 := resource.PropertyMap{"in": resource.NewProperty("v1")}
	var oldDeps []resource.URN
	if c.oldDepends {
		oldDeps = []resource.URN{failingURN}
	}
	oldDependent := &pkgresource.State{
		Type:         oldDependentURN.Type(),
		URN:          oldDependentURN,
		Custom:       !c.component,
		Inputs:       v1,
		Outputs:      v1,
		Dependencies: oldDeps,
	}
	if !c.component {
		oldDependent.ID = "dependent-id"
	}
	old := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: failingURN.Type(), URN: failingURN, Custom: true, ID: "failing-id", Inputs: v1, Outputs: v1},
			oldDependent,
			{
				Type: thirdURN.Type(), URN: thirdURN, Custom: true, ID: "third-id", Inputs: v1, Outputs: v1,
				Dependencies: []resource.URN{oldDependentURN},
			},
		},
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "failing", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  property.NewMap(map[string]property.Value{"in": property.New("v2")}),
		})
		if err != nil {
			return err
		}

		opts := deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  property.NewMap(map[string]property.Value{"in": property.New("v1")}),
		}
		if c.newDepends {
			opts.Dependencies = []resource.URN{resp.URN}
		}
		if c.alias {
			opts.AliasURNs = []resource.URN{oldDependentURN}
		}
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dependent", !c.component, opts)
		if err != nil {
			return err
		}
		if c.component {
			err = monitor.RegisterResourceOutputs(dep.URN, property.NewMap(map[string]property.Value{
				"in": property.New("registered"),
			}))
			mu.Lock()
			outputsErr = err
			mu.Unlock()
		}

		third, err := monitor.RegisterResource("pkgA:m:typA", "third", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  property.NewMap(map[string]property.Value{"in": property.New("v2")}),
			Dependencies:            []resource.URN{dep.URN},
		})
		mu.Lock()
		failingResp, dependentResp, thirdResp = resp, dep, third
		mu.Unlock()
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			SkipDisplayTests: true,
			HostF:            hostF,
			UpdateOptions: UpdateOptions{
				ContinueOnError: c.continueOnError,
				Refresh:         true,
				RefreshProgram:  true,
			},
		},
	}

	// Run on the test goroutine, the framework requires that. A hung deployment is caught by the watchdog.
	stop := watchdog(60 * time.Second)
	defer stop()
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, old), p.Options, c.preview, p.BackendClient, nil, "0")

	mu.Lock()
	defer mu.Unlock()

	require.Error(t, err)
	if _, ok := c.updateErr.(*plugin.AwaitError); ok {
		assert.ErrorContains(t, err, "resource is awaiting")
	} else {
		assert.ErrorContains(t, err, c.updateErr.Error())
	}
	if !c.preview {
		require.NotNil(t, snap)
	}
	require.NotNil(t, dependentResp, "dependent's RegisterResource never returned")
	require.NotNil(t, thirdResp, "third's RegisterResource never returned")

	assert.Equal(t, dependentURN, dependentResp.URN)
	assert.Equal(t, pulumirpc.Result_SUCCESS, dependentResp.Result)
	assert.Equal(t, c.skipped(), dependentResp.Unknown, "dependent skipped state")
	assert.Equal(t, pulumirpc.Result_SUCCESS, thirdResp.Result)
	assert.Equal(t, c.skipped(), thirdResp.Unknown, "third skipped state")

	// The provider reads the dependent exactly when its refresh was not skipped.
	if !c.component {
		var readDependent bool
		for _, urn := range reads {
			readDependent = readDependent || urn == oldDependentURN || urn == dependentURN
		}
		assert.Equal(t, !c.skipped(), readDependent, "dependent read")
	}

	find := func(urn resource.URN) *pkgresource.State {
		for _, r := range snap.Resources {
			if r.URN == urn {
				return r
			}
		}
		return nil
	}

	if c.preview {
		return
	}

	third := find(thirdURN)
	require.NotNil(t, third)
	if c.skipped() {
		// Nothing was journaled for the dependent, so its prior state is kept (under its old name), and so is
		// everything downstream of it.
		dep := find(oldDependentURN)
		require.NotNil(t, dep, "skipped dependent should keep its prior state")
		assert.Equal(t, oldDependent.ID, dep.ID)
		assert.Equal(t, v1, dep.Outputs)
		assert.Equal(t, oldDeps, dep.Dependencies)
		assert.Equal(t, v1, third.Inputs, "dependent of a skipped resource should be skipped")
		return
	}

	dep := find(dependentURN)
	require.NotNil(t, dep, "dependent should be processed normally")
	assert.Empty(t, dep.Dependencies)
	if c.component {
		require.NoError(t, outputsErr)
		assert.Equal(t, resource.PropertyMap{"in": resource.NewProperty("registered")}, dep.Outputs)
	} else {
		assert.Equal(t, resource.ID("dependent-id"), dep.ID)
		assert.Equal(t, resource.PropertyMap{"in": resource.NewProperty("drifted")}, dep.Outputs)
	}
	assert.Equal(t, resource.PropertyMap{"in": resource.NewProperty("v2")}, third.Inputs,
		"dependent of a resource that wasn't skipped should be processed normally")
	require.NotNil(t, failingResp)
	assert.Equal(t, pulumirpc.Result_FAIL, failingResp.Result)
}

func TestRefreshProgramSkip(t *testing.T) {
	t.Parallel()

	failed := errors.New("intentionally failed update")
	cases := map[string]refreshSkipCase{
		// The dependent still depends on the failed resource, so the refresh is skipped. This hung.
		"continue-on-error, dependency kept": {
			updateErr: failed, continueOnError: true, oldDepends: true, newDepends: true,
		},
		// The dependent no longer depends on the failed resource, so it is refreshed and processed normally.
		// The program no longer depends on "failing", so the refresh runs and the resource proceeds normally.
		"continue-on-error, dependency dropped": {
			updateErr: failed, continueOnError: true, oldDepends: true,
		},
		"await error, dependency kept": {
			updateErr: &plugin.AwaitError{}, oldDepends: true, newDepends: true,
		},
		"await error, dependency dropped": {
			updateErr: &plugin.AwaitError{}, oldDepends: true,
		},
		// The refresh is skipped as well as the step after it, because both are decided by the program.
		"continue-on-error, dependency added": {
			updateErr: failed, continueOnError: true, newDepends: true,
		},
		"component, dependency kept": {
			updateErr: failed, continueOnError: true, oldDepends: true, newDepends: true, component: true,
		},
		"component, dependency dropped": {
			updateErr: failed, continueOnError: true, oldDepends: true, component: true,
		},
		"aliased, dependency kept": {
			updateErr: failed, continueOnError: true, oldDepends: true, newDepends: true, alias: true,
		},
		"aliased, dependency dropped": {
			updateErr: failed, continueOnError: true, oldDepends: true, alias: true,
		},
		"preview, dependency kept": {
			updateErr: failed, continueOnError: true, preview: true, oldDepends: true, newDepends: true,
		},
		"preview, dependency dropped": {
			updateErr: failed, continueOnError: true, preview: true, oldDepends: true,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runRefreshSkipCase(t, c)
		})
	}
}

// A resource whose provider failed under --continue-on-error has its refresh skipped: the program says it uses
// the failed provider, so refreshing it would read with a provider that isn't there. Its registration must still
// complete, as skipped, and its prior state is kept.
func testRefreshProgramSkipFailedProvider(t *testing.T, explicit bool) {
	t.Parallel()

	var mu sync.Mutex
	var reads []resource.URN
	var dependentResp *deploytest.RegisterResourceResponse

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					if v, ok := req.Inputs.GetOk("foo"); ok && v.AsString() == "new" {
						return plugin.ConfigureResponse{}, errors.New("intentionally failed provider configure")
					}
					return plugin.ConfigureResponse{}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					mu.Lock()
					reads = append(reads, req.URN)
					mu.Unlock()
					inputs, outputs := req.Inputs, req.State
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{ID: req.ID, Inputs: &inputs, Outputs: &outputs},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	provName := "default"
	if explicit {
		provName = "prov"
	}
	provURN := resource.NewURN("test", "test", "", "pulumi:providers:pkgA", provName)
	provID := resource.ID("11111111-1111-1111-1111-111111111111")
	provRef := provURN + "::" + resource.URN(provID)
	dependentURN := resource.NewURN("test", "test", "", "pkgA:m:typA", "dependent")
	v1 := resource.PropertyMap{"in": resource.NewProperty("v1")}
	oldProvInputs := resource.PropertyMap{"foo": resource.NewProperty("old")}

	old := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type: provURN.Type(), URN: provURN, Custom: true, ID: provID,
				Inputs: oldProvInputs, Outputs: oldProvInputs,
			},
			{
				Type: dependentURN.Type(), URN: dependentURN, Custom: true, ID: "dependent-id",
				Inputs: v1, Outputs: v1, Provider: string(provRef),
			},
		},
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		opts := deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  property.NewMap(map[string]property.Value{"in": property.New("v1")}),
		}
		if explicit {
			_, err := monitor.RegisterResource(tokens.Type("pulumi:providers:pkgA"), "prov", true,
				deploytest.ResourceOptions{
					SupportsResultReporting: true,
					Inputs:                  property.NewMap(map[string]property.Value{"foo": property.New("new")}),
				})
			if err != nil {
				return err
			}
			opts.Provider = string(provRef)
		}
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dependent", true, opts)
		mu.Lock()
		dependentResp = dep
		mu.Unlock()
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			SkipDisplayTests: true,
			HostF:            hostF,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
				Refresh:         true,
				RefreshProgram:  true,
			},
		},
	}
	if !explicit {
		p.Config = config.Map{config.MustMakeKey("pkgA", "foo"): config.NewValue("new")}
	}

	stop := watchdog(60 * time.Second)
	defer stop()
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, old), p.Options, false, p.BackendClient, nil, "0")

	require.ErrorContains(t, err, "intentionally failed provider configure")
	require.NotNil(t, snap)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, dependentResp, "dependent's RegisterResource never returned")
	assert.Equal(t, pulumirpc.Result_SUCCESS, dependentResp.Result)
	assert.True(t, dependentResp.Unknown, "dependent of a failed provider should be skipped")
	assert.NotContains(t, reads, dependentURN)

	var dependent *pkgresource.State
	for _, r := range snap.Resources {
		if r.URN == dependentURN {
			dependent = r
		}
	}
	require.NotNil(t, dependent, "skipped dependent should keep its prior state")
	assert.Equal(t, resource.ID("dependent-id"), dependent.ID)
	assert.Equal(t, v1, dependent.Outputs)
}

func TestRefreshProgramSkipFailedExplicitProvider(t *testing.T) {
	testRefreshProgramSkipFailedProvider(t, true)
}

func TestRefreshProgramSkipFailedDefaultProvider(t *testing.T) {
	testRefreshProgramSkipFailedProvider(t, false)
}
