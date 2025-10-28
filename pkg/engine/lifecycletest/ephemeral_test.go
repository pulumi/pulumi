// Copyright 2025-2025, Pulumi Corporation.
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
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestSingleEphemeralResource(t *testing.T) {
	t.Parallel()

	resources := map[string]resource.PropertyMap{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if _, has := resources[string(req.ID)]; !has {
						return plugin.DeleteResponse{}, fmt.Errorf("unknown resource ID: %s", req.ID)
					}

					delete(resources, string(req.ID))

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					resources[id.String()] = req.Properties

					return plugin.CreateResponse{
						ID:         resource.ID(id.String()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		props := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
		resp, err := monitor.RegisterResource("pkgA:index:typ", "resA", true, deploytest.ResourceOptions{
			Inputs:    props,
			Ephemeral: true,
		})
		require.NoError(t, err)
		assert.Equal(t, props, resp.Outputs)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		// TODO: THIS SHOULD TEST DISPLAY
		SkipDisplayTests: true,
	}
	p := &lt.TestPlan{}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Should just have the provider resource left
	require.Len(t, snap.Resources, 1)
}
