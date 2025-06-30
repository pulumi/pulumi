// Copyright 2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that we can run a preview --refresh --run-program by executing the program for it the refresh stage.
func TestPreviewRefreshWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	updateOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("qux")}
	readOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if req.Name == "resA" {
						assert.Equal(t, createOutputs, req.Inputs)
						assert.Equal(t, createOutputs, req.State)

						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								ID:      req.ID,
								Inputs:  req.Inputs,
								Outputs: readOutputs,
							},
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  resource.PropertyMap{},
							Outputs: resource.PropertyMap{},
						},
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
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					assert.True(t, req.Preview)

					if req.Name == "resA" {
						// This should get called as part of the preview _after_ refresh so we should see the
						// program inputs and the read outputs.
						assert.Equal(t, programInputs, req.NewInputs)
						assert.Equal(t, readOutputs, req.OldOutputs)

						return plugin.UpdateResponse{
							Properties: updateOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.UpdateResponse{
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

		// First time we should see the create outputs, second time the update outputs
		if programExecutions == 1 {
			assert.Equal(t, createOutputs, resp.Outputs)
		} else {
			assert.Equal(t, updateOutputs, resp.Outputs)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, createOutputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider for refresh
	programInputs["foo"] = resource.NewStringProperty("qux")
	// Run a preview with refresh
	p.Options.Refresh = true
	p.Options.RefreshProgram = true
	_, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, true, p.BackendClient,
			func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				events []engine.Event, err error,
			) error {
				return err
			}, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
}
