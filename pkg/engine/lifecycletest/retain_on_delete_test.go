// Copyright 2024, Pulumi Corporation.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestRetainOnDelete(t *testing.T) {
	t.Parallel()

	idCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					assert.Fail(t, "Delete was called")
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			retainOnDelete := true
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:         ins,
				RetainOnDelete: &retainOnDelete,
			})
			require.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())

	// Run a new update which will cause a replace, we shouldn't see a provider delete but should get a new id
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-1", snap.Resources[1].ID.String())

	// Run a new update which will cause a delete, we still shouldn't see a provider delete
	createResource = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}
