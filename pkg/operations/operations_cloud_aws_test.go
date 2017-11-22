package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractLambdaLogMessage(t *testing.T) {
	res := extractLambdaLogMessage("START RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723 Version: $LATEST", "foo")
	assert.Nil(t, res)
	res = extractLambdaLogMessage("2017-11-17T20:30:27.736Z	25e0d1e0-cbd6-11e7-9808-c7085dfe5723	GET /todo", "foo")
	assert.NotNil(t, res)
	assert.Equal(t, "GET /todo", res.Message)
	res = extractLambdaLogMessage("END RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723", "foo")
	assert.Nil(t, res)
}

func Test_functionNameFromLogGroupNameRegExp(t *testing.T) {
	match := functionNameFromLogGroupNameRegExp.FindStringSubmatch("/aws/lambda/examples-todoc57917fa-023a27bc")
	assert.Len(t, match, 2)
	assert.Equal(t, "examples-todoc57917fa", match[1])
}
