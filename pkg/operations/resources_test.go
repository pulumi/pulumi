// Copyright 2016-2022, Pulumi Corporation.
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

package operations

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func getPulumiResources(t *testing.T, path string) *Resource {
	ctx := context.Background()
	var checkpoint apitype.CheckpointV3
	byts, err := os.ReadFile(path)
	require.NoError(t, err)
	err = json.Unmarshal(byts, &checkpoint)
	require.NoError(t, err)
	snapshot, err := stack.DeserializeCheckpoint(ctx, b64.Base64SecretsProvider, &checkpoint)
	require.NoError(t, err)
	resources := NewResourceTree(snapshot.Resources)
	return resources
}

func TestTodo(t *testing.T) {
	t.Parallel()

	components := getPulumiResources(t, "testdata/todo.json")
	require.Len(t, components.Children, 4)

	// Table child
	table, ok := components.GetChild("cloud:table:Table", "todo")
	assert.True(t, ok)
	require.NotNil(t, table)
	require.Len(t, table.State.Inputs, 2)
	assert.Equal(t, "id", table.State.Inputs["primaryKey"].StringValue())
	require.Len(t, table.Children, 1)
	table, ok = table.GetChild("aws:dynamodb/table:Table", "todo")
	assert.True(t, ok)
	require.NotNil(t, table)

	// Endpoint child
	endpoint, ok := components.GetChild("cloud:http:HttpEndpoint", "todo")
	assert.True(t, ok)
	require.NotNil(t, endpoint)
	require.Len(t, endpoint.State.Inputs, 5)
	assert.Equal(t,
		"https://eupwl7wu4i.execute-api.us-east-2.amazonaws.com/", endpoint.State.Inputs["url"].StringValue())
	require.Len(t, endpoint.Children, 14)
	endpoint, ok = endpoint.GetChild("aws:apigateway/restApi:RestApi", "todo")
	assert.True(t, ok)
	require.NotNil(t, endpoint)

	// Nonexistant resource.
	r, ok := endpoint.GetChild("garden:ornimentation/gnome", "stone")
	assert.False(t, ok)
	assert.Nil(t, r)
}

func TestCrawler(t *testing.T) {
	t.Parallel()

	components := getPulumiResources(t, "testdata/crawler.json")
	require.Len(t, components.Children, 7)

	// Topic child
	topic, ok := components.GetChild("cloud:topic:Topic", "countDown")
	assert.True(t, ok)
	require.NotNil(t, topic)
	assert.Empty(t, topic.State.Inputs)
	require.Len(t, topic.Children, 1)
	topic, ok = topic.GetChild("aws:sns/topic:Topic", "countDown")
	assert.True(t, ok)
	require.NotNil(t, topic)

	// Timer child
	heartbeat, ok := components.GetChild("cloud:timer:Timer", "heartbeat")
	assert.True(t, ok)
	require.NotNil(t, heartbeat)
	require.Len(t, heartbeat.State.Inputs, 1)
	assert.Equal(t, "rate(5 minutes)", heartbeat.State.Inputs["scheduleExpression"].StringValue())
	require.Len(t, heartbeat.Children, 4)

	// Function child of timer
	function, ok := heartbeat.GetChild("cloud:function:Function", "heartbeat")
	assert.True(t, ok)
	require.NotNil(t, function)
	require.Len(t, function.State.Inputs, 1)
	require.Len(t, function.Children, 3)
}
