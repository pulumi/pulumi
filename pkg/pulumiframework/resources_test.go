package pulumiframework

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/component"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
)

var sess *session.Session

func init() {
	var err error
	config := aws.NewConfig()
	config.Region = aws.String("eu-west-1")
	sess, err = session.NewSession(config)
	if err != nil {
		panic("Could not create AWS session")
	}
}

func getPulumiResources(t *testing.T, path string) (component.Components, tokens.QName) {
	var checkpoint stack.Checkpoint
	byts, err := ioutil.ReadFile(path)
	assert.NoError(t, err)
	err = json.Unmarshal(byts, &checkpoint)
	assert.NoError(t, err)
	name, _, snapshot, err := stack.DeserializeCheckpoint(&checkpoint)
	assert.NoError(t, err)
	resources := GetComponents(snapshot.Resources)
	spew.Dump(resources)
	return resources, name
}

func TestTodo(t *testing.T) {
	components, targetName := getPulumiResources(t, "testdata/todo.json")
	assert.Equal(t, 5, len(components))

	rawURN := resource.NewURN(targetName, "todo", "aws:dynamodb/table:Table:", "todo")

	tableArn := newPulumiFrameworkURN(rawURN, tokens.Type(pulumiTableType), tokens.QName("todo"))
	table, ok := components[tableArn]
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, 1, len(table.Properties))
	assert.Equal(t, "id", table.Properties[resource.PropertyKey("primaryKey")].StringValue())
	assert.Equal(t, 1, len(table.Resources))
	assert.Equal(t, pulumiTableType, table.Type)

	endpointArn := newPulumiFrameworkURN(rawURN, tokens.Type(pulumiEndpointType), tokens.QName("todo"))
	endpoint, ok := components[endpointArn]
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, 1, len(endpoint.Properties))
	assert.Equal(t, "https://eupwl7wu4i.execute-api.us-east-2.amazonaws.com/stage/",
		endpoint.Properties[resource.PropertyKey("url")].StringValue())
	assert.Equal(t, 3, len(endpoint.Resources))
	assert.Equal(t, pulumiEndpointType, endpoint.Type)
}

func TestCrawler(t *testing.T) {
	components, targetName := getPulumiResources(t, "testdata/crawler.json")
	assert.Equal(t, 4, len(components))

	rawURN := resource.NewURN(targetName, "countdown", "aws:sns/topic:Topic", "countDown")

	countDownArn := newPulumiFrameworkURN(rawURN, tokens.Type(pulumiTopicType), tokens.QName("countDown"))
	countDown, ok := components[countDownArn]
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, 0, len(countDown.Properties))
	assert.Equal(t, 1, len(countDown.Resources))
	assert.Equal(t, pulumiTopicType, countDown.Type)

	heartbeatArn := newPulumiFrameworkURN(rawURN, tokens.Type(pulumiTimerType), tokens.QName("heartbeat"))
	heartbeat, ok := components[heartbeatArn]
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, 1, len(heartbeat.Properties))
	assert.Equal(t, "rate(5 minutes)", heartbeat.Properties[resource.PropertyKey("schedule")].StringValue())
	assert.Equal(t, 3, len(heartbeat.Resources))
	assert.Equal(t, pulumiTimerType, heartbeat.Type)
}
