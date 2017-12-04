// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUniqueHex(t *testing.T) {
	prefix := "prefix"
	maxlen := 100
	id, err := NewUniqueHex(prefix, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexMaxLen2(t *testing.T) {
	prefix := "prefix"
	maxlen := 13
	_, err := NewUniqueHex(prefix, maxlen)
	assert.NotNil(t, err)
}

func TestNewUniqueHexEnsureRandomness2(t *testing.T) {
	prefix := "prefix"
	// Just enough space to have 8 chars of randomenss
	maxlen := 14
	id, err := NewUniqueHex(prefix, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueDefaults(t *testing.T) {
	prefix := "prefix"
	id, err := NewUniqueHex(prefix, -1)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexID(t *testing.T) {
	prefix := "prefix"
	maxlen := 100
	id, err := NewUniqueHexID(prefix, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueHexMaxLenID(t *testing.T) {
	prefix := "prefix"
	maxlen := 20
	id, err := NewUniqueHexID(prefix, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueDefaultsID(t *testing.T) {
	prefix := "prefix"
	id, err := NewUniqueHexID(prefix, -1)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}
