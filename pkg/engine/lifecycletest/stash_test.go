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
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func TestStashImportError(t *testing.T) {
	t.Parallel()

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _ = monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			ImportID: "someid",
		})
		// The resource registration fails, and the engine knows this and
		// cancels the deployment. RegisterResource will not return.
		t.Fatalf("We should not return from RegisterResource")
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "stash can not be imported")
}

func TestStash(t *testing.T) {
	t.Parallel()

	input := resource.NewProperty("first")
	expectedOutput := input
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"input": input,
			},
		})
		require.NoError(t, err)
		require.Equal(t, input, resp.Outputs["input"])
		require.Equal(t, expectedOutput, resp.Outputs["output"])

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	input = resource.NewProperty("second")
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
