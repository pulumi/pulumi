// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWSFriendlyNames(t *testing.T) {
	assert.Equal(t, "test", makeAWSFriendlyName("test", false))
	assert.Equal(t, "Test", makeAWSFriendlyName("test", true))
	assert.Equal(t, "test", makeAWSFriendlyName("Test", false))
	assert.Equal(t, "Test", makeAWSFriendlyName("Test", true))
	assert.Equal(t, "tEST", makeAWSFriendlyName("TEST", false))
	assert.Equal(t, "TEST", makeAWSFriendlyName("TEST", true))
	assert.Equal(t, "test123Test", makeAWSFriendlyName("test123Test", false))
	assert.Equal(t, "Test123Test", makeAWSFriendlyName("test123Test", true))
	assert.Equal(t, "test123TestAbc", makeAWSFriendlyName("!.:test-123.Test/abc", false))
	assert.Equal(t, "Test123TestAbc", makeAWSFriendlyName("!.:test-123.Test/abc", true))
}
