package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractLambdaLogMessage(t *testing.T) {
	res := extractLambdaLogMessage("START RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723 Version: $LATEST")
	assert.Nil(t, res)
	res = extractLambdaLogMessage("2017-11-17T20:30:27.736Z	25e0d1e0-cbd6-11e7-9808-c7085dfe5723	GET /todo")
	assert.NotNil(t, res)
	assert.Equal(t, "GET /todo", res.Message)
	res = extractLambdaLogMessage("END RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723")
	assert.Nil(t, res)
	res = extractLambdaLogMessage("REPORT RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723	Duration: 222.92 ms	Billed Duration: 300 ms 	Memory Size: 128 MB	Max Memory Used: 33 MB")
	assert.Nil(t, res)
}
