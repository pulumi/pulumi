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
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// remoteComponentWithAwaitingChild runs two real updates against a remote component whose only child awaits on
// the first update and succeeds on the second. The component exposes the child's output as its own output "out".
// It returns the "out" value the program observed on each update, and the whole registration response.
func remoteComponentWithAwaitingChild(
	t *testing.T, withoutGrpc bool,
) (first, second *deploytest.RegisterResourceResponse) {
	awaitChild := true

	var opts []deploytest.ProviderOption
	if withoutGrpc {
		opts = append(opts, deploytest.WithoutGrpc)
	} else {
		opts = append(opts, deploytest.WithGrpc)
	}
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context, req plugin.ConstructRequest, monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					comp, err := monitor.RegisterResource(req.Type, req.Name, false,
						deploytest.ResourceOptions{Parent: req.Parent})
					require.NoError(t, err)

					child, err := monitor.RegisterResource("pkgB:m:child", "child", true, deploytest.ResourceOptions{
						Parent:                  comp.URN,
						SupportsResultReporting: true,
					})
					require.NoError(t, err)

					// Behave like a component author: expose the child's output, unknown if the child is.
					out := resource.MakeComputed(resource.NewProperty(""))
					if !child.Unknown {
						out = child.Outputs["out"]
					}
					outs := resource.PropertyMap{"out": out}
					require.NoError(t, monitor.RegisterResourceOutputs(comp.URN, outs))
					return plugin.ConstructResponse{URN: comp.URN, Outputs: resource.FromResourcePropertyMap(outs)}, nil
				},
			}, nil
		}, opts...),
		// The awaiting child lives in a separate in-process provider: the test provider server does not
		// forward AwaitError over gRPC.
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if awaitChild {
						return plugin.CreateResponse{Status: resource.StatusOK}, &plugin.AwaitError{}
					}
					return plugin.CreateResponse{
						ID:         "id",
						Properties: resource.FromResourcePropertyMap(resource.PropertyMap{"out": resource.NewProperty("value")}),
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	var observed *deploytest.RegisterResourceResponse
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:comp", "comp", false, deploytest.ResourceOptions{
			Remote:                  true,
			SupportsResultReporting: true,
		})
		require.NoError(t, err)
		observed = resp
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	// The first update is a real (non-preview) update; the child awaits so the update reports an error.
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.Error(t, err)
	first = observed

	awaitChild = false
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	second = observed
	return first, second
}

// A remote component that exposes an awaiting child's output must report that output as unknown to the program
// in a real update, and as the child's value once the child succeeds.
//
// The provider is served over gRPC, so this exercises the engine's plugin client for Construct.
func TestRemoteComponentUnknownChildOutputRealUpdate(t *testing.T) {
	t.Parallel()

	first, second := remoteComponentWithAwaitingChild(t, false /* withoutGrpc */)
	require.NotNil(t, first)
	assert.True(t, first.Outputs["out"].IsComputed(),
		"component output derived from an awaiting child should be unknown, got %v", first.Outputs["out"])

	require.NotNil(t, second)
	assert.Equal(t, resource.NewProperty("value"), second.Outputs["out"])
}

// Same scenario with the provider called in-process (no gRPC marshaling in the engine's Construct client), to
// isolate what the engine's RegisterResource handler itself returns for a remote component.
func TestRemoteComponentUnknownChildOutputRealUpdateInProcess(t *testing.T) {
	t.Parallel()

	first, second := remoteComponentWithAwaitingChild(t, true /* withoutGrpc */)
	require.NotNil(t, first)
	assert.True(t, first.Outputs["out"].IsComputed(),
		"component output derived from an awaiting child should be unknown, got %v", first.Outputs["out"])
	// Language SDKs only treat computed values as unknown in a real update when the response says so for custom
	// resources (resp.Unknown); for components the flag is never set, so assert it is not what carries the signal.
	assert.False(t, first.Unknown, "RegisterResource for a remote component never sets Unknown")

	require.NotNil(t, second)
	assert.Equal(t, resource.NewProperty("value"), second.Outputs["out"])
}
