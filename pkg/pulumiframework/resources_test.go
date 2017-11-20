package pulumiframework

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
	resources := NewResource(snapshot.Resources)
	spew.Dump(resources)
	return resources
}

func TestTodo(t *testing.T) {
	components := getPulumiResources(t, "testdata/todo.json")
	assert.Equal(t, 4, len(components.children))

	// Table child
	table := components.GetChild("cloud:table:Table", "todo")
	if !assert.NotNil(t, table) {
		return
	}
	assert.Equal(t, 2, len(table.state.Inputs))
	assert.Equal(t, "id", table.state.Inputs["primaryKey"].StringValue())
	assert.Equal(t, 1, len(table.children))
	assert.NotNil(t, table.GetChild("aws:dynamodb/table:Table", "todo"))

	// Endpoint child
	endpoint := components.GetChild("cloud:http:HttpEndpoint", "todo")
	if !assert.NotNil(t, endpoint) {
		return
	}
	assert.Equal(t, 5, len(endpoint.state.Inputs))
	assert.Equal(t, "https://eupwl7wu4i.execute-api.us-east-2.amazonaws.com/", endpoint.state.Inputs["url"].StringValue())
	assert.Equal(t, 14, len(endpoint.children))
	assert.NotNil(t, endpoint.GetChild("aws:apigateway/restApi:RestApi", "todo"))
}

func TestCrawler(t *testing.T) {
	components := getPulumiResources(t, "testdata/crawler.json")
	assert.Equal(t, 7, len(components.children))

	// Topic child
	topic := components.GetChild("cloud:topic:Topic", "countDown")
	if !assert.NotNil(t, topic) {
		return
	}
	assert.Equal(t, 0, len(topic.state.Inputs))
	assert.Equal(t, 1, len(topic.children))
	assert.NotNil(t, topic.GetChild("aws:sns/topic:Topic", "countDown"))

	// Timer child
	heartbeat := components.GetChild("cloud:timer:Timer", "heartbeat")
	if !assert.NotNil(t, heartbeat) {
		return
	}
	assert.Equal(t, 1, len(heartbeat.state.Inputs))
	assert.Equal(t, "rate(5 minutes)", heartbeat.state.Inputs["scheduleExpression"].StringValue())
	assert.Equal(t, 4, len(heartbeat.children))

	// Function child of timer
	function := heartbeat.GetChild("cloud:function:Function", "heartbeat")
	if !assert.NotNil(t, function) {
		return
	}
	assert.Equal(t, 1, len(function.state.Inputs))
	assert.Equal(t, 3, len(function.children))
}
