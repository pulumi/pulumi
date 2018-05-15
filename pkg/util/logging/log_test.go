// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package logging

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

func TestFilter(t *testing.T) {
	filter1 := CreateFilter([]string{"secret1", "secret2"}, "[secret]")
	msg1 := filter1.Filter("These are my secrets: secret1, secret2, secret3, secret10")
	assert.Equal(t, msg1, "These are my secrets: [secret], [secret], secret3, [secret]0")

	// Ensure htat special characters don't screw up the regex we create
	filter2 := CreateFilter([]string{"secret.*", "secre[t]3"}, "[creds]")
	msg2 := filter2.Filter("These are my secrets: secret1, secret2, secret3, secret.*, secre[t]3")
	assert.Equal(t, msg2, "These are my secrets: secret1, secret2, secret3, [creds], [creds]")
}
