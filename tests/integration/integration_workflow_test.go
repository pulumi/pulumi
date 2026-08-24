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

package ints

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// TestWorkflowSmoke drives a workflow of random resources end to end: one release cursor per region
// deploys its region, parks on a gated join, and — once approved — the join merges both cursors into a
// single rollout that deploys prod. A further up reconciles every node without touching its resources,
// and a destroy sweeps the workflow with everything its nodes made.
//
//nolint:paralleltest // ProgramTestManualLifeCycle calls t.Parallel itself.
func TestWorkflowSmoke(t *testing.T) {
	// Resolve the local SDK explicitly so the test does not depend on GOPATH conventions or
	// PULUMI_GO_DEP_ROOT (this repo may be checked out as a worktree under another name).
	sdk, err := filepath.Abs(filepath.Join("..", "..", "sdk"))
	require.NoError(t, err)
	pt := integration.ProgramTestManualLifeCycle(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("workflow", "go"),
		Dependencies: []string{"github.com/pulumi/pulumi/sdk/v3=" + sdk},
		Config:       map[string]string{"sha": "abc", "approve": "false"},
		Quick:        true,
		// The program uses the random provider, which CI must be allowed to download.
		Env: []string{"PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=false"},
	})
	t.Cleanup(func() {
		pt.TestFinished = true
		pt.TestCleanUp()
	})
	require.NoError(t, pt.TestLifeCyclePrepare(), "prepare")
	require.NoError(t, pt.TestLifeCycleInitialize(), "initialize")
	up := func() {
		require.NoError(t, pt.RunPulumiCommand("up", "--non-interactive", "--yes", "--skip-preview"))
	}

	type snapshot struct {
		cursors map[string]string               // cursor name -> node
		ids     map[string]string               // RandomId URN -> hex output
		records map[string]resource.PropertyMap // node -> record
	}
	export := func() snapshot {
		t.Helper()
		file := filepath.Join(t.TempDir(), "state.json")
		require.NoError(t, pt.RunPulumiCommand("stack", "export", "--file", file))
		raw, err := os.ReadFile(file)
		require.NoError(t, err)
		var untyped apitype.UntypedDeployment
		require.NoError(t, json.Unmarshal(raw, &untyped))
		var deployment apitype.DeploymentV3
		require.NoError(t, json.Unmarshal(untyped.Deployment, &deployment))

		got := snapshot{cursors: map[string]string{}, ids: map[string]string{}, records: map[string]resource.PropertyMap{}}
		for _, res := range deployment.Resources {
			switch string(res.Type) {
			case "pulumi:index:Workflow":
				for _, c := range res.Outputs["cursors"].([]any) {
					c := c.(map[string]any)
					got.cursors[c["name"].(string)] = c["node"].(string)
				}
				for node, record := range res.Outputs["records"].(map[string]any) {
					got.records[node] = resource.NewPropertyMapFromMap(record.(map[string]any))
				}
			case "random:index/randomId:RandomId":
				got.ids[string(res.URN)] = res.Outputs["hex"].(string)
			default:
			}
		}
		return got
	}

	// Up 1: each entry places its region's cursor, which deploys a RandomId and parks on the gated join.
	up()
	first := export()
	assert.Equal(t, map[string]string{"east#1": "east", "west#2": "west"}, first.cursors)
	require.Len(t, first.ids, 2)
	assert.Equal(t, "east#1", first.records["east"]["occupant"].StringValue())
	assert.Equal(t, "west#2", first.records["west"]["occupant"].StringValue())
	assert.Empty(t, first.records["prod"]["visits"].ArrayValue(), "prod has not been visited")

	// Up 2: approved. The join's branches pass, the merge combines both cursors into release#3, and prod
	// deploys from the merged values. The regions' resources are reconciled, not replaced.
	require.NoError(t, pt.RunPulumiCommand("config", "set", "approve", "true"))
	up()
	second := export()
	assert.Equal(t, map[string]string{"release#3": "prod"}, second.cursors)
	require.Len(t, second.ids, 3)
	for urn, hex := range first.ids {
		assert.Equal(t, hex, second.ids[urn], "%v is reconciled, not replaced", urn)
	}
	assert.Equal(t, "release#3", second.records["prod"]["occupant"].StringValue())
	// The merged cursor entered prod with each region's deployment and the branch overlays applied: the
	// gate's approval stamp survives, its deletion of "region" does too.
	entered := second.records["prod"]["visits"].ArrayValue()[0].ObjectValue()["inputs"].ObjectValue()
	assert.Equal(t, resource.NewProperty("abc"), entered["sha"])
	assert.NotContains(t, entered, resource.PropertyKey("region"))
	for _, region := range []string{"east", "west"} {
		record := second.records[region]
		_, occupied := record["occupant"]
		assert.False(t, occupied, "%v was vacated by the join", region)
		visit := record["visits"].ArrayValue()[0].ObjectValue()
		assert.Contains(t, visit, resource.PropertyKey("left"))
		assert.Equal(t, entered[resource.PropertyKey("from-"+region)], visit["outputs"].ObjectValue()["deployment"])
	}

	// Up 3: nothing changed. Every node reconciles from the values its cursor entered with; nothing is
	// replaced and the cursor stays put.
	up()
	third := export()
	assert.Equal(t, second.cursors, third.cursors)
	assert.Equal(t, second.ids, third.ids)

	// Destroy sweeps the workflow, its nodes and everything their programs made (and removes the stack).
	require.NoError(t, pt.TestLifeCycleDestroy(), "destroy")
}
