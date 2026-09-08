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

package display

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStepDiffJSON_SameHasNoDiff(t *testing.T) {
	t.Parallel()

	step := engine.StepEventMetadata{
		Op:  deploy.OpSame,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{"a": resource.NewProperty("1")}},
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{"a": resource.NewProperty("2")}},
	}

	assert.Nil(t, stepDiffJSON(step, false))
}

func TestStepDiffJSON_CreateIsAllCreates(t *testing.T) {
	t.Parallel()

	step := engine.StepEventMetadata{
		Op: deploy.OpCreate,
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"bucket": resource.NewProperty("mybucket"),
		}},
	}

	got := stepDiffJSON(step, false)
	require.NotNil(t, got)
	assert.Equal(t, map[string]any{"bucket": "mybucket"}, got.Creates)
	assert.Empty(t, got.Deletes)
	assert.Empty(t, got.Updates)
}

func TestStepDiffJSON_DeleteIsAllDeletes(t *testing.T) {
	t.Parallel()

	step := engine.StepEventMetadata{
		Op: deploy.OpDelete,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"bucket": resource.NewProperty("mybucket"),
		}},
	}

	got := stepDiffJSON(step, false)
	require.NotNil(t, got)
	assert.Equal(t, map[string]any{"bucket": "mybucket"}, got.Deletes)
	assert.Empty(t, got.Creates)
}

func TestStepDiffJSON_UpdateUsesDetailedDiff(t *testing.T) {
	t.Parallel()

	step := engine.StepEventMetadata{
		Op: deploy.OpUpdate,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"tags": resource.NewProperty(resource.PropertyMap{
				"owner": resource.NewProperty("tom"),
			}),
		}},
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"tags": resource.NewProperty(resource.PropertyMap{
				"owner": resource.NewProperty("tim"),
				"env":   resource.NewProperty("prod"),
			}),
			"versioning": resource.NewProperty(true),
		}},
		DetailedDiff: map[string]plugin.PropertyDiff{
			"tags.owner": {Kind: plugin.DiffUpdate, InputDiff: true},
			"tags.env":   {Kind: plugin.DiffAdd, InputDiff: true},
			"versioning": {Kind: plugin.DiffAdd, InputDiff: true},
		},
	}

	got := stepDiffJSON(step, false)
	require.NotNil(t, got)

	assert.Equal(t, map[string]any{"versioning": true}, got.Creates)

	tags, ok := got.Updates["tags"]
	require.True(t, ok, "expected a nested diff for tags")
	require.NotNil(t, tags.Object)
	assert.Equal(t, map[string]any{"env": "prod"}, tags.Object.Creates)

	owner, ok := tags.Object.Updates["owner"]
	require.True(t, ok)
	require.NotNil(t, owner.Old)
	require.NotNil(t, owner.New)
	assert.Equal(t, "tom", *owner.Old)
	assert.Equal(t, "tim", *owner.New)
}

func TestStepDiffJSON_SecretsAreBlinded(t *testing.T) {
	t.Parallel()

	step := engine.StepEventMetadata{
		Op: deploy.OpCreate,
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"password": resource.MakeSecret(resource.NewProperty("hunter2")),
		}},
	}

	encoded, err := json.Marshal(stepDiffJSON(step, false /* showSecrets */))
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "hunter2")
}

func TestStepDiffJSON_HiddenPathsAreReportedNotLeaked(t *testing.T) {
	t.Parallel()

	step := engine.StepEventMetadata{
		Op: deploy.OpUpdate,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"secretish": resource.NewProperty("before"),
			"plain":     resource.NewProperty("a"),
		}},
		New: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"secretish": resource.NewProperty("after"),
				"plain":     resource.NewProperty("b"),
			},
			HideDiffs: []resource.PropertyPath{{"secretish"}},
		},
	}

	got := stepDiffJSON(step, false)
	require.NotNil(t, got)
	assert.Equal(t, []string{"secretish"}, got.Hidden)
	assert.NotContains(t, got.Updates, "secretish")
	assert.Contains(t, got.Updates, "plain")
}
