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
	"fmt"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshBeforeUpdate(t *testing.T) {
	t.Parallel()

	var readToken int

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(_ context.Context, req plugin.ProviderHandshakeRequest) (
					*plugin.ProviderHandshakeResponse, error,
				) {
					assert.True(t, req.SupportsRefreshBeforeUpdate)
					return &plugin.ProviderHandshakeResponse{
						AcceptSecrets:   true,
						AcceptResources: true,
						AcceptOutputs:   true,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					props := req.Properties.Copy()
					props["result"] = props["input"]
					return plugin.CreateResponse{
						Properties:          props,
						ID:                  "new-id",
						RefreshBeforeUpdate: true,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					if req.NewInputs.DeepEquals(req.OldInputs) {
						return plugin.DiffResponse{Changes: plugin.DiffNone}, nil
					}
					return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					assert.Equal(t, fmt.Sprintf("<FRESH-INPUT-%d>", readToken), req.OldInputs["input"].StringValue())
					assert.Equal(t, fmt.Sprintf("<FRESH-RESULT-%d>", readToken), req.OldOutputs["result"].StringValue())

					props := req.NewInputs.Copy()
					props["result"] = props["input"]
					return plugin.UpdateResponse{
						Properties:          props,
						RefreshBeforeUpdate: true,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					inputs := req.Inputs.Copy()
					inputs["input"] = resource.NewProperty(fmt.Sprintf("<FRESH-INPUT-%d>", readToken))
					props := req.State.Copy()
					props["input"] = inputs["input"]
					props["result"] = resource.NewProperty(fmt.Sprintf("<FRESH-RESULT-%d>", readToken))
					return plugin.ReadResponse{
						Status: resource.StatusOK,
						ReadResult: plugin.ReadResult{
							ID:                  "new-id",
							Inputs:              inputs,
							Outputs:             props,
							RefreshBeforeUpdate: true,
						},
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{
		"input": resource.NewStringProperty("value-1"),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// First update.
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
	assert.Equal(t, "default", snap.Resources[1].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("value-1"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("value-1"),
		"result": resource.NewProperty("value-1"),
	}, snap.Resources[2].Outputs)
	assert.True(t, snap.Resources[2].RefreshBeforeUpdate)

	// Second update.
	inputs = resource.PropertyMap{
		"input": resource.NewStringProperty("value-2"),
	}
	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
	assert.Equal(t, "default", snap.Resources[1].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("value-2"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("value-2"),
		"result": resource.NewProperty("value-2"),
	}, snap.Resources[2].Outputs)
	assert.True(t, snap.Resources[2].RefreshBeforeUpdate)

	// Third update (same).
	readToken++
	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
	assert.Equal(t, "default", snap.Resources[1].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("value-2"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("value-2"),
		"result": resource.NewProperty("value-2"),
	}, snap.Resources[2].Outputs)
	assert.True(t, snap.Resources[2].RefreshBeforeUpdate)

	// Fourth update with refresh (same).
	readToken++
	p.Steps = []lt.TestStep{{Op: Update}}
	p.Options.Refresh = true
	snap = p.Run(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
	assert.Equal(t, "default", snap.Resources[1].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("value-2"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("value-2"),
		"result": resource.NewProperty("value-2"),
	}, snap.Resources[2].Outputs)
	assert.True(t, snap.Resources[2].RefreshBeforeUpdate)

	// Refresh.
	readToken++
	p.Steps = []lt.TestStep{{Op: Refresh}}
	p.Options.Refresh = false
	snap = p.Run(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
	assert.Equal(t, "default", snap.Resources[1].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("<FRESH-INPUT-3>"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("<FRESH-INPUT-3>"),
		"result": resource.NewProperty("<FRESH-RESULT-3>"),
	}, snap.Resources[2].Outputs)
	assert.True(t, snap.Resources[2].RefreshBeforeUpdate)

	// Import.
	readToken = 42
	p.Steps = []lt.TestStep{{Op: lt.ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}})}}
	snap = p.Run(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
	assert.Equal(t, "default", snap.Resources[1].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("<FRESH-INPUT-3>"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("<FRESH-INPUT-3>"),
		"result": resource.NewProperty("<FRESH-RESULT-3>"),
	}, snap.Resources[2].Outputs)
	assert.True(t, snap.Resources[2].RefreshBeforeUpdate)
	assert.Equal(t, "resB", snap.Resources[3].URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("<FRESH-INPUT-42>"),
	}, snap.Resources[3].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"input":  resource.NewProperty("<FRESH-INPUT-42>"),
		"result": resource.NewProperty("<FRESH-RESULT-42>"),
	}, snap.Resources[3].Outputs)
	assert.True(t, snap.Resources[3].RefreshBeforeUpdate)
}
