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
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestPackageRef tests we can request a package ref from the engine and then use that, instead of Version,
// PackageDownloadURL etc.
func TestPackageRef(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn urn.URN, inputs resource.PropertyMap, timeout float64, preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "0", inputs, resource.StatusOK, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn urn.URN, inputs resource.PropertyMap, timeout float64, preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "1", inputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkg1Ref, err := monitor.RegisterProvider("pkgA", "1.0.0", "", nil)
		require.NoError(t, err)
		pkg2Ref, err := monitor.RegisterProvider("pkgA", "2.0.0", "", nil)
		require.NoError(t, err)

		// If we register the "same" provider in parallel, we should get the same ref.
		promises := []*promise.Promise[string]{}
		for i := 0; i < 100; i++ {
			var pcs promise.CompletionSource[string]
			promises = append(promises, pcs.Promise())
			go func() {
				ref, err := monitor.RegisterProvider("pkgB", "1.0.0", "downloadUrl", nil)
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
			Provider: pkg1Ref,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("0"), resp.ID)

		resp, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Provider: pkg2Ref,
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

					param = value.Value.(string)

					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				CreateF: func(urn urn.URN, inputs resource.PropertyMap, timeout float64, preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					if urn.Type() == "pkgExt:m:typA" {
						assert.Equal(t, "replacement", param)
					}

					return "id", inputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgRef, err := monitor.RegisterProvider("pkgA", "1.0.0", "", nil)
		require.NoError(t, err)

		// Register a resource using that base provider
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: pkgRef,
		})
		require.NoError(t, err)

		// Now register a replacement provider
		extRef, err := monitor.RegisterProvider("pkgExt", "0.5.0", "", &pulumirpc.ProviderParameter{
			Name:    "pkgA",
			Version: "1.0.0",
			Value:   structpb.NewStringValue("replacement"),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgExt:m:typA", "resB", true, deploytest.ResourceOptions{
			Provider: extRef,
		})
		require.NoError(t, err)

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

	// Check that we loaded the provider twice
	assert.Equal(t, 2, loadCount)

	snap, err = TestOp(Refresh).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
}
