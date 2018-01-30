// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractLambdaLogMessage(t *testing.T) {
	res := extractLambdaLogMessage("START RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723 Version: $LATEST\n", "foo")
	assert.Nil(t, res)
	res = extractLambdaLogMessage("2017-11-17T20:30:27.736Z	25e0d1e0-cbd6-11e7-9808-c7085dfe5723	GET /todo\n", "foo")
	assert.NotNil(t, res)
	assert.Equal(t, "GET /todo", res.Message)
	res = extractLambdaLogMessage("END RequestId: 25e0d1e0-cbd6-11e7-9808-c7085dfe5723\n", "foo")
	assert.Nil(t, res)
}

func Test_functionNameFromLogGroupNameRegExp(t *testing.T) {
	match := oldFunctionNameFromLogGroupNameRegExp.FindStringSubmatch("/aws/lambda/examples-todoc57917fa023a27bc")
	assert.Len(t, match, 2)
	assert.Equal(t, "examples-todoc57917fa", match[1])
}

func Test_oldFunctionNameFromLogGroupNameRegExp(t *testing.T) {
	match := functionNameFromLogGroupNameRegExp.FindStringSubmatch("/aws/lambda/examples-todoc57917fa-023a27b")
	assert.Len(t, match, 2)
	assert.Equal(t, "examples-todoc57917fa", match[1])
}

func Test_extractMultilineLambdaLogMessage(t *testing.T) {
	res := extractLambdaLogMessage("2018-01-30T06:48:09.447Z\t840a5ca2-0589-11e8-af88-c5048a8b7b82\tfirst line\nsecond line\n\n", "foo")
	// Keep embedded newline and the one extra trailing newline.
	assert.Equal(t, "first line\nsecond line\n", res.Message)
}
