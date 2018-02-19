// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValidDefaultProjectName(t *testing.T) {
	// Valid names remain the same.
	for _, name := range getValidProjectNamePrefixes() {
		assert.Equal(t, name, getValidProjectName(name))
	}
	assert.Equal(t, "foo", getValidProjectName("foo"))
	assert.Equal(t, "foo1", getValidProjectName("foo1"))
	assert.Equal(t, "foo-", getValidProjectName("foo-"))
	assert.Equal(t, "foo-bar", getValidProjectName("foo-bar"))
	assert.Equal(t, "foo_", getValidProjectName("foo_"))
	assert.Equal(t, "foo_bar", getValidProjectName("foo_bar"))
	assert.Equal(t, "foo.", getValidProjectName("foo."))
	assert.Equal(t, "foo.bar", getValidProjectName("foo.bar"))

	// Invalid characters are left off.
	assert.Equal(t, "foo", getValidProjectName("@foo"))
	assert.Equal(t, "foo", getValidProjectName("-foo"))
	assert.Equal(t, "foo", getValidProjectName("0foo"))
	assert.Equal(t, "foo", getValidProjectName("1foo"))
	assert.Equal(t, "foo", getValidProjectName("2foo"))
	assert.Equal(t, "foo", getValidProjectName("3foo"))
	assert.Equal(t, "foo", getValidProjectName("4foo"))
	assert.Equal(t, "foo", getValidProjectName("5foo"))
	assert.Equal(t, "foo", getValidProjectName("6foo"))
	assert.Equal(t, "foo", getValidProjectName("7foo"))
	assert.Equal(t, "foo", getValidProjectName("8foo"))
	assert.Equal(t, "foo", getValidProjectName("9foo"))

	// Invalid names are replaced with a fallback name.
	assert.Equal(t, "project", getValidProjectName("@"))
	assert.Equal(t, "project", getValidProjectName("-"))
	assert.Equal(t, "project", getValidProjectName("0"))
	assert.Equal(t, "project", getValidProjectName("1"))
	assert.Equal(t, "project", getValidProjectName("2"))
	assert.Equal(t, "project", getValidProjectName("3"))
	assert.Equal(t, "project", getValidProjectName("4"))
	assert.Equal(t, "project", getValidProjectName("5"))
	assert.Equal(t, "project", getValidProjectName("6"))
	assert.Equal(t, "project", getValidProjectName("7"))
	assert.Equal(t, "project", getValidProjectName("8"))
	assert.Equal(t, "project", getValidProjectName("9"))
	assert.Equal(t, "project", getValidProjectName("@1"))
}

func getValidProjectNamePrefixes() []string {
	var results []string
	for ch := 'A'; ch <= 'Z'; ch++ {
		results = append(results, string(ch))
	}
	for ch := 'a'; ch <= 'z'; ch++ {
		results = append(results, string(ch))
	}
	results = append(results, "_")
	results = append(results, ".")
	return results
}
