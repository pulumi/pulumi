// Copyright 2020-2025, Pulumi Corporation.
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
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Test that we can run a simple destroy by executing the program for it.
func TestDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" {
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{
						ID:         resource.ID(uuid.String()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: programInputs,
		})
		assert.NoError(t, err)

		// Should see the create outputs both times we run this program
		assert.Equal(t, createOutputs, resp.Outputs)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: true,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewStringProperty("qux")
	// Run a refresh
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Resources should be deleted
	assert.Len(t, snap.Resources, 0)
}
