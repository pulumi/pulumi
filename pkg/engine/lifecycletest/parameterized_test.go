// Copyright 2024-2024, Pulumi Corporation.
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestPackageRef tests we can request a package ref from the engine and then use that, instead of Version,
// PackageDownloadURL etc.
func TestPackageRef(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "0",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "1",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkg1Ref, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil)
		require.NoError(t, err)
		pkg2Ref, err := monitor.RegisterPackage("pkgA", "2.0.0", "", nil, nil)
		require.NoError(t, err)

		// If we register the "same" provider in parallel, we should get the same ref.
		promises := []*promise.Promise[string]{}
		for i := 0; i < 100; i++ {
			var pcs promise.CompletionSource[string]
			promises = append(promises, pcs.Promise())
			go func() {
				ref, err := monitor.RegisterPackage("pkgB", "1.0.0", "downloadUrl", nil, nil)
				require.NoError(t, err)
				pcs.MustFulfill(ref)
			}()
		}
		ctx := context.Background()
		expected, err := promises[0].Result(ctx)
		require.NoError(t, err)
		for i := 1; i < 100; i++ {
			got, err := promises[i].Result(ctx)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
		}

		// Now register some resources using the UUID for the provider, instead of a normal provider ref.
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: pkg1Ref,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("0"), resp.ID)

		resp, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: pkg2Ref,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("1"), resp.ID)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)

	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, string(snap.Resources[0].URN)+"::"+string(snap.Resources[0].ID), snap.Resources[1].Provider)
	assert.Equal(t, string(snap.Resources[2].URN)+"::"+string(snap.Resources[2].ID), snap.Resources[3].Provider)
}

// TestReplacementParameterizedProvider tests that we can register a parameterized provider that replaces a base
// provider.
func TestReplacementParameterizedProvider(t *testing.T) {
	t.Parallel()

	loadCount := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			loadCount++

			var param string

			return &deploytest.Provider{
				ParameterizeF: func(
					ctx context.Context, req plugin.ParameterizeRequest,
				) (plugin.ParameterizeResponse, error) {
					value := req.Parameters.(*plugin.ParameterizeValue)

					param = string(value.Value)

					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Type() == "pkgExt:m:typA" {
						assert.Equal(t, "replacement", param)
					}

					return plugin.CreateResponse{
						ID:         "id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "pkgExt:index:func", req.Tok.String())
					assert.Equal(t, resource.NewStringProperty("in"), req.Args["input"])

					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output": resource.NewStringProperty("in " + param),
						},
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if param == "" {
						assert.Equal(t, tokens.Type("pkgA:m:typA"), req.URN.Type())
					} else {
						assert.Equal(t, tokens.Type("pkgExt:m:typA"), req.URN.Type())
					}

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil)
		require.NoError(t, err)

		// Register a resource using that base provider
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: pkgRef,
		})
		require.NoError(t, err)

		// Now register a replacement provider
		extRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, &pulumirpc.Parameterization{
			Name:    "pkgExt",
			Version: "0.5.0",
			Value:   []byte("replacement"),
		})
		require.NoError(t, err)

		// Test registering a resource with the replacement provider
		_, err = monitor.RegisterResource("pkgExt:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: extRef,
		})
		require.NoError(t, err)

		// Test invoking a function on the replacement provider
		result, _, err := monitor.Invoke("pkgExt:index:func", resource.PropertyMap{
			"input": resource.NewStringProperty("in"),
		}, "", "", extRef)
		require.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"output": resource.NewStringProperty("in replacement"),
		}, result)

		// Test reading a resource on the replacement provider
		_, _, err = monitor.ReadResource("pkgExt:m:typA", "resC", "id", "", resource.PropertyMap{}, "", "", "", extRef)
		require.NoError(t, err)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "up")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 5)

	// Check that we loaded the provider twice
	assert.Equal(t, 2, loadCount)

	// Check the state of the parameterized provider is what we expect
	prov := snap.Resources[2]
	assert.Equal(t, tokens.Type("pulumi:providers:pkgExt"), prov.Type)
	assert.Equal(t, "default_0_5_0", prov.URN.Name())
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]any{
		"name":    "pkgA",
		"version": "1.0.0",
		"parameterization": map[string]any{
			"version": "0.5.0",
			"value":   "cmVwbGFjZW1lbnQ=",
		},
	}), prov.Inputs)

	snap, err = TestOp(Refresh).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "refresh")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 5)

	snap, err = TestOp(Destroy).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "destroy")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}
