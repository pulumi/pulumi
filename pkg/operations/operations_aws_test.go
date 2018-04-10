// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionCache(t *testing.T) {
	// Create a default session in us-west-2.
	sess1, err := getAWSSession("us-west-2", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, sess1)
	assert.Equal(t, "us-west-2", *sess1.Config.Region)

	// Create a session with explicit credentials and ensure they're set.
	sess2, err := getAWSSession("us-west-2", "AKIA123", "456", "xyz")
	assert.NoError(t, err)

	creds, err := sess2.Config.Credentials.Get()
	assert.NoError(t, err)
	assert.Equal(t, "AKIA123", creds.AccessKeyID)
	assert.Equal(t, "456", creds.SecretAccessKey)
	assert.Equal(t, "xyz", creds.SessionToken)

	// Create a session with different creds and make sure they're different.
	sess3, err := getAWSSession("us-west-2", "AKIA123", "456", "hij")
	assert.NoError(t, err)

	creds, err = sess3.Config.Credentials.Get()
	assert.NoError(t, err)
	assert.Equal(t, "AKIA123", creds.AccessKeyID)
	assert.Equal(t, "456", creds.SecretAccessKey)
	assert.Equal(t, "hij", creds.SessionToken)
}
