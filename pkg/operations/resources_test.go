// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package operations

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource/stack"
)

func getPulumiResources(t *testing.T, path string) *Resource {
	var checkpoint stack.Checkpoint
	byts, err := ioutil.ReadFile(path)
	assert.NoError(t, err)
	err = json.Unmarshal(byts, &checkpoint)
	assert.NoError(t, err)
	_, _, snapshot, err := stack.DeserializeCheckpoint(&checkpoint)
	assert.NoError(t, err)
	resources := NewResourceTree(snapshot.Resources)
	spew.Dump(resources)
	return resources
}

func TestTodo(t *testing.T) {
	components := getPulumiResources(t, "testdata/todo.json")
	assert.Equal(t, 4, len(components.Children))

	// Table child
	table := components.GetChild("cloud:table:Table", "todo")
	if !assert.NotNil(t, table) {
		return
	}
	assert.Equal(t, 2, len(table.State.Inputs))
	assert.Equal(t, "id", table.State.Inputs["primaryKey"].StringValue())
	assert.Equal(t, 1, len(table.Children))
	assert.NotNil(t, table.GetChild("aws:dynamodb/table:Table", "todo"))

	// Endpoint child
	endpoint := components.GetChild("cloud:http:HttpEndpoint", "todo")
	if !assert.NotNil(t, endpoint) {
		return
	}
	assert.Equal(t, 5, len(endpoint.State.Inputs))
	assert.Equal(t,
		"https://eupwl7wu4i.execute-api.us-east-2.amazonaws.com/", endpoint.State.Inputs["url"].StringValue())
	assert.Equal(t, 14, len(endpoint.Children))
	assert.NotNil(t, endpoint.GetChild("aws:apigateway/restApi:RestApi", "todo"))
}

func TestCrawler(t *testing.T) {
	components := getPulumiResources(t, "testdata/crawler.json")
	assert.Equal(t, 7, len(components.Children))

	// Topic child
	topic := components.GetChild("cloud:topic:Topic", "countDown")
	if !assert.NotNil(t, topic) {
		return
	}
	assert.Equal(t, 0, len(topic.State.Inputs))
	assert.Equal(t, 1, len(topic.Children))
	assert.NotNil(t, topic.GetChild("aws:sns/topic:Topic", "countDown"))

	// Timer child
	heartbeat := components.GetChild("cloud:timer:Timer", "heartbeat")
	if !assert.NotNil(t, heartbeat) {
		return
	}
	assert.Equal(t, 1, len(heartbeat.State.Inputs))
	assert.Equal(t, "rate(5 minutes)", heartbeat.State.Inputs["scheduleExpression"].StringValue())
	assert.Equal(t, 4, len(heartbeat.Children))

	// Function child of timer
	function := heartbeat.GetChild("cloud:function:Function", "heartbeat")
	if !assert.NotNil(t, function) {
		return
	}
	assert.Equal(t, 1, len(function.State.Inputs))
	assert.Equal(t, 3, len(function.Children))
}
