// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"crypto/sha1"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUniqueHex(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 100
	id, err := NewUniqueHex(prefix, maxlen, randlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+randlen*2, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexMaxLen(t *testing.T) {
	prefix := "prefix"
	randlen := 10
	maxlen := 20
	_, err := NewUniqueHex(prefix, maxlen, randlen)
	assert.NotNil(t, err)
}

func TestNewUniqueHexEnsureRandomness1(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	// not enough space for the prefix if we want randomness
	maxlen := 6
	_, err := NewUniqueHex(prefix, maxlen, randlen)
	assert.NotNil(t, err)
}

func TestNewUniqueHexEnsureRandomness2(t *testing.T) {
	prefix := "prefix"
	randlen := 4
	// Just enough space to have 8 chars of randomenss
	maxlen := 14
	id, err := NewUniqueHex(prefix, maxlen, randlen)
	assert.Nil(t, err)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueDefaults(t *testing.T) {
	prefix := "prefix"
	id, err := NewUniqueHex(prefix, -1, -1)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+(sha1.Size*2), len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexID(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 100
	id, err := NewUniqueHexID(prefix, maxlen, randlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+randlen*2, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueHexMaxLenID(t *testing.T) {
	prefix := "prefix"
	randlen := 7
	maxlen := 20
	id, err := NewUniqueHexID(prefix, maxlen, randlen)
	assert.Nil(t, err)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueDefaultsID(t *testing.T) {
	prefix := "prefix"
	id, err := NewUniqueHexID(prefix, -1, -1)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+(sha1.Size*2), len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}
