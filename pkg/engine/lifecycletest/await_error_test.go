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
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAwaitErrorCreate verifies that when a provider returns an AwaitError from Create:
//   - no resource state is written to the snapshot,
//   - the SDK receives Result_FAIL for the awaiting resource,
//   - the deployment does not halt: independent resources registered after the awaited one still
//     complete successfully,
//   - the deployment reports an overall error so the CLI can surface the dedicated exit code.
func TestAwaitErrorCreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Name() == "awaiting" {
						return plugin.CreateResponse{Status: resource.StatusOK}, &plugin.AwaitError{}
					}
					return plugin.CreateResponse{ID: "id-" + resource.ID(req.URN.Name()), Status: resource.StatusOK}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		awaitResp, err := monitor.RegisterResource(
			"pkgA:m:typA", "awaiting", true, deploytest.ResourceOptions{SupportsResultReporting: true})
		require.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_FAIL, awaitResp.Result)

		nextResp, err := monitor.RegisterResource(
			"pkgA:m:typA", "after", true, deploytest.ResourceOptions{SupportsResultReporting: true})
		require.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, nextResp.Result)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.Error(t, err, "await failure should surface as an overall deployment error")
	require.NotNil(t, snap)

	var haveAwaiting, haveAfter bool
	for _, r := range snap.Resources {
		if r.URN.Name() == "awaiting" {
			haveAwaiting = true
		}
		if r.URN.Name() == "after" {
			haveAfter = true
		}
	}
	assert.False(t, haveAwaiting, "no state should be written for a resource that awaited on Create")
	assert.True(t, haveAfter, "independent resources registered after an await should still be created")
}

// TestAwaitErrorUpdate verifies that when a provider returns an AwaitError from Update:
//   - the prior resource state is preserved unchanged in the snapshot,
//   - the SDK receives Result_FAIL,
//   - the deployment does not halt, but reports an overall error.
func TestAwaitErrorUpdate(t *testing.T) {
	t.Parallel()

	priorOutputs := resource.PropertyMap{
		"kept": resource.NewProperty("prior-value"),
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				UpdateF: func(_ context.Context, _ plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{Status: resource.StatusOK}, &plugin.AwaitError{}
				},
				DiffF: func(_ context.Context, _ plugin.DiffRequest) (plugin.DiffResponse, error) {
					return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	resURN := resource.NewURN("test", "test", "", "pkgA:m:typA", "awaiting")
	old := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "existing-id",
				Inputs:  resource.PropertyMap{"in": resource.NewProperty("v1")},
				Outputs: priorOutputs,
			},
		},
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "awaiting", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  resource.PropertyMap{"in": resource.NewProperty("v2")},
		})
		require.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_FAIL, resp.Result)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, old), p.Options, false, p.BackendClient, nil, "0")
	require.Error(t, err, "await failure should surface as an overall deployment error")
	require.NotNil(t, snap)

	var found *pkgresource.State
	for _, r := range snap.Resources {
		if r.URN == resURN {
			found = r
			break
		}
	}
	require.NotNil(t, found, "prior resource should still be present in state")
	assert.Equal(t, resource.ID("existing-id"), found.ID)
	assert.Equal(t, priorOutputs, found.Outputs, "outputs must be preserved unchanged on Update await")
	assert.Equal(t, resource.PropertyMap{"in": resource.NewProperty("v1")}, found.Inputs,
		"inputs must be preserved unchanged on Update await")
}
