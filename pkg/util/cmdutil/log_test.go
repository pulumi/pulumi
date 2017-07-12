// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitLogging(t *testing.T) {
	// Just ensure we can initialize logging (and reset it afterwards).
	prevLog := LogToStderr
	prevV := Verbose
	prevFlow := LogFlow
	InitLogging(true, 9, true)
	InitLogging(prevLog, prevV, prevFlow)
	assert.Equal(t, prevLog, LogToStderr)
	assert.Equal(t, prevV, Verbose)
	assert.Equal(t, prevFlow, LogFlow)
}
